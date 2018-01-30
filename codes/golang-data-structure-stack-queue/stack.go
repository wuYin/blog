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
