package stack

import "testing"

var stack ItemStack

// 初始化栈
func initStack() *ItemStack {
	if stack.items == nil {
		stack = ItemStack{}
		stack.New()
	}
	return &stack
}

func TestPush(t *testing.T) {
	stack := initStack()
	stack.Push(1)
	stack.Push(2)
	stack.Push(3)
	if size := len(stack.items); size != 3 {
		t.Errorf("Wrong stack size, expected 3 and got %d", size)
	}
}

func TestPop(t *testing.T) {
	stack.Pop()
	if size := len(stack.items); size != 2 {
		t.Errorf("Wrong stack size, expected 2 and got %d", size)
	}

	stack.Pop()
	stack.Pop()
	if size := len(stack.items); size != 0 {
		t.Errorf("Wrong stack size, expected 0 and got %d", size)
	}
}
