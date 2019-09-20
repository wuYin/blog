---
title: Redis 双端链表
date: 2019-09-19 20:15:48
tags: Redis
---

总结 Redis 双端链表的实现。

<!-- more -->

## 函数指针

Redis 链表结构内置了三个指向操作函数的函数指针，先梳理下函数指针用法。

### 定义

函数体在编译期间会被存入进程代码段内存的一片连续区域中，而函数名就是该区域的起始地址。可将该地址赋值给函数指针，通过指针间接调用函数。

```c
int sum(int x, int y) { return x + y;}

int main() {
    int (*p)(int, int) = sum;
    printf("%p\n%p\n", sum, p);
    printf("%d\n%d\n%d\n", p(1, 1), (*p)(1, 1), (****p)(1, 1)); // 函数指针可以无限解引用
}
```

地址相同：

 <img src="https://images.yinzige.com/2019-09-19-130042.png" width=40% />

### 用法

比如 `qsort` 库函数第四个参数便是函数指针：`int (* cmp)(const void *, const void *));`，用于自定义排序规则：

```c
int less(const void *a, const void *b) {
    return *(int *) a > *(int *) b; // void* 泛型指针强转为 int* 指针后取值比较
}

int more(const void *a, const void *b) {    
  return *(int *) a < *(int *) b; // 从大到小排序
}

int main() {
    int nums[5] = {4, 1, 5, 3, 2};
    qsort(nums, 5, sizeof(int), less);  debug(nums);
    qsort(nums, 5, sizeof(int), more);  debug(nums);
}
```

排序结果：

 <img  src="https://images.yinzige.com/2019-09-19-131821.png" width=50% />

注：`void *` 为泛型指针，类似 Go 的 `interface`，本节链表节点的值为 `void*` 类型，可存储任意类型的值。

### 区分

根据 `()` 逐个拆解：

```c
int (*p)(int); // p 是指向 int f(int) 函数的指针
int *(*p)(int); // p 是指向 int* f(int) 函数的指针
int (**p)(int); // p 是指向 int f(int) 函数指针的指针
```



## Redis 双端链表

### 优缺点

- 优点：增删节点无需内存重排，直接操作复杂度为 `O(1)`。维护了 `len` 字段，取长度操作为 `O(1)`
- 缺点：由于各节点在内存上不连续，遍历搜索复杂度为 `O(N)`。但可以通过空间换时间，使用 `O(N)` 空间的哈希表，可将搜索复杂度降为 `O(1)`，这点在链表实现 LRU 时有应用。

### 结构

```c
// 链表节点结构
typedef struct listNode {
    struct listNode *prev; // 前驱节点
    struct listNode *next; // 后继节点
    void *value;           // 节点值，类型为 void*，可保存任意类型的数据
} listNode;

// 双端链表结构，注意是无环的
typedef struct list {
    listNode *head; // 表头节点
    listNode *tail; // 表尾节点

    void *(*dup)(void *ptr); // 节点值复制函数
    void (*free)(void *ptr); // 节点值释放函数
    int (*match)(void *ptr, void *key); // 节点值对比函数

    unsigned long len; // 链表所包含的节点数量
} list;
```

注意节点的结构标记是 `listNode`，前驱和后继节点为自引用结构。同时声明该类型的同名变量 `listNode`

### 内存布局

![image-20190920095856353](https://images.yinzige.com/2019-09-20-015856.png)



## list API 实现

记录几个重要 API 实现，源码：[adlist.c](https://github.com/wuYin/redis-3.0/blob/master/adlist.c)

### O(1) 操作

```C
#define listFirst(l) ((l)->head) // 返回给定链表的表头节点
#define listLast(l) ((l)->tail) // 返回给定链表的表尾节点
#define listPrevNode(n) ((n)->prev) // 返回给定节点的前置节点
#define listNextNode(n) ((n)->next) // 返回给定节点的后置节点
#define listNodeValue(n) ((n)->value) // 返回给定节点的值

#define listSetDupMethod(l, m) ((l)->dup = (m)) // 将链表 l 的值复制函数设置为 m
#define listGetDupMethod(l) ((l)->dup) // 返回给定链表的值复制函数
// ...
```

注意取 list 字段值的简单操作都是宏定义，且对参数 `n` 加括号避免了展开优先级错乱的问题。



### listCreate

```c
// 创建新链表
list *listCreate(void) {
    struct list *list;
    if ((list = zmalloc(sizeof(*list))) == NULL) // 分配内存
        return NULL;

  // ...
    return list;
}
```

注意 zmalloc 分配内存后返回 `void*` 指针，可赋值给其他任意类型的指针，在 C 中无需显式强转，但 C++ 中必须显式转换。



### listInsertNode

```c
// 创建 value 新节点，并插入到 old_node 的之前或之后
list *listInsertNode(list *list, listNode *old_node, void *value, int after) {
    listNode *node;

    if ((node = zmalloc(sizeof(*node))) == NULL)
        return NULL;
    node->value = value;

    // 添加到 old_node 之后
    if (after) {
        node->prev = old_node;
        node->next = old_node->next;
        if (list->tail == old_node) { // old_node 是表尾节点
            list->tail = node;
        }
        // 添加到 old_node 之前
    } else {
        node->next = old_node;
        node->prev = old_node->prev;
        if (list->head == old_node) { // old_node 是表头节点
            list->head = node;
        }
    }

    if (node->prev != NULL) { // 更新新节点的前置指针
        node->prev->next = node;
    }
    if (node->next != NULL) { // 更新后置指针
        node->next->prev = node;
    }

    list->len++; // 更新链表节点数 
    return list;
}
```

和刷 LeetCode 时链表题一样，直接画图最好理解。对链表的操作需检查：

- 链表是否为空
- 操作的节点是否为头尾节点
- 调整节点前后指针指向时顺序是否正确



### listIter

Redis 为链表封装了一个迭代器：

```c
typedef struct listIter {
    listNode *next; // 当前迭代到的节点
    int direction; // 迭代的方向，常量 0 向后，1 向前
} listIter;
```

用于链表的搜索等操作：

```c
listNode *listSearchKey(list *list, void *key) {
    listIter *iter;
    listNode *node;

    // 迭代整个链表
    iter = listGetIterator(list, AL_START_HEAD);
    while ((node = listNext(iter)) != NULL) {
        // 定义过值比较函数则调用
        if (list->match) {
            if (list->match(node->value, key)) { // 匹配则释放迭代器
                listReleaseIterator(iter);
                return node;
            }
        } else {
            if (key == node->value) {
                listReleaseIterator(iter); // 否则就强制地址是否相同
                return node;
            }
        }
    }

    listReleaseIterator(iter);
    return NULL; // 未找到
}
```

此处用到了 `match` 函数指针，其内部会对类型均为 `void*` 的 value, key 做类型转换，返回值是否相等的结果。

`listIter` 负责迭代，`list` 负责存储，避免了手动迭代中各种临时变量带来的开销，按功能分离模块的设计很优秀。



## 总结

Redis list 是双向无环链表，节点值类型是 `void*`，类似于 Go 的 `interface`，所以在比较节点值时需调用自定义比较函数，故引入了三个函数指针。Redis 还将迭代功能的实现分离到了 `listIter`，比较巧妙。