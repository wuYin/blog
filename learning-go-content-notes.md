---
title: learning-go-content-notes
date: 2018-02-22 14:07:55
tags: Golang
---



### 简介

#### 变量、类型和关键字

超过两个语句放在一行时，分号 `;` 才是必需的，Go 官方认为，分号 `;` 是编译器使用而对用户不是必需的，在语法层面尽量避免使用。

简短声明一步完成了声明和赋值，如：

```go
var a int	// 声明变量 a 为 int 类型
a = 10		// 赋值 a 等于 10

a := 10		// 声明并赋值，变量类型由值类型自动推算，但只能在函数内部使用
```

即使在 32 位系统上， `int64` 与 `float64` 都是 64 位的实现



#### 运算符与内建结构

Go 不支持方法重载（不同参数的同名函数），但一些内建运算符支持重载比如 `+` ：可用于数字、字符串等不同类型。

Go 不支持三元表达式，且 `++` 和 `—` 仅是运算符。



#### 控制结构

`if`、`for` 及函数开头的 `{` 不能单独成行，使用 [gofmt](https://golang.org/cmd/gofmt/) 即可格式化：

```go
if(true) {	// 正确的代码格式
}	

if(true)	// syntax error: unexpected newline, expecting { after if clause
{
}
```

代码风格：

```go
file, err := os.Open(fileName, os.O_RDONLY, 0)
// 简洁易读
if err != nil {
	return err
}
doSomething(file)

// 应尽量避免 else
if err == nil {
    doSomething(file)
} else {
 	return err   
}
```

直接使用 range 迭代字符串，得到的值是 rune 类型的 Unicode 字符



#### 内建函数

```go
close	// 关闭 channel
delete 	// 删除 map 中的元素
len		// 获取字符串、slice和数组的实际长度
cap		// 获取 slice 的容量
new		// 类型变量的内存分配
make	// 为内建类型（map、slice 和 channel）分配实际的内存
copy	// 复制 slice
append	// 追加 slice
panic recover				// 异常处理
print println				// 输出
panic	complex real imag	// 复数相关函数
```



### array、slice 和 map

#### array

数组赋值给另一数组时，将复制所有元素。作为函数参数时是传值使用：

```go
arr := [3]int{1, 2, 3}		// 声明数组时必须在 [] 中指定数字 或 ...
arr := [...]int{1, 2, 3}	// 简写，自动统计元素个数

arr := [3][2]int{ [2]int{1,2}, [2]int{3,4}, [2]int{5,6} }
arr := [3][2]int{ [...]int{1,2}, [...]int{3,4}, [...]int{5,6} }		// 简写
arr := [3][2]int{ {1,2}, {3,4}, {5,6} }		// 多维数组内外数据类型一致，可省略内部元素类型
```

#### slice

slice 本质上是指向数组的指针，在作为函数参数时是传址使用。注意从数组创建 slice 的几种方式：

```go
arr := [...]{1, 2, 3, 4, 5}
s1 := arr[2:4]	// 左闭右开	// 包含元素 3, 4
s2 := arr[:3]	// : 左侧默认是0，右侧默认是 len(arr)	// 包含元素 1, 2, 3
s3 := arr[:]	// 用 arr 中所有元素创建 slice，是 arr[0:len(arr)] 的简写
s4 := s1[:]		// 此时 s4 与 s1 指向同一个数组
```

**注意**：

- `append(slice []Type, elems ...Type) []Type`

  若向 slice `append` 元素后超过了其容量，则 `append` 会分配一个 2 倍空间的新 slice 来存放原 slice 和要追加的值，此时原来的数组值不变，此时返回的 slice 与原 slice 指向不同底层数组。如：

  ```go
  arr1 := [5]int{1, 2, 3, 4, 5}
  s1 := arr1[1:2]
  s1 = append(s1, 6, 7, 8)	// arr1: 1 2 6 7 8	// s1 未超过容量

  arr2 := [5]int{1, 2, 3, 4, 5}
  s2 := arr2[1:3]
  s2 = append(s2, 6, 7, 8)	// arr2: 1 2 3 4 5	// s2 超过容量
  ```

- `copy(dst, src []Type) int`

  将 src 复制到 dst 中并返回复制元素的个数。注意元素复制的数量是 `min(len(dst), len(src))`，如：

  ```go
  var a = [...]int{0, 1, 2, 3, 4, 5, 6, 7}
  var s = make([]int, 6)

  n1 := copy(s, a[0:])	// 6 < 8 	// n1 == 6		// s: 0,1,2,3,4,5
  n2 := copy(s, s[2:])	// 6 > 4 	// n2 == 4		// s: 2,3,4,5, 4,5
  ```

  ​

### map

```go
name := map[string]string{
	"first": "Go",
	"last": "lang",		// , 是必需的
}
v, ok := name["nonexist"]	// ok == false	// 判断 map 中是否存在某个元素
```























