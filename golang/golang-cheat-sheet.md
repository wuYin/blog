---
title: Golang 速查表
date: 2018-06-11 10:32:06
tags: Golang
---

简要介绍 Go 语法及特性。原文：[go-lang-cheat-sheet](https://github.com/a8m/go-lang-cheat-sheet)

<!-- more -->

## 目录

1. 基础语法
2. 运算符
   - 算术运算符
   - 比较运算符
   - 逻辑运算符
   - 其他
3. 声明
4. 函数
   - 函数作为值和回调使用
   - 可变参数函数
5. 内置类型
6. 类型转换
7. package
8. 流程控制结构
   - 条件判断（if）
   - 循环（for）
   - 多条件分支（switch）
9. array, slice, range
   - array
   - slice
   - array 和 slice 的操作函数
10. map
11. 结构体
12. 指针
13. 接口
14. 结构体和接口的组合嵌入
15. Errors
16. 并发
    - goroutine
    - channel
    - channel 开发原则
17. 输出
18. 代码片段
    - Http-Server

## 前言

### 参考

文中大部分代码都摘抄自 [A Tour of Go](http://tour.golang.org/)，对新手来说是很好的参考资料。

### Go 特性

- [命令式编程](https://zh.wikipedia.org/wiki/%E6%8C%87%E4%BB%A4%E5%BC%8F%E7%B7%A8%E7%A8%8B)
- 静态类型
- 类 C 语法（括号使用频率更少 & 无需分号），类 [Oberon-2](https://www.inf.ethz.ch/personal/wirth/Oberon/Oberon07.Report.pdf) 的语法结构
- 代码能编译为本地可执行文件（无需 JVM 类的虚拟机）
- `struct` 和 `method` 取代类的概念
- 接口
- [类型组合](http://golang.org/doc/effective_go.html#embedding) 取代显式继承
- 有[头等函数](https://zh.wikipedia.org/wiki/%E5%A4%B4%E7%AD%89%E5%87%BD%E6%95%B0)
- 有回调函数
- 函数可有多个返回值
- 保留指针，但不能直接参与算术运算
- 内置并发原语：`goroutine` 和 `channel`



## 基础语法

### Hello World

文件 `hello.go`：

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello Go")
}
```

运行：`$ go run hello.go`



## 运算符

### 算术运算符

| 运算符 |        描述         |
| :----: | :-----------------: |
|  `+`   |         加          |
|  `-`   |         减          |
|  `*`   |         乘          |
|  `/`   |         除          |
|  `%`   |        取余         |
|  `&`   |       按位与        |
| &#166; |       按位或        |
|  `^`   |      按位异或       |
|  `&^`  | 按位清除（AND NOT） |
|  `<<`  |        左移         |
|  `>>`  |        右移         |

`&^` 即是 `AND NOT(x, y) = AND(x, NOT(Y))`，如：

```go
package main

import "fmt"

func main() {
	x := 0xDC	// 11011100
	y := 0xF0	// 11110000
	z := x &^ y	// 00001100	// y 中为 1 的位全部被清除为 0
	fmt.Printf("%08b", z)
}
```



### 比较运算符

| 运算符 |   描述   |
| :----: | :------: |
|  `==`  |   相等   |
|  `!=`  |   不等   |
|  `<`   |   小于   |
|  `<=`  | 小于等于 |
|  `>`   |   大于   |
|  `>=`  | 大于等于 |



### 逻辑运算符

|    运算符    |  描述  |
| :----------: | :----: |
|     `&&`     | 逻辑与 |
| &#166;&#166; | 逻辑或 |
|     `!`      |  取反  |



### 其他

| 运算符 |             描述             |
| :----: | :--------------------------: |
|  `&`   |       寻址（生成指针）       |
|  `*`   |      获取指针指向的数据      |
|  `<-`  | 向 channel 中发送 / 接收数据 |



## 声明

与 C 不同，[类型放在标识符后面](https://blog.golang.org/gos-declaration-syntax)：

```go
var foo int 			// 无初值的声明
var foo int = 42 		// 带初值的声明
var foo, bar int = 42, 1302	// 一次性声明并初始化多个变量
var foo = 42 			// 类型推断，由使用的上下文决定
foo := 42 			// 简短声明，只能用在函数内部
const constant = "This is a constant"
```



## 函数

```go
// 最简单的函数
func functionName() {}

// 带参数的函数(注意类型也是放在标识符之后的)
func functionName(param1 string, param2 int) {}

// 类型相同的多个参数
func functionName(param1, param2 int) {}

// 声明返回值的类型
func functionName() int {
    return 42
}

// 一次返回多个值
func returnMulti() (int, string) {
    return 42, "foobar"
}
var x, str = returnMulti()

// 只使用 return 返回多个命名返回值
func returnMulti2() (n int, s string) {
    n = 42
    s = "foobar"
    // n 和 s 会被返回
    return
}
var x, str = returnMulti2()
```



### 函数作为值和回调使用

```go
func main() {
    // 将函数作为值，赋给变量
    add := func(a, b int) int {
        return a + b
    }
    // 使用变量直接调用函数
    fmt.Println(add(3, 4))
}

// 回调函数作用域：在定义回调函数时能访问外部函数的值
func scope() func() int{
    outer_var := 2
    foo := func() int { return outer_var}
    return foo
}

func another_scope() func() int{
    // 编译错误，两个变量不在此函数作用域内
    // undefined: outer_var
    outer_var = 444
    return foo
}

// 回调函数不会修改外部作用域的数据
func outer() (func() int, int) {
    outer_var := 2
    inner := func() int {
        outer_var += 99 	// 试着使用外部作用域的 outer_var 变量
        return outer_var 	// 返回值是 101，但只在 inner() 内部有效
    }
    return inner, outer_var	// 返回值是 inner, 2 (outer_var 仍是 2）
}

inner, outer_var := outer();	// inner, 2
inner();	// 返回 101
inner();	// 返回 200	// 回调函数的特性
```



### 可变参数函数

```go
func main() {
	fmt.Println(adder(1, 2, 3)) 	// 6
	fmt.Println(adder(9, 9))	// 18
	
	nums := []int{10, 20, 30}
	fmt.Println(adder(nums...))	// 60
}

// 在函数的最后一个参数类型前，使用 ... 可表明函数还能接收 0 到多个此种类型的参数
// 下边的函数在调用时传多少个参数都可以
func adder(args ...int) int {
	total := 0
	for _, v := range args {	// 使用迭代器逐个访问参数
		total += v
	}
	return total
}
```





## 内置类型

```go
bool

string

int  int8  int16  int32  int64
uint uint8 uint16 uint32 uint64 uintptr

byte // uint8 类型的别名	// 存储 raw data

rune // int32 类型的别名	// 一个 Unicode code point 字符

float32 float64

complex64 complex128
```



## 类型转换

```go
var i int = 42
var f float64 = float64(i)
var u uint = uint(f)

// 简化语法
i := 42
f := float64(i)
u := uint(f)
```



## package

1. package 在源文件开头声明
2. main package 才是可执行文件
3. 约定：package 名字与 import 路径的最后一个单词一致（如导入 math/rand 则 package 叫 rand）
4. 大写开头的标识符（变量名、函数名…）：对其他 package 是可访问的
5. 小写开头的标识符：对其他 package 是不可见的





## 流程控制结构

### if

```go
func main() {
	// 一般的条件判断
	if x > 0 {
		return x
	} else {
		return -x
	}
    	
	// 在条件判断语句前可塞一条语句，使代码更简洁
	if a := b + c; a < 42 {
		return a
	} else {
		return a - 42
	}
    
	// 使用 if 做类型断言
	var val interface{}
	val = "foo"
	if str, ok := val.(string); ok {
		fmt.Println(str)
	}
}
```



### Loops

```go
// Go 语言中循环结构只有 for，没有 do、while、until、foreach 等等
for i := 1; i < 10; i++ {
}
for ; i < 10;  { 	// 等效于 while 循环
}
for i < 10  { 		// 只有一个判断条件时可省去分号
}
for { 			// 无条件循环时，等效于 while(true)
}
```



### switch

```go
// switch 分支语句
switch operatingSystem {
    case "darwin":
        fmt.Println("Mac OS Hipster")
        // case 语句自带 break，想执行所有 case 需要手动 fallthrough
    case "linux":
        fmt.Println("Linux Geek")
    default:
        // Windows, BSD, ...
        fmt.Println("Other")
}

// 和 if、for 语句一样，可在判断变量之前加入一条赋值语句
switch os := runtime.GOOS; os {
    case "darwin": ...
}

// 在 switch 中还能做比较，相当于 switch (true) {...}
number := 42
switch {
    case number < 42:
        fmt.Println("Smaller")
    case number == 42:
        fmt.Println("Equal")
    case number > 42:
        fmt.Println("Greater")
}

// 多个 case 可使用逗号分隔统一处理
var char byte = '?'
switch char {
    case ' ', '?', '&', '=', '#', '+', '%':
    fmt.Println("Should escape")
} 
```



## Arrays, Slices, Ranges

### Arrays

```go
var a [10]int // 声明长度为 10 的 int 型数组，注意数组类型 = （元素类型 int，元素个数 10）
a[3] = 42     // 设置元素值
i := a[3]     // 读取元素值

// 声明并初始化数组
var a = [2]int{1, 2}
a := [2]int{1, 2} 	// 简短声明
a := [...]int{1, 2}	// 数组长度使用 ... 代替，编译器会自动计算元素个数
```

### slices

```go
var a []int           		// 声明 slice，相当于声明未指定长度的数组
var a = []int {1, 2, 3, 4}	// 声明并初始化 slice (基于 {} 中给出的底层数组)
a := []int{1, 2, 3, 4}		// 简短声明
chars := []string{0:"a", 2:"c", 1: "b"}  // ["a", "b", "c"]

var b = a[lo:hi]	// 创建从 lo 到 hi-1 的 slice 
var b = a[1:4]		// 创建从 1  到 3    的 slice
var b = a[:3]		// 缺省 start index 则默认为 0 
var b = a[3:]		// 缺省 end   index 则默认为 len(a)
a =  append(a,17,3)	// 向 slice a 中追加 17 和 3
c := append(a,b...)	// 合并两个 slice

// 使用 make 创建 slice
a = make([]byte, 5, 5)	// 第一个参数是长度，第二个参数是容量
a = make([]byte, 5)	// 容量参数是可选的

// 从数组创建 slice
x := [3]string{"Лайка", "Белка", "Стрелка"}
s := x[:] 		// slice s 指向底层数组 x
```



### 数组和 slice 的操作函数

```go
// 迭代数组或 slice
for i, e := range a {
    // i 是索引
    // e 是元素值
}

// 如果你只要值，可用 _ 来丢弃返回的索引
for _, e := range a {
}

// 如果你只要索引
for i := range a {
}

// 在 Go 1.4 以前的版本，如果 i 和 e 你都不用，直接 range 编译器会报错
for range time.Tick(time.Second) {
    // 每隔 1s 执行一次
}
```



## map

```go
var m map[string]int
m = make(map[string]int)
m["key"] = 42
fmt.Println(m["key"])

delete(m, "key")

elem, ok := m["key"] // 检查 m 中是否键为 key 的元素，如果有 ok 才为 true

// 使用键值对的形式来初始化 map
var m = map[string]Vertex{
    "Bell Labs": {40.68433, -74.39967},
    "Google":    {37.42202, -122.08408},
}
```





## 结构体

Go 语言中没有 class 类的概念，取而代之的是 struct，struct 的方法对应到类的成员函数。

```go
// struct 是一种类型，也是字段成员的集合体

// 声明 struct
type Vertex struct {
    X, Y int
}

// 初始化 struct
var v = Vertex{1, 2}			// 字段名有序对应值
var v = Vertex{X: 1, Y: 2} 		// 字段名对应值
var v = []Vertex{{1,2},{5,2},{5,5}}	// 初始化多个 struct 组成的 slice

// 访问成员
v.X = 4

// 在 func 关键字和函数名之间，声明接收者是 struct
// 在方法内部，struct 实例被复制，传值引用
func (v Vertex) Abs() float64 {
    return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

// 调用方法(有接收者的函数)
v.Abs()

// 有的方法接收者是指向 struct 的指针
// 此时在方法内调用实例，将是传址引用
func (v *Vertex) add(n float64) {
    v.X += n
    v.Y += n
}
```



### 匿名结构体

使用 `map[string]interface{}` 开销更小且更为安全。

```go
point := struct {
	X, Y int
}{1, 2}
```





## 指针

```go
p := Vertex{1, 2}  // p 是一个 Vertex
q := &p            // q 是指向 Vertex 的指针
r := &Vertex{1, 2} // r 也是指向 Vertex 的指针

var s *Vertex = new(Vertex) // new 返回的指向该实例指针
```





## 接口

```go
// 声明接口
type Awesomizer interface {
    Awesomize() string
}

// 无需手动声明 implement 接口
type Foo struct {}

// 自定义类型如果实现了接口的所有方法，那它就自动实现了该接口
func (foo Foo) Awesomize() string {
    return "Awesome!"
}
```



## 结构体和接口的组合嵌入

```go
// 实现 ReadWriter 的类型要同时实现了 Reader 和 Writer 两个接口
type ReadWriter interface {
    Reader
    Writer
}

// Server 暴露出 Logger 所有开放的方法
type Server struct {
    Host string
    Port int
    *log.Logger
}

// 初始化自定义的组合类型
server := &Server{"localhost", 80, log.New(...)}

// 组合的结构体能直接跨节点调用方法
server.Log(...) // 等同于调用 server.Logger.Log(...)

// 字段同理
var logger *log.Logger = server.Logger
```





## Errors

Go 中没有异常处理机制，函数在调用时在有可能会产生错误，可返回一个 `Error` 类型的值，`Error` 接口：

```go
type error interface {
    Error() string
}
```

一个可能产生错误的函数：

```go
func doStuff() (int, error) {
}

func main() {
    result, err := doStuff()
    if err != nil {
        // 错误处理
    }
    // 使用 result 处理正常逻辑
}
```



## 并发

### goroutine

goroutine（协程）是轻量级的线程（Go runtime 自行管理，而不是操作系统），代码 `go f(a, b)` 就开了一个运行 `f`  函数的协程。

```go
func doStuff(s string) {
}

func main() {
    // 在协程中执行函数
    go doStuff("foobar")

    // 在协程中执行匿名函数
    go func (x int) {
        // 函数实现
    }(42)
}
```



## Channels

```go
ch := make(chan int) 	// 创建类型为 int 的 channel
ch <- 42             	// 向 channel ch 写数据 42
v := <-ch            	// 从 channel ch 读数据，此时 v 的值为 42
			// 无缓冲的 channel 此时会阻塞
			// 如果 channel 中无数据，则读操作会被阻塞，直到有数据可读

// 创建带缓冲的 channel
// 向带缓冲的 channel 写数据不会被阻塞，除非该缓冲区已满
ch := make(chan int, 100)

close(ch) // 发送者主动关闭 channel

// 在从 channel 读数据的同时检测其是否已关闭
// 如果 ok 为 false，则 ch 已被关闭
v, ok := <-ch	

// 从 channel 中读数据直到它被关闭
for i := range ch {
    fmt.Println(i)
}

// select 语句中 任一 channel 不阻塞则自动执行对应的 case
func doStuff(channelOut, channelIn chan int) {
    select {
        case channelOut <- 42:
            fmt.Println("We could write to channelOut!")
        case x := <- channelIn:
            fmt.Println("We could read from channelIn")
        case <-time.After(time.Second * 1):
            fmt.Println("timeout")
    }
}
```



### channel 开发原则

1. 向 nil channel 写数据将卡死，一直阻塞 <img src="http://p7f8yck57.bkt.clouddn.com/2018-06-11-104452.png" width=60%/>

2. 从 nil channel 读数据将卡死，一直阻塞<img src="http://p7f8yck57.bkt.clouddn.com/2018-06-11-104239.png" width=60%>

3. 向已关闭的 channel 写数据将造成 panic

   ```go
   package main
   
   func main() {
   	var c = make(chan string, 1)
   	c <- "Hello, World!"
   	close(c)
   	c <- "Hello, Panic!"
   }
   ```

   运行：

   <img src="http://p7f8yck57.bkt.clouddn.com/2018-06-11-104657.png" width=60%> 

4. 从已关闭的 channel 读数据将返回零值

   ```go
   package main
   
   func main() {
   	var c = make(chan int, 2)
   	c <- 1
   	c <- 2
   	close(c)
   	for i := 0; i < 3; i++ {
   		println(<-c)
   	}
   }
   ```

   运行：

   <img src="http://p7f8yck57.bkt.clouddn.com/2018-06-11-104956.png" width=60%> 

   





## 输出

```go
fmt.Println("Hello, 你好, नमस्ते, Привет, ᎣᏏᏲ") 		// 最基本的输出，会自动加一个换行
p := struct { X, Y int }{ 17, 2 }
fmt.Println( "My point:", p, "x coord=", p.X ) 		// 输出结构体字段等
s := fmt.Sprintln( "My point:", p, "x coord=", p.X )	// 组合字符串并返回

fmt.Printf("%d hex:%x bin:%b fp:%f sci:%e",17,17,17,17.0,17.0) // 类 C 的格式化输出
s2 := fmt.Sprintf( "%d %f", 17, 17.0 )			// 格式化字符串并返回

hellomsg := `
 "Hello" in Chinese is 你好 ('Ni Hao')
 "Hello" in Hindi is नमस्ते ('Namaste')
` 
// 声明多行字符串，在前后均使用反引号 `
```





## 代码片段

###  HTTP Server 

```go
package main

import (
    "fmt"
    "net/http"
)

// 定义响应的数据结构
type Hello struct{}

// Hello 实现 http.Handler 中定义的 ServeHTTP 方法
func (h Hello) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello!")
}

func main() {
    var h Hello
    http.ListenAndServe("localhost:4000", h)
}

// http.ServeHTTP 在接口内的定义如下：
// type Handler interface {
//     ServeHTTP(w http.ResponseWriter, r *http.Request)
// }
```

运行：

 <img src="http://p7f8yck57.bkt.clouddn.com/2018-06-11-110330.png" width=60%> 

## 总结

上边十七个知识点简要概括了常见语法，可复习使用，但涉及到的细节不多，细读[《Go 程序设计语言》](https://book.douban.com/subject/27044219/) 才是。











