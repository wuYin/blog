---
title: Golang 数据结构：二叉搜索树
date: 2018-02-05 18:09:01
tags: data-structure
---

Golang 中二叉搜索树的实现及常用操作，数据结构系列原文：[flaviocopes.com](https://flaviocopes.com/golang-data-structure-binary-search-tree/)，翻译已获作者授权。

<!-- more -->

### 概念

树（tree）：一种分层的数据结构，类比家谱

二叉树（binary tree）：每个节点最多只有 2 个子节点的树

二叉搜索树（binary search tree）：左节点的值均小于右节点值的二叉树

- 深度（depth）：从 root 根结点到当前节点唯一路径的长度
- 高度（height）：从当前节点到一片树叶最长的路径的长度

- 根（Root）：深度为 0 的树节点
- 内部节点（Internal node）：至少有一个子节点的节点
- 树叶（Leaf）：无子节点的节点
- 兄弟节点（sibling）：拥有相同父节点的子节点

![](https://contents.yinzige.com/concept.png)



### 二叉搜索树

#### 常用操作与节点定义

```go
Insert(v)		// 向二叉搜索树的合适位置插入节点
Search(k)		// 检查序号为 k 的元素在树中是否存在
Remove(v)		// 移除树中所有值为 v 的节点
Min()			// 获取二叉搜索树中最小的值
Max()			// 获取二叉搜索树中最大的值
InOrderTraverse()	// 中序遍历树
PreOrderTraverse()	// 先序遍历树
PostOrderTraverse()	// 后续遍历树
String()		// 在命令行格式化打印出二叉树
```

同样使用 genny 提供代码的复用性，树类型命名为： `ItemBinarySearchTree`，树节点的结构体定义如下：

```go
type Node struct {
	key   int	// 中序遍历的节点序号
	value Item	// 节点存储的值
	left  *Node	// 左子节点
	right *Node	// 右子节点
}
```

key 是各节点的位置在先序遍历中的序号，key 的值这里使用 int，可以是任意可比较的数据类型。

#### 插入操作与遍历

插入操作需要使用到递归，插入操作需要从上到下查找新节点在树中合适的位置：新节点的值小于任意节点，则向左子树继续寻找，同理向右子树查找，直到查找到树叶节点再插入。

遍历操作有三种方式：

- 中序遍历（in-order）：左子树-->根结点--> 右树：`1->2->3->4->5->6->7->8->9->10->11`

- 先序遍历（pre-order）：根结点-->左子树-->右子树：`8->4->2->1->3->6->5->7 >10->9->11`

- 后序遍历（post-order）：左子树-->右子树-->根结点：`1->3->2->5->7->6->4->9->11->10->8`

  ​

 ![](https://contents.yinzige.com/bstree.png)



`String()` 可视化树结构：

 ![](https://contents.yinzige.com/visual-tree.png)



<br/>

### 代码实现

#### Insert

```go
// 向二叉搜索树的合适位置插入节点
func (tree *ItemBinarySearchTree) Insert(key int, value Item) {
	tree.lock.Lock()
	defer tree.lock.Unlock()
	newNode := &Node{key, value, nil, nil}
	// 初始化树
	if tree.root == nil {
		tree.root = newNode
	} else {
		// 在树中递归查找正确的位置并插入
		insertNode(tree.root, newNode)
	}
}

func insertNode(node, newNode *Node) {
	// 插入到左子树
	if newNode.key < node.key {
		if node.left == nil {
			node.left = newNode
		} else {
			// 递归查找左边插入
			insertNode(node.left, newNode)
		}
	} else {
		// 插入到右子树
		if node.right == nil {
			node.right = newNode
		} else {
			// 递归查找右边插入
			insertNode(node.right, newNode)
		}
	}
}

```



#### Search

```go
// 检查序号为 k 的元素在树中是否存在
func (tree *ItemBinarySearchTree) Search(key int) bool {
	tree.lock.RLock()
	defer tree.lock.RUnlock()
	return search(tree.root, key)
}
func search(node *Node, key int) bool {
	if node == nil {
		return false
	}
	// 向左搜索更小的值
	if key < node.key {
		return search(node.left, key)
	}
	// 向右搜索更大的值
	if key > node.key {
		return search(node.right, key)
	}
	return true // key == node.key
}
```



#### Remove

##### 删除节点的流程

先递归查找，再删除节点。但在删除时需根据节点拥有子节点的数量，分如下 3 种情况：

![](https://contents.yinzige.com/remove-node.png)

##### 代码实现

```go
// 删除指定序号的节点
func (tree *ItemBinarySearchTree) Remove(key int) {
	tree.lock.Lock()
	defer tree.lock.Unlock()
	remove(tree.root, key)
}

// 递归删除节点
func remove(node *Node, key int) *Node {
	// 要删除的节点不存在
	if node == nil {
		return nil
	}

	// 寻找节点
	// 要删除的节点在左侧
	if key < node.key {
		node.left = remove(node.left, key)
		return node
	}
	// 要删除的节点在右侧
	if key > node.key {
		node.right = remove(node.right, key)
		return node
	}

	// 判断节点类型
	// 要删除的节点是叶子节点，直接删除
	// if key == node.key {
	if node.left == nil && node.right == nil {
		node = nil
		return node
	}

	// 要删除的节点只有一个节点，删除自身
	if node.left == nil {
		node = node.right
		return node
	}
	if node.right == nil {
		node = node.left
		return node
	}

	// 要删除的节点有 2 个子节点，找到右子树的最左节点，替换当前节点
	mostLeftNode := node.right
	for {
		// 一直遍历找到最左节点
		if mostLeftNode != nil && mostLeftNode.left != nil {
			mostLeftNode = mostLeftNode.left
		} else {
			break
		}
	}
	// 使用右子树的最左节点替换当前节点，即删除当前节点
	node.key, node.value = mostLeftNode.key, mostLeftNode.value
	node.right = remove(node.right, node.key)
	return node
}
```



#### Min、Max

```go
// 获取树中值最小的节点：最左节点
func (tree *ItemBinarySearchTree) Min() *Item {
	tree.lock.RLock()
	defer tree.lock.RUnlock()
	node := tree.root
	if node == nil {
		return nil
	}
	for {
		if node.left == nil {
			return &node.value
		}
		node = node.left
	}
}

// 获取树中值最大的节点：最右节点
func (tree *ItemBinarySearchTree) Max() *Item {
	tree.lock.RLock()
	defer tree.lock.RUnlock()
	node := tree.root
	if node == nil {
		return nil
	}
	for {
		if node.right == nil {
			return &node.value
		}
		node = node.right
	}
}
```



#### Traverse 

```go
// 先序遍历：根节点 -> 左子树 -> 右子树
func (tree *ItemBinarySearchTree) PreOrderTraverse(printFunc func(Item)) {
	tree.lock.RLock()
	defer tree.lock.RUnlock()
	preOrderTraverse(tree.root, printFunc)
}
func preOrderTraverse(node *Node, printFunc func(Item)) {
	if node != nil {
		printFunc(node.value)                   // 先打印根结点
		preOrderTraverse(node.left, printFunc)  // 再打印左子树
		preOrderTraverse(node.right, printFunc) // 最后打印右子树
	}
}

// 中序遍历：左子树 -> 根节点 -> 右子树
func (tree *ItemBinarySearchTree) PostOrderTraverse(printFunc func(Item)) {
	tree.lock.RLock()
	defer tree.lock.RUnlock()
	postOrderTraverse(tree.root, printFunc)
}
func postOrderTraverse(node *Node, printFunc func(Item)) {
	if node != nil {
		postOrderTraverse(node.left, printFunc)  // 先打印左子树
		postOrderTraverse(node.right, printFunc) // 再打印右子树
		printFunc(node.value)                    // 最后打印根结点
	}
}

// 后序遍历：左子树 -> 右子树 -> 根结点
func (tree *ItemBinarySearchTree) InOrderTraverse(printFunc func(Item)) {
	tree.lock.RLock()
	defer tree.lock.RUnlock()
	inOrderTraverse(tree.root, printFunc)
}
func inOrderTraverse(node *Node, printFunc func(Item)) {
	if node != nil {
		inOrderTraverse(node.left, printFunc)  // 先打印左子树
		printFunc(node.value)                  // 再打印根结点
		inOrderTraverse(node.right, printFunc) // 最后打印右子树
	}
}
```



#### String

```go
// 后序遍历打印树结构
func (tree *ItemBinarySearchTree) String() {
	tree.lock.Lock()
	defer tree.lock.Unlock()
	if tree.root == nil {
		println("Tree is empty")
		return
	}
	stringify(tree.root, 0)
	println("----------------------------")
}
func stringify(node *Node, level int) {
	if node == nil {
		return
	}

	format := ""
	for i := 0; i < level; i++ {
		format += "\t" // 根据节点的深度决定缩进长度
	}
	format += "----[ "
	level++
	// 先递归打印左子树
	stringify(node.left, level)
  	// 打印值
	fmt.Printf(format+"%d\n", node.key)
	/// 再递归打印右子树
	stringify(node.right, level)
}
```

测试用例：[tree_test.go](https://github.com/wuYinBest/blog/blob/master/codes/golang-data-structure-binary-search-tree/tree_test.go)



<br/>

### 总结

对于二叉搜索树的操作，增删查都与递归相关，所以在实现时一定要分析清楚递归的终止条件，在正确的条件下 `return`，避开死循环