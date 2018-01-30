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
