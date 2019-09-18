---
title: Redis 内存管理：zmalloc
date: 2019-09-17 20:26:14
tags: Redis
---

总结下 Redis 内存分配工具 zmalloc 的实现。

<!-- more -->

## 前言

因工作需要，最近在参考 [如何阅读 Redis 源码](http://blog.huangz.me/ diary/2014/how-to-read-redis-source-code.html)和[《Redis设计与实现》](https://book.douban.com/subject/25900156/)开始学习 [Redis3.0](https://github.com/huangz1990/redis-3.0-annotated) 的源码实现。

本文是 RoadMap 的 1.1 小节 “内存分配” 的学习笔记。本文代码：[zmalloc.c](https://github.com/wuYin/redis-3.0/blob/master/zmalloc.c)



## Redis 内存分配

查看头文件 [zmalloc.h](https://github.com/wuYin/redis-3.0/blob/master/zmalloc.h) 的函数原型声明，能找到 Redis 的内存操作函数有：

```c
void *zmalloc(size_t size); // 内存分配，对应 malloc
void *zcalloc(size_t size); // 带初始化的内存分配，对应 calloc
void *zrealloc(void *ptr, size_t size); // 内存重分配，对应 realloc
void zfree(void *ptr); // 释放内存，对应 free
```



### zmalloc

#### 源码分析

```c
void *zmalloc(size_t size) {
    void *ptr = malloc(size + PREFIX_SIZE); // size + 8 + 内存对齐所需字节数 = 申请空间精确大小

    if (!ptr) zmalloc_oom_handler(size);
#ifdef HAVE_MALLOC_SIZE
    update_zmalloc_stat_alloc(zmalloc_size(ptr)); // 有内存长度计算函数的系统直接取长度
    return ptr;
#else
    *((size_t *) ptr) = size; // 将 size 存到 PREFIX 中
    update_zmalloc_stat_alloc(size + PREFIX_SIZE); // 算上 PREFIX 计算对齐空间，更新 used_memory
    return (char *) ptr + PREFIX_SIZE; // 向右偏移 PREFIX 字节
#endif
}
```

入参 `size` 是申请分配的字节数，其内存布局如下：

![image-20190917211507245](https://images.yinzige.com/2019-09-17-131507.png)

如调用 `zmalloc(10)` 申请 10 字节，会在前 8 字节 `PREFIX_SIZE` 中存 10，且 malloc 会额外分配 8 字节做内存对齐。Redis 使用 libc，没有 `malloc_size` 等函数，为精确到字节来统计已分配的内存大小，故在每段内存头部存储其长度。



#### 宏 update_zmalloc_stat_alloc

Redis 使用全局变量 `used_memory` 记录已分配内存的字节数，zmalloc 第 6 行用本宏更新其值：

```c
// 手动计算内存对齐后实际空间的大小
// & 位运算取余比 % 更高效，如申请 10 字节，发现 (10 & 7) == 2 不对齐，需加 8-2 = 6 共 16 字节对齐，为 malloc 最终分配的大小
#define update_zmalloc_stat_alloc(__n) do { \
    size_t _n = (__n); \
    if (_n&(sizeof(long)-1)) _n += sizeof(long)-(_n&(sizeof(long)-1)); \
    if (zmalloc_thread_safe) { \
        update_zmalloc_stat_add(_n); \
    } else { \
        used_memory += _n; \
    } \
} while(0)
```

Redis 实现大量宏，避免了函数调用的运行时开销，从而提升性能。宏的本质是字符展开，需注意用 `()` 保证计算次序，如：

```c
#define square(x) x*x // 求次方
square(2+1); // 展开为 2+1*2+1 == 5
// #define square(x) (x)*(x) // 正确定义
```

同理，宏定义中的 `do{} while(0)` 保证了代码块在展开后依旧作为整体执行。



#### 宏 update_zmalloc_stat_add

若开启线程安全，在访问 `used_memory` 前，先对 `used_memory_mutex` 互斥锁上锁，操作后释放：

```c
#define update_zmalloc_stat_add(__n) do { \
    pthread_mutex_lock(&used_memory_mutex); \
    used_memory += (__n); \
    pthread_mutex_unlock(&used_memory_mutex); \
} while(0)
```



### zcalloc

- 相比 `stdlib.h` 的 `calloc`  去掉了 count 计算，size 就是总大小。
- 相比 zmalloc，分配的内存已被初始化。

```c
void *zcalloc(size_t size) {
    void *ptr = calloc(1, size + PREFIX_SIZE); // 入参 size 已乘过 nmemb
    if (!ptr) zmalloc_oom_handler(size);
    // ...
    *((size_t *) ptr) = size;
    update_zmalloc_stat_alloc(size + PREFIX_SIZE);
    return (char *) ptr + PREFIX_SIZE;
}
```



### zrealloc

源码中直接复用了 `realloc`，内存重新分配后，旧指针会被系统自动回收，不能手动 `free`



### zfree

清理内存段时，需左移 PREFIX_SIZE 头部找到内存段的真正起始位置，再释放内存。

```c
void zfree(void *ptr) {
    // ...
    realptr = (char *) ptr - PREFIX_SIZE; // 回退到 malloc 分配的起点
    oldsize = *((size_t *) realptr);
    update_zmalloc_stat_free(oldsize + PREFIX_SIZE);
    free(realptr);
}
```





## Redis 内存统计

Redis 命令 INFO 的输出中：

```yaml
# Memory
used_memory:1039360
used_memory_human:1015.00K
used_memory_rss:2256896
used_memory_rss_human:2.15M
mem_fragmentation_ratio:2.25
mem_fragmentation_bytes:1251776
```

如上的内存指标分别来源于 zmalloc 中的内存统计函数：

```c
size_t zmalloc_used_memory(void); // 获取 Redis 已分配的内存大小
size_t zmalloc_get_rss(void); // 获取 Redis 的 RSS 内存大小
float zmalloc_get_fragmentation_ratio(size_t rss); // 计算 Redis 的内存碎片率
```



### zmalloc_used_memory

直接返回 Redis 记录的已分配内存的大小，线程安全模式下启用互斥锁读。

```c
size_t zmalloc_used_memory(void) {
    size_t um;
    if (zmalloc_thread_safe) {
#ifdef HAVE_ATOMIC
    um = __sync_add_and_fetch(&used_memory, 0);
#else
    pthread_mutex_lock(&used_memory_mutex); // 互斥锁保证读取 used_memory 时不会被其他线程修改，即线程安全
    um = used_memory;
    pthread_mutex_unlock(&used_memory_mutex);
#endif
    } else {
    um = used_memory;
    }
    return um;
}
```



### zmalloc_get_rss

RSS 是 Resident Set Size 的缩写，其值为进程驻内存的空间大小，不含被系统分配到 swap 的空间。在 Linux 中，每个进程在 `/proc/[pid]/stat` 文件中记录有进程状态，[第 24 列](http://man7.org/linux/man-pages/man5/proc.5.html)为 RSS 的值：

![image-20190918093254636](https://images.yinzige.com/2019-09-18-013255.png)

Redis 直接打开文件并切割读取：

```c
size_t zmalloc_get_rss(void) {
   int page = sysconf(_SC_PAGESIZE); // 获取系统的内存页大小配置信息，4KB
   size_t rss;
   char buf[4096];
   char filename[256];
   int fd, count;
   char *p, *x;

   snprintf(filename,256,"/proc/%d/stat",getpid()); // 获取进程 pid 定位 stat 文件
   if ((fd = open(filename,O_RDONLY)) == -1) return 0;
   if (read(fd,buf,4096) <= 0) { // 读失败或为空则退出
      close(fd);
      return 0;
   }
   close(fd);

   p = buf;
   count = 23; /* RSS is the 24th field in /proc/<pid>/stat */
   while(p && count--) {
      p = strchr(p,' '); // 按空格切割 23 次
      if (p) p++;
   }
   if (!p) return 0;
   x = strchr(p,' '); // p->RSS x->...
   if (!x) return 0;
   *x = '\0'; // 截取 RSS

   rss = strtoll(p,NULL,10); // 将 p->NULL 之间的字符串转为 10 进制数
   rss *= page; // * 4KB 即得 redis 驻内存大小
   return rss;
}
```



### zmalloc_get_fragmentation_ratio

该函数计算 Redis 的内存碎片率，直接用 RSS 除 used_memory

```c
/* Fragmentation = RSS / allocated-bytes */
float zmalloc_get_fragmentation_ratio(size_t rss) {
    return (float) rss / zmalloc_used_memory();
}
```

该指标的三个区间对应三种情况：

- ratio < 1：RSS 驻内存大小少于内存分配器分配的大小，说明部分冷数据被系统存入了 Swap 分区。对随机访问，磁盘耗时 10ms 级，内存耗时 100ns 级，相差五个量级。故比值越低，响应延迟会越高。
- 1 < ratio < 1.5：正常。
- ratio > 1.5：Redis 内存分配器未及时释放内存产生内部碎片，导致系统分配给 Redis 的大量内存未被有效利用。



## 总结

zmalloc 在 libc 上为实现精确的内存统计，在分配的每段内存头部 PREFIX_SIZE 中存储 size 的大小，并在寻址时左移跳过该头部。它还维护了全局变量 used_memory 并进行线程安全地读写，实现了 RSS 的读取并计算内存碎片率等。

文章内容待完善，2019.9.18