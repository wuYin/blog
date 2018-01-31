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
