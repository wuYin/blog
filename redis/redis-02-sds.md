---
title: Redis 字符串：SDS
date: 2019-09-19 09:31:35
tags: Redis
---

总结 Redis 封装 C 字符串为 SDS 的实现。

<!-- more -->

## SDS 结构

### 结构定义

SDS 全称 <u>Simple Dynamic String</u>（简单动态字符串），是 Redis 对 C 原生字符串的封装，结构定义如下：

```c
// sds 是 char * 的类型别名，用于指向 sdshdr 头部的 buf 字符串
typedef char *sds;

// Redis 保存字符串对象的结构
struct sdshdr {
	int len;    // buf 已占用空间长度
	int free;   // buf 剩余可用空间长度
	char buf[]; // sds 二进制字节数组，C99 支持将 struct 最后一个成员定义为无长度数组，不自动分配内存
};
```

- len：有符号 `int` 类型，占 4 字节，最大能表示 `2^31 B` = `2^21 KB` = `2^11 MB` = `2 GB` 大的数据。但 Redis 把单 key 字符串值最长限制在了 `512MB`。参考：[maximum-value-size-in-redis](https://stackoverflow.com/questions/5606106/what-is-the-maximum-value-size-you-can-store-in-redis)

- buf：声明时无长度的柔性数组，是 C99 标准中的不完整类型，虽然结构体中的各字段在内存上是连续的，但柔性数组空间并不计入结构体总内存：

  ```c
  printf("%zu\n", sizeof(struct sdshdr)); // 8 
  ```



### 内存布局

假设存入字符串 "Redis"，其内存布局如下：

![image-20190919175541169](https://images.yinzige.com/2019-09-19-095541.png)

64 位 Linux 上各内存段长度验证：

```c
int main() {
	char *sds = sdsnew("Redis");
	char *free = sds - sizeof(int);
	char *len = free - sizeof(int);
	char *prefixSize = len - sizeof(size_t);
	printf("used_memory: %zu\n", zmalloc_used_memory());
	printf("prefix_size: %d\nlen: %d\nfree: %d\n", *prefixSize, *len, *free);
}
```

 <img src="https://images.yinzige.com/2019-09-19-080001.png" width=65% />





## SDS API 实现

源码：[sds.c](https://github.com/wuYin/redis-3.0/blob/master/sds.c)，几个重要的 API 实现：

### sdslen

O(1) 复杂度返回字符串长度。直接让 SDS 左移 2 个 int 长度寻址，读取字符串的长度后返回：

```c
static inline size_t sdslen(const sds s) {
	struct sdshdr *sh = (void *) (s - (sizeof(struct sdshdr))); // sds - 8
	return sh->len;
}
```



### sdsnew

注意 `zmalloc` 没有为 buf 柔性数组分配内存，其值是用 `memcpy` 高效复制字符串的值进行初始化的：

```c
// 新建 sds
sds sdsnew(const char *init) {
	size_t initlen = (init == NULL) ? 0 : strlen(init);
	return sdsnewlen(init, initlen);
}

// 根据字符串 init 及其长度 initlen 创建 sds
// 成功则返回 sdshdr 地址，失败返回 NULL
sds sdsnewlen(const void *init, size_t initlen) {
	struct sdshdr *sh;
	if (init) {
		sh = zmalloc(sizeof(struct sdshdr) + initlen + 1); // 有值则不初始化内存，+1 是为 '\0' 预留
	} else {
		sh = zcalloc(sizeof(struct sdshdr) + initlen + 1); // 空字符串则 SDS 初始化为全零
	}
	if (sh == NULL) return NULL;

	sh->len = initlen;
	sh->free = 0; // 新 sds 不预留空闲空间
	if (initlen && init)
		memcpy(sh->buf, init, initlen); // 复制字符串 init 到 buf

	sh->buf[initlen] = '\0'; // 以 \0 结尾
	return (char *) sh->buf; // buf 部分即 sds
}
```



### sdsclear

惰性删除，将 SDS 清空为空字符串，未释放的空间会保留给下次分配使用：

```c
void sdsclear(sds s) {
	struct sdshdr *sh = (void *) (s - (sizeof(struct sdshdr)));
	sh->free += sh->len; // 全部可用
	sh->len = 0;
	sh->buf[0] = '\0'; // 手动截断 buf
}
```



### sdsMakeRoomFor

Redis 的内存预分配策略根据新内存字节数决定：

-  `[0, 1 MB)`：翻倍增长
- `[1, ∞)`：每次仅增长 1 MB

```c
// 扩展 sds 空间增加 addlen 长度，进行内存预分配
sds sdsMakeRoomFor(sds s, size_t addlen) {

	struct sdshdr *sh, *newsh;
	size_t free = sdsavail(s);
	size_t len, newlen;

	if (free >= addlen) return s; // sdsclear 惰性删除保留的内存够用，无须扩展

	len = sdslen(s);
	sh = (void *) (s - (sizeof(struct sdshdr)));

	newlen = (len + addlen); // 新长度不把 free 算入，和初始化时一样恰好够用就行

	// 空间预分配策略：新长度在 (..., 1MB) 则成倍增长，[1MB, ...) 则每次仅增长 1 MB
	if (newlen < SDS_MAX_PREALLOC)
		newlen *= 2;
	else
		newlen += SDS_MAX_PREALLOC;

	// 重分配
	newsh = zrealloc(sh, sizeof(struct sdshdr) + newlen + 1);
	if (newsh == NULL) return NULL;

	newsh->free = newlen - len; // 更新 free 但不更新 len
	return newsh->buf;
}
```



### sdscat

用 `memcpy` 高效地拷贝内存，将字符串连接到 SDS 尾部，其使用 `sdsMakeRoomFor` 进行空间预分配：

```c
// 将长度为 len 的字符串 t 追加到 sds
sds sdscatlen(sds s, const void *t, size_t len) {

	struct sdshdr *sh;
	size_t curlen = sdslen(s);
	s = sdsMakeRoomFor(s, len);
	if (s == NULL) return NULL;

	sh = (void *) (s - (sizeof(struct sdshdr)));
	memcpy(s + curlen, t, len);  // 复制 t 中的内容到字符串后部

	sh->len = curlen + len;
	sh->free = sh->free - len;
	s[curlen + len] = '\0';

	return s;
}

// 追加字符串到 sds
sds sdscat(sds s, const char *t) {
	return sdscatlen(s, t, strlen(t));
}
```

注意相近的 `sdscpy` 函数会将字符串覆盖式地拷贝到 SDS 中。



### sdsfree

释放 SDS 整块内存：

```c
void sdsfree(sds s) {
	if (s == NULL) return;
	zfree(s - sizeof(struct sdshdr)); // 同样左移寻址
}
```



## SDS 优点

结合如上的 API 实现，总结下 SDS 相比 C 原生字符串的 4 个优点：

### O(1) 复杂度获取字符串长度

C 字符串是最后一个元素为 `\0` 的字符数组，获取长度需从头到尾 O(N) 遍历。

SDS 结构中用 len 字段记录了字符串长度，并在各种增删操作中动态维护其长度，使用 `strlen` 直接读取字段值即可。



### 避免缓冲区溢出

C 字符串的 `strcpy` 操作，若 `dst` 未分配足够内存，应用有 crash 或被缓冲区溢出攻击的风险。

SDS 在操作字符串前会检查其空间，若不够则预分配，从根源上杜绝溢出。



### 二进制安全

C 以 `'\0'` 作为字符串分隔符，故不能保存如图片等穿插大量空字符的数据。

SDS API 用 len 长度来界定字符串边界，存入什么就取出什么，故操作二进制数据是安全的。但 SDS 同样以 `'\0'` 作为字符串的分隔符，方便直接重用 `string.h` 中的丰富库函数。



### 内存预分配与惰性释放

C 的字符串每次增长或缩减，都要 realloc 重分配内存。

SDS 每次以成倍或加 1MB 的方式扩展空间，且清空时不释放内存预留下次使用。从而将 N 次字符串操作，内存重分配次数从必定 N 次减少为最多 N 次。



## 总结

Redis 封装 C 原生字符串为 SDS，并实现了取长度、复制、比较、内存预分配等 API 供上层使用，可以看到 API 实现中对 `buf` 直接进行内存拷贝等操作，十分高效。

因为封装，SDS 相比原生字符串中间隔了一层取地址等操作，但其 API 耗时并未成为 Redis 的性能瓶颈，设计上十分精巧。