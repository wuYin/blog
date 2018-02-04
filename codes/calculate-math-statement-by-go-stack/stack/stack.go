package stack

import (
	"sync"
)

type Item string

type ItemStack struct {
	items []string
	lock  sync.RWMutex
}

// 创建栈
func (s *ItemStack) New() *ItemStack {
	s.items = []string{}
	return s
}

// 入栈
func (s *ItemStack) Push(t string) {
	s.lock.Lock()
	s.items = append(s.items, t)
	s.lock.Unlock()
}

// 出栈
func (s *ItemStack) Pop() string {
	s.lock.Lock()
	item := s.items[len(s.items)-1]
	s.items = s.items[0:len(s.items)-1 ]
	s.lock.Unlock()
	return item
}

// 取栈顶
func (s *ItemStack) Top() string {
	return s.items[len(s.items)-1]
}

// 判空
func (s *ItemStack) IsEmpty() bool {
	return len(s.items) == 0
}
