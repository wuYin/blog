---
title: Golang 数据结构：栈与队列
date: 2018-01-30 15:22:28
tags: data structure
---



Golang 中栈、队列的实现及常用操作，数据结构系列原文：[flaviocopes.com](https://flaviocopes.com/golang-data-structures/)，翻译已获作者授权。

<!-- more -->



#### 栈

##### 概述

栈是数据按照后进先出 LIFO(Last-In-First-Out) 原则组成的集合。添加和移除元素都是在栈顶进行，类比书堆，不能在栈底增删元素。

栈的应用很广泛，比如网页跳转后一层层返回，ctrl+z 撤销操作等。

使用 `slice` 动态类型来实现栈，栈元素的类型是使用 [genny](https://github.com/cheekybits/genny) 创建的通用类型 `ItemStack`，实现以下常用操作：

```go
New()	// 生成栈的构造器
Push()	
Pull()
```



##### 代码实现

```go
package stack

import (
	"github.com/cheekybits/genny/generic"
	"sync"
)

type Item generic.Type

type ItemStack struct {
	items []Item
	lock  sync.RWMutex
}

// 创建栈
func (s *ItemStack) New() *ItemStack {
	s.items = []Item{}
	return s
}

// 入栈
func (s *ItemStack) Push(t Item) {
	s.lock.Lock()
	s.items = append(s.items, t)
	s.lock.Unlock()
}

// 出栈
func (s *ItemStack) Pop() *Item {
	s.lock.Lock()
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1 ]
	s.lock.Unlock()
	return &item
}
```

测试用例：[stack_test.go](https://github.com/wuYinBest/blog/blob/master/codes/golang-data-structure-stack-queue/stack_test.go)



---



#### 队列

##### 概述

队列是数据按照先进先出 FIFO(First-In-First-Out) 原则组成的集合，类比排队，在队列任一端添加元素，从对应的另一端删除元素。

使用 `slice` 动态类型来实现队列，元素的类型为通用类型 `ItemQueue` ，实现以下常用操作：

```go
New()		// 生成队列的构造器
Enqueue()
Dequeue()
Front()
IsEmpty()
Size()
```



##### 代码实现

```go
package queue

import (
	"github.com/cheekybits/genny/generic"
	"sync"
)

type Item generic.Type

type ItemQueue struct {
	items []Item
	lock  sync.RWMutex
}

// 创建队列
func (q *ItemQueue) New() *ItemQueue {
	q.items = []Item{}
	return q
}

// 如队列
func (q *ItemQueue) Enqueue(t Item) {
	q.lock.Lock()
	q.items = append(q.items, t)
	q.lock.Unlock()
}

// 出队列
func (q *ItemQueue) Dequeue() *Item {
	q.lock.Lock()
	item := q.items[0]
	q.items = q.items[1:len(q.items)]
	q.lock.Unlock()
	return &item
}

// 获取队列的第一个元素，不移除
func (q *ItemQueue) Front() *Item {
	q.lock.Lock()
	item := q.items[0]
	q.lock.Unlock()
	return &item
}

// 判空
func (q *ItemQueue) IsEmpty() bool {
	return len(q.items) == 0
}

// 获取队列的长度
func (q *ItemQueue) Size() int {
	return len(q.items)
}
```

测试用例：[queue_test.go](https://github.com/wuYinBest/blog/blob/master/codes/golang-data-structure-stack-queue/queue_test.go)



















