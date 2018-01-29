package linkedlist

import (
	"testing"
	"fmt"
)

var list ItemLinkedList

func TestAppend(t *testing.T) {
	if !list.IsEmpty() {
		t.Errorf("Linked list should be empty")
	}
	list.Append("first")
	if list.IsEmpty() {
		t.Errorf("Linked list should not be empty")
	}
	if size := list.Size(); size != 1 {
		t.Errorf("Wrong count, expected 1 but got %d", size)
	}

	list.Append("second")
	list.Append("third")
	if size := list.Size(); size != 3 {
		t.Errorf("Wrong count, expected 3 but got %d", size)
	}
}

func TestRemoveAt(t *testing.T) {
	_, err := list.RemoveAt(1) // 删除 second
	if err != nil {
		t.Errorf("Unexcepted error: %s", err)
	}
	if size := list.Size(); size != 2 {
		t.Errorf("Wrong count, expected 2 but got %d", size)
	}
}

func TestInsert(t *testing.T) {
	// 测试插入到链表中间
	err := list.Insert(2, "second2")
	if err != nil {
		t.Errorf("Unexcepted error: %s", err)
	}
	if size := list.Size(); size != 3 {
		t.Errorf("Wrong count, expected 3 but got %d", size)
	}
	// 测试插入到链表两侧
	err = list.Insert(0, "zero")
	if err != nil {
		t.Errorf("Unexcepted error: %s", err)
	}
}

func TestIndexOf(t *testing.T) {
	if i := list.IndexOf("zero"); i != 0 {
		t.Errorf("Excepted postion 0 but got: %d", i)
	}
	if i := list.IndexOf("first"); i != 1 {
		t.Errorf("Excepted postion 1 but got: %d", i)
	}
	if i := list.IndexOf("second2"); i != 2 {
		t.Errorf("Excepted postion 2 but got: %d", i)
	}
	if i := list.IndexOf("third"); i != 3 {
		t.Errorf("Excepted postion 3 but got: %d", i)
	}
}

func TestHead(t *testing.T) {
	head := list.head
	content := fmt.Sprint(head.content)
	if content != "zero" {
		t.Errorf("Excepted `zero` but got: %s", content)
	}
}
