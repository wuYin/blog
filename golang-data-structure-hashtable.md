---
title: golang-data-structure-hashtable
date: 2018-01-31 08:58:53
tags: data structure
---

Golang 中链表的实现及常用操作，数据结构系列原文：[flaviocopes.com](https://flaviocopes.com/golang-data-structures/)，翻译已获作者授权。

<!-- more -->



#### 概述

哈希表是和 `map` 类型的键值对存储方式不同（PHP 中的关联数组），它的哈希函数能根据 key 值计算出 key 在数组中的切确位置（索引）。

区别哈希表与 Golang 的 map、PHP 中的关联数组：

 ![](https://contents.yinzige.com/hash-table.png)



#### 实现

##### 常用操作

使用内置的 `map` 类型来实现哈希表，并实现以下常用操作：

```go
Put()
Remove()
Get()
Size()
```

类似的，创建通用类型 `ValueHashTable` 来作为哈希表的结构类型，其中键值需实现 `Stringer` 接口。

##### 代码实现

```go
package hashtable

import (
	"github.com/cheekybits/genny/generic"
	"sync"
	"fmt"
)

type Key generic.Type
type Value generic.Type

type ValueHashTable struct {
	items map[int]Value
	lock  sync.RWMutex
}

// 使用霍纳规则在 O(n) 复杂度内生成 key 的哈希值
func hash(k Key) int {
	key := fmt.Sprintf("%s", k)
	hash := 0
	for i := 0; i < len(key); i++ {
		hash = 31*hash + int(key[i])
	}
	return hash
}

// 新增键值
func (ht *ValueHashTable) Put(k Key, v Value) {
	ht.lock.Lock()
	defer ht.lock.Unlock()
	h := hash(k)
	if ht.items == nil {
		ht.items = make(map[int]Value)
	}
	ht.items[h] = v
}

// 删除键
func (ht *ValueHashTable) Remove(k Key) {
	ht.lock.Lock()
	defer ht.lock.Unlock()
	h := hash(k)
	delete(ht.items, h)
}

// 获取键的哈希值
func (ht *ValueHashTable) Get(k Key) Value {
	ht.lock.RLock()
	defer ht.lock.RUnlock()
	h := hash(k)
	return ht.items[h]
}

// 获取哈希表的大小
func (ht *ValueHashTable) Size() int {
	ht.lock.RLock()
	defer ht.lock.RUnlock()
	return len(ht.items)
}
```

测试用例：[hashtable_test.go](https://github.com/wuYinBest/blog/blob/master/codes/golang-data-structure-hashtable/hashtable_test.go)







































