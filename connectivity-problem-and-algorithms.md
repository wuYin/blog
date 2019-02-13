---
title: 解决连通性问题的四种算法
date: 2018-01-27 20:04:13
tags: algorithms
---

> 最近做 [B 站弹幕分析](https://github.com/wuYinBest/bilibili-live-crawler) 的项目，学习 [Jieba](https://github.com/fxsjy/jieba) 中文分词的动态规划算法，发现自己的算法知识待系统的学习，遂读 Sedgewick 的《算法 C 实现第三版》，这一系列算法的代码放在 [Github](https://github.com/wuYinBest/algorithms-in-golang)



### 连通性问题

#### 问题概述

先来看一张图：


![](https://contents.yinzige.com/connectivity-example.png)

在这个彼此连接和断开的点网络中，我们可以找到一条 p 点到 q 点的路径。在计算机网络中判断两台主机是否连通、在社交网络中判断两个用户是否存在间接社交关系等，都可以抽象成连通性问题。



#### 问题抽象

可将网络中的点（主机、人）抽象为对象，`p-q` 表示 **p连接到q**，连通关系可传递： `p-q & q-r => p-r`；为简述问题，将两个对象标记为一个整数对，则给定整数对序列就能描述出点网络。

如下图结点数 N = 5 的网络（使用 0 ~ N-1表示对象），可用整数对序列 `0-1  1-3  2-4` 来描述连通关系， 其中 0 和 3 也是连通的，存在两个连通分量：{0, 1, 3} 和 {2, 4}

 ![](https://contents.yinzige.com/5-nodes-example.png)

问题：给定描述连通关系的整数对序列，任给其中两个整数 p 和 q，判断其是否能连通？



#### 问题示例

```
输入 	不连通	连通 
3-4 	3-4
4-9 	4-9
8-0 	8-0
2-3 	2-3
5-6 	5-6
2-9 		2-3-4-9	
5-9 	5-9
7-3 	7-3
4-8 	4-8
5-6 		5-6
0-2 		0-8-4-3-2
6-1 	6-1
```

 对应的连通图如下，黑线表示首次连接两个结点，绿线表示两结点已存在连通关系： ![](https://contents.yinzige.com/integers-sequence.png)



#### 算法一：快速查找算法

使用数组 `id[i]` 存储结点的值， `i` 为结点序号，即初始状态序号和数组值相同 ：  ![](https://contents.yinzige.com/arrray.png)

当输入前两个连通关系后， `id[i]` 变化如下：  ![](https://contents.yinzige.com/connect.png)

可以看出， `id[i]` 的值是完成连通后，`i` 连接到的终点结点。若 p 和 q 连通，则 `id[p]` 和 `id[q]` 值应相等。

如完成 `4-9` 后， `id[3]` 和 `id[4]` 的值均为终点结点 9。此时判断 3 和 9 是否连通，直接判断 `id[3]` 和 `id[9]` 的值是否相等，相等则连通，不等则不存在连通关系。显然  `id[3] == id[9] == 9`，即存在连通关系。



##### 算法实现

```go
/** file: 1.1-quick_find.go */
package main

import ...

const N = 10
var id [N]int

func main() {
	reader := bufio.NewReader(os.Stdin)

	// 初始化 id 数组，元素值与结点序号相等
	for i := 0; i < N; i++ {
		id[i] = i
	}

	// 读取命令行输入
	for {
		data, _, _ := reader.ReadLine()
		str := string(data)
		if str == "\n" {
			continue
		}
		if str == "#" {
			break
		}

		values := strings.Split(str, " ")
		p, _ := strconv.Atoi(values[0])
		q, _ := strconv.Atoi(values[1])

		if Connected(p, q) {
			fmt.Printf("Already Connected nodes: %d-%d\n", p, q)
			continue
		}
		Union(p, q)
	}
}

// 判断整数 p 和 q 的结点是否连通
func Connected(p, q int) bool {
	return id[p] == id[q]
}

// 连通 p-q 结点
func Union(p, q int) {
	pid := id[p]
	qid := id[q]
	// 遍历 id 数组，将所有值为 id[p] 的结点全部替换为 id[q]
	for i := 0; i < N; i++ {
		if id[i] == pid {
			id[i] = qid
		}
	}
	fmt.Printf("Unconnected nodes: %d-%d\n", p, q)
}
```



##### 运行效果：能判断 2-9 已存在连通关系

 ![](https://contents.yinzige.com/connect-run.png)

##### 复杂度

快速查找算法在判断 p 和 q 是否连通时，只需判断 `id[p]` 和 `id[q]` 是否相等。但 p 和 q 不连通时会进行合并，每次合并都需要遍历整个数组。特性：查找快、合并慢



#### 算法二：快速合并算法

##### 概述

快速查找算法每次合并都会全遍历数组导致低效。我们想能不能不要每次都遍历 `id[]` ，优化为每次只遍历数组的部分值，复杂度都会降低。

这时应想到树结构，在连通关系的传递性中，`p->r & q->r => p->q`，可将 r 视为根，p 和 q 视为子结点，因为 p 和 q 有相同的根 r，所以 p 和 q 是连通的。这里的树是连通关系的抽象。

##### 数据结构

使用数组作为树的实现：

-   结点数组 `id[N]`，`id[i]` 存放 `i` 的父结点
-   `i` 的根结点是 `id[id[...id[i]...]]`，不断向上找父结点的父结点...直到根结点（父结点是自身）

##### 使用树的优势

将整数对序列的表示从数组改为树，每个结点存储它的父结点位置，这种树有 2 点好处：

1.  判断 p 和 q 是否连通：是否有相同的根结点
2.  合并 p 到 q：将 p 的根结点改为 q 的根结点（无需全遍历，快速合并）

##### 例子：

对于上边的整数对序列，查找、合并过程如下，橙色是合并动作、灰色是已连通状态、绿色是存储树的数组。

注意红色的 `2-3`，不是直接把 2 作为 3 的子结点，而是找到 3 的根结点 9，合并 `2-3` 与 `3-4-9` ，生成 `2-9`

 ![](https://contents.yinzige.com/tree.png)



算法实现：

```go
/** file: 1.2-quick_union.go */

// p 和 q 有相同的根结点，则是连通的
func Connected(p, q int) bool {
	return getRoot(p) == getRoot(q)
}

// 连通 p-q 结点
func Union(p, q int) {
	pRoot := getRoot(p)
	qRoot := getRoot(q)
	id[pRoot] = qRoot		// q 树的根此时有了父结点（p 树的根），完成合并
	fmt.Printf("Unconnected nodes: %d-%d\n", p, q)
}


// 获取结点 i 的根结点
func getRoot(i int) int {
	// 没到根结点就继续向上寻找
	for i != id[i] {
		i = id[i]
	}
	return i
}
```



#### 算法三：带权快速合并算法

##### 概述

快速合并算法有一个缺陷：数据量很大时，任意合并子树，会导致树越来越高，在查找根结点时要遍历数组大部分的值，依旧会很慢。下图中判断 p、q 是否连通，就需要查找 13 个结点：

 ![](https://contents.yinzige.com/big-tree.png)

如果树合并后的依旧比较矮，各子树之间平衡，则查找根结点会少遍历很多结点，下图中再判断 p、q 是否连通，只需查找 7 个结点：

 ![](https://contents.yinzige.com/balance-tree.png)

##### 平衡树的构建

构建平衡的树需要在合并时，将小树合并到大树上，保证合并后的树增高缓慢或者就不增高，从而使大部分的合并需要遍历的结点大大减少。区分小树、大树使用的是树的权值：子树含有结点的个数。

##### 数据结构

树结点的存储依旧使用 `id[i]` ，但需要一个额外的数组 `size[i]`，记录结点 i 的子结点数。

##### 算法实现

```go
/**
file: 1.3-weighted_version.go
在快速合并算法的基础上，只需要在合并操作中，将小树合并到大树上即可
*/

var id [N]int
var size [N]int

func main() {
 	// 初始化 id 数组，元素值与结点序号相等
	for i := 0; i < N; i++ {
		id[i] = i
		size[i] = i
	} 
  	...
}  

...

// 连通 p-q 结点
func Union(p, q int) {
	pRoot := getRoot(p)
	qRoot := getRoot(q)

	// p 树是大树
	if size[pRoot] < size[qRoot] {
		id[pRoot] = qRoot
		size[qRoot] += size[pRoot]
	} else {
		id[qRoot] = id[pRoot]
		size[pRoot] += size[qRoot]
	}

	id[pRoot] = qRoot // q 树的根此时有了父结点（p 树的根），完成合并
	fmt.Printf("Unconnected nodes: %d-%d\n", p, q)
}
```



#### 算法四：路径压缩的加权快速合并算法

##### 概述

加权快速合并算法在大部分整数对都是直接连接的情况下，生成的树依旧会比较高，比如序列：

```
10-8 8-6 11-9 12-9 9-6 6-3 7-3 3-1 4-1 5-1 1-0 2-0
```

生成的树如下：

 ![](https://contents.yinzige.com/special-tree.png)

此时判断 `9-2` 的连通关系，需要分别找到 9 和 2 的根结点。在寻找 9 的根结点时经过 6、3、1树，因为6、3、1树的子节点和 9 一样，根结点都是 0，所以直接把6、3、1树变成 0 的子树。如下： ![](https://contents.yinzige.com/flat-tree.png)

##### 优化

每次计算某个节点的根结点时，将沿路检查的结点也指向根结点。尽可能的展平树，在检查连通状态时将大大减少遍历的结点数目。

##### 算法实现

```go
/**
file: 1.4-path_compression_by_halving.go
改动的代码很少，但很精妙
*/

// 获取结点 i 的根结点
func getRoot(i int) int {
	// 没到根结点就继续向上寻找
	for i != id[i] {
		id[i] = id[id[i]]		// 将结点、结点的父结点不断往上挪动，直到都连接上了根结点
		i = id[i]
	}
	return i
}
```



##### 复杂度

N 是结点集合的大小，T 是树的高度。

|     算法      | 初始化的复杂度 |     合并复杂度     |   查找复杂度   |
| :---------: | :-----: | :-----------: | :-------: |
|    快速查找     |    N    |    N（全遍历）     | 1（数组取值对比） |
|    快速合并     |    N    |    T（遍历树）     |  T（遍历树）   |
|   带权快速合并    |    N    |     lg N      |   lg N    |
| 路径压缩的带权快速合并 |    N    | 接近1（树的高度几乎为2） |    接近1    |



#### 总结

上边介绍了 4 种解决连通性问题的算法，从低效完成基本功能的快速查找，到不断优化降低复杂度接近1 的路径压缩带权快速合并。可以学到算法解决程序问题的大致步骤：先完成基本功能，再针对低效操作来优化降低复杂度。

