package tree

import (
	"testing"
	"fmt"
)

var tree ItemBinarySearchTree

func initTree(tree *ItemBinarySearchTree) {
	tree.Insert(8, "8")
	tree.Insert(4, "4")
	tree.Insert(10, "10")
	tree.Insert(2, "2")
	tree.Insert(6, "6")
	tree.Insert(1, "1")
	tree.Insert(3, "3")
	tree.Insert(5, "5")
	tree.Insert(7, "7")
	tree.Insert(9, "9")
}

func TestInsert(t *testing.T) {
	initTree(&tree)
	tree.String()
	tree.Insert(11, "11")
	tree.String()
}

func TestPreOrderTraverse(t *testing.T) {
	traverse := ""
	tree.PreOrderTraverse(func(value Item) {
		traverse += fmt.Sprintf("%s\t", value)
	})
	println(traverse)
}

func TestInOrderTraverse(t *testing.T) {
	traverse := ""
	tree.InOrderTraverse(func(value Item) {
		traverse += fmt.Sprintf("%s\t", value)
	})
	println(traverse)
}

func TestPostOrderTraverse(t *testing.T) {
	traverse := ""
	tree.PostOrderTraverse(func(value Item) {
		traverse += fmt.Sprintf("%s\t", value)
	})
	println(traverse)
}

func TestMin(t *testing.T) {
	min := *tree.Min()
	if fmt.Sprintf("%s", min) != "1" {
		t.Errorf("Min() should return 1 but return %s", min)
	}
}

func TestMax(t *testing.T) {
	max := *tree.Max()
	if fmt.Sprintf("%s", max) != "11" {
		t.Errorf("Max() should return 11 but return %s", max)
	}
}

func TestSearch(t *testing.T) {
	for i := 1; i <= 11; i++ {
		if !tree.Search(i) {
			t.Errorf("Search() can't find %d", i)
		}
	}
}

func TestRemove(t *testing.T) {
	tree.Remove(1)
	if fmt.Sprintf("%s", *tree.Min()) != "2" {
		t.Errorf("Remove(1) failed")
	}
}
