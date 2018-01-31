package hashtable

import (
	"fmt"
	"testing"
)

func generateHashTable(count int, start int) *ValueHashTable {
	ht := ValueHashTable{}
	for i := start; i < (start + count); i++ {
		ht.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}
	return &ht
}

func TestPut(t *testing.T) {
	ht := generateHashTable(3, 0)
	if size := ht.Size(); size != 3 {
		t.Errorf("Wrong count, expected 3 and got %d", size)
	}
	ht.Put("key1", "value1") // 修改已存在的值
	if size := ht.Size(); size != 3 {
		t.Errorf("Wrong count, expected 3 and got %d", size)
	}
	ht.Put("key4", "value4")
	if size := ht.Size(); size != 4 {
		t.Errorf("Wrong count, expected 4 and got %d", size)
	}
}

func TestRemove(t *testing.T) {
	ht := generateHashTable(3, 0)
	ht.Remove("key2")
	if size := ht.Size(); size != 2 {
		t.Errorf("Wrong count, expected 2 and got %d", size)
	}
}
