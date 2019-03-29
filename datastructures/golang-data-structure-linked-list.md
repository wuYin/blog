---
title: Golang 数据结构：链表
date: 2018-01-29 18:30:28
tags: data structure
---



Golang 中链表的实现及常用操作，数据结构系列原文：[flaviocopes.com](https://flaviocopes.com/golang-data-structure-linked-list/)，翻译已获作者授权。

<!-- more -->



#### 前言

链表的结构类似于数组，但插入元素的代价比数组小得多，因为在数组中插入元素，需要把插入位置后边所有的元素后移一个位置，删除元素则要全部前移。

数组将元素顺序存储在内存单元中（静态分配内存），而链表是通过元素中的指针，将元素存储在零散的内存中（动态分配内存）

链表相比数组有个明显的缺点：查找元素时不知道元素在链表中的地址，需要从第一个元素开始遍历整条链表来寻找。



#### 链表结构

##### 基本操作：

```go
Append(t)		// 将元素 t 追加到链表尾部
Insert(i, t)    	// 在位置 i 处插入元素 t
RemoveAt(i)		// 移除位置 i 的元素
IndexOf(t)		// 返回元素 t 的位置
IsEmpty(l)		// l 是空链表则返回 true
Size()			// 返回链表的长度
String()		// 返回链表的字符串表示
Head()			// 返回链表的首结点，以便迭代链表
```

我使用 [genny](https://github.com/cheekybits/genny) 来创建了一个通用的类型： `ItemLinkedList`，使得链表能存储任意数据类型的元素。这样能将链表的实现和具体存储的数据分离开，高度封装了链表的实现。

##### 代码实现

```go
// linkedlist 包为 Item 类型的元素创建一个 ItemLinkedList 链表
package linkedlist

import (
	"github.com/cheekybits/genny/generic"
	"sync"
	"fmt"
)

type Item generic.Type

type Node struct {
	content Item
	next    *Node
}

type ItemLinkedList struct {
	head *Node
	size int
	lock sync.RWMutex
}

// 在链表结尾追加元素
func (list *ItemLinkedList) Append(t Item) {
	list.lock.Lock()
	newNode := Node{t, nil}

	// 查找并追加
	if list.head == nil { // 空链表第一次追加元素
		list.head = &newNode
	} else {
		curNode := list.head // 遍历链表，找到尾部结点
		for {
			if curNode.next == nil {
				break
			}
			curNode = curNode.next
		}
		curNode.next = &newNode
	}

	// 追加后链表长度+1
	list.size++
	list.lock.Unlock()
}

// 在链表指定位置插入指定元素
func (list *ItemLinkedList) Insert(i int, t Item) error {
	list.lock.Lock()
	defer list.lock.Unlock()
	if i < 0 || i > list.size {
		return fmt.Errorf("Index %d out of bonuds", i)
	}
	newNode := Node{t, nil}

	if i == 0 { // 插入到链表头部
		newNode.next = list.head
		list.head = &newNode
		list.size++
		return nil
	}

	preNode := list.head
	preIndex := 0
	for preIndex < i-2 {
		preIndex++
		preNode = preNode.next
	}
	// 执行插入
	newNode.next = preNode.next
	preNode.next = &newNode
	list.size++
	return nil
}

// 删除指定位置的元素
func (list *ItemLinkedList) RemoveAt(i int) (*Item, error) {
	list.lock.Lock()
	defer list.lock.Unlock()

	if i < 0 || i > list.size {
		return nil, fmt.Errorf("Index %d out of bonuds", i)
	}

	curNode := list.head
	preIndex := 0
	for preIndex < i-1 {
		preIndex++
		curNode = curNode.next
	}
	item := curNode.content
	curNode.next = curNode.next.next
	list.size--
	return &item, nil
}

// 获取指定元素在链表中的索引
func (list *ItemLinkedList) IndexOf(t Item) int {
	list.lock.RLock()
	defer list.lock.RUnlock()
	curNode := list.head
	locIndex := 0
	for {
		if curNode.content == t {
			return locIndex
		}
		if curNode.next == nil {
			return -1
		}
		curNode = curNode.next
		locIndex++
	}
}

// 检查链表是否为空
func (list *ItemLinkedList) IsEmpty() bool {
	list.lock.RLock()
	defer list.lock.RUnlock()
	if list.head == nil {
		return true
	}
	return false
}

// 获取链表的长度
func (list *ItemLinkedList) Size() int {
	list.lock.RLock()
	defer list.lock.RUnlock()
	size := 1
	nextNode := list.head
	for {
		if nextNode == nil || nextNode.next == nil { // 单结点链表的 nextNode == nil
			break
		}
		size++
		nextNode = nextNode.next
	}
	return size
}

// 格式化打印链表
func (list *ItemLinkedList) String() {
	list.lock.RLock()
	defer list.lock.RUnlock()
	curNode := list.head
	for {
		if curNode == nil {
			break
		}
		print(curNode.content)
		print(" ")
		curNode = curNode.next
	}
	println()
}

// 获取链表的头结点
func (list *ItemLinkedList) Head() *Node {
	list.lock.RLock()
	defer list.lock.RUnlock()
	return list.head
}
```



##### 测试用例：[linkedlist_test.go](https://github.com/wuYinBest/blog/blob/master/codes/golang-data-structure-linked-list/linkedlist_test.go)

 ![](https://contents.yinzige.com/testing-pass.png)



##### 使用

通过 `generate` 来为链表指定元素具体的数据类型，如：

```go
//generate a `IntLinkedList` linked list of `int` values
genny -in linkedlist.go -out linkedlist_int.go gen "Item=int"
```

