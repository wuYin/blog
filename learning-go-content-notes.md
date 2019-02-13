---
title: Learning Go 读书笔记
date: 2018-02-22 14:07:55
tags: Golang
---



[《学习 Go 语言》](https://mikespook.com/learning-go/) 读书笔记。

<!-- more -->



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
if(true) {
    // 正确的代码格式
}	

if(true)
{
    // syntax error: unexpected newline, expecting { after if clause
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
close		// 关闭 channel
delete 		// 删除 map 中的元素
len		// 获取字符串、slice和数组的实际长度
cap		// 获取 slice 的容量
new		// 类型变量的内存分配
make		// 为内建类型（map、slice 和 channel）分配实际的内存
copy		// 复制 slice
append		// 追加 slice
panic recover			// 异常处理
print println			// 输出
panic	complex real imag	// 复数相关函数
```



### array、slice 和 map

#### array

数组赋值给另一数组时，将复制所有元素。作为函数参数时是传值使用：

```go
arr := [3]int{1, 2, 3}		// 声明数组时必须在 [] 中指定数字 或 ...
arr := [...]int{1, 2, 3}	// 简写，自动统计元素个数

arr := [3][2]int{ [2]int{1,2}, [2]int{3,4}, [2]int{5,6} }
arr := [3][2]int{ [...]int{1,2}, [...]int{3,4}, [...]int{5,6} }	// 简写
arr := [3][2]int{ {1,2}, {3,4}, {5,6} }		// 多维数组内外数据类型一致，可省略内部元素类型
```

#### slice

slice 本质上是指向数组的指针，在作为函数参数时是传址使用。注意从数组创建 slice 的几种方式：

```go
arr := [...]{1, 2, 3, 4, 5}

s1 := arr[2:4]	// 左闭右开	// 包含元素 3, 4
s2 := arr[:3]	// : 左侧默认是0，右侧默认是 len(arr)	// 包含元素 1, 2, 3
s3 := arr[:]	// 用 arr 中所有元素创建 slice，是 arr[0:len(arr)] 的简写
s4 := s1[:]	// 此时 s4 与 s1 指向同一个数组
```

**注意**：

`append(slice []Type, elems ...Type) []Type`

若向 slice `append` 元素后超过了其容量，则 `append` 会分配一个 2 倍空间的新 slice 来存放原 slice 和要追加的值，此时原来的数组值不变，此时返回的 slice 与原 slice 指向不同底层数组。如：

```go
arr1 := [5]int{1, 2, 3, 4, 5}
s1 := arr1[1:2]
s1 = append(s1, 6, 7, 8)	// arr1: 1 2 6 7 8	// s1 未超过容量

arr2 := [5]int{1, 2, 3, 4, 5}
s2 := arr2[1:3]
s2 = append(s2, 6, 7, 8)	// arr2: 1 2 3 4 5	// s2 超过容量
```


`copy(dst, src []Type) int`

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





### 函数

#### 函数定义

```go
func funcName(p int) (r, s int) {	// r, s 初始化为 0
  	r = p * 2	// p 的值被复制，通过传值来传递参数
    s = r - p	// r, s 是函数返回的命名参数，可在函数体内直接使用
    return 
}
```

函数定义顺序随意。Go 不允许函数嵌套，不过可以使用匿名函数实现。

#### 作用域

函数外定义的是全局变量，函数内定义的是局部变量。

命名覆盖：在函数内部有局部变量与某个全局变量同名，则函数执行时局部变量会覆盖全局变量。

#### 延迟调用

常使用 `defer` 关闭资源句柄，保证函数调用结束前指定的函数会被执行

函数内 `defer` 的延迟函数是按栈（先进后出）的顺序执行的：

```go
for i := 1; i < 4; i++ {
	defer println(i)	// 打印 3 2 1
}	
```

#### 变参

```go
func demo(arg ...int){
	fmt.Printf("%T\n", arg)		// []int	// slice 类型
    demoPart1(arg[1:])
}
```

#### 函数作为值使用

```go
tmp := func() {
	// 函数只是一个值，可赋值给变量
}
fmt.Printf("%T\n", tmp)	// func() 类型
tmp()			// 调用函数

func callback(x int, f func(int)) {
    f(x)		// 回调函数执行   
}    
```

#### Panic 与 Recover

Go 使用 panic-and-recover 和 defer 代替了异常处理机制，不过在代码中应该尽量少使用 panic 和 recover :

```go
func main() {
	tmp := func() {
        panic(100)			// 手动调用 panic 中断执行，但 main() 内的 defer 依旧会执行
		println("not exec")	// 已被中断，不执行
	}
	println(throwsPanic(tmp))	// true
}

func throwsPanic(f func()) (b bool) {
	defer func() {
        // recover() 仅在 defer 中生效。若执行过程正常则返回 nil，否则返回 panic() 的参数值
        if x := recover(); x != nil {
			b = true
			log.Print(x)	// 值为 panic() 的参数 100
		}
	}()
	f()
	return
}
```



### 包

与 PHP 的 namespace 类似，package 是 func、struct 和 变量等数据的集合，用于避免命名冲突和组织代码。

包中大写字母开头的数据是公有可导出的：在其他包中可直接引用，小写字母开头的数据是私有不能被外部引用的

#### 标识符与文档

命名应有意义，习惯上包名为全小写且与文件夹同名的一个单词，函数使用驼峰式命名。

包的注释写在任一文件的头部，用于介绍包提供的功能和完整情况。导出的函数应有文字描述函数的功能，如：

```go
/*
The regexp package implements a simple library forregular expressions.
...	
*/
package regexp

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func Printf(format string, a ...interface) (n int, err error)
```

#### 测试包

测试文件命名为 `*_test.go` 且测试函数以 `Test` 开头，注意测试包一般与被测试的包有相同的包名，以便测试私有函数。注意几个用来表明测试失败的函数：

```go
func (t *testing.T) Fail()	// 标记测试失败，但继续执行其他测试
func (t *testing.T) FailNow()	// 标记测试失败，立刻结束当前文件的测试，测试下一文件
func (t *testing.T) Log(args ...interface{})	// 格式化参数并记录
func (t *testing.T) Fatal(args ...interface{})	// Log() & FailNow()
```

#### 常用包（库）

- fmt ：实现格式化的 IO 函数，多用于格式化输出

- io：封装 os 包原始的 IO 操作

- os：提供与平台无关的操作系统功能接口，是根据 Unix 形式设计的

- os/exec：执行外部命令

- sync：提供同步原语如 mutex

- bufio：实现缓冲 IO

- sort：实现对数组、用户自定义集合的排序功能

- strconv：提供字符串与基本数据类型之间的转换

- flag：命令行参数解析

- encoding/json：解析和编码 JSON 对象和字串

- html/template：数据填充的视图模板，用于如 HTML 的文本输出

- net/http：实现发起和响应 HTTP 请求，解析 URL及可扩展的 HTTP 服务

- unsafe：包含 Go 在数据类型上不安全的操作，一般不使用

- reflect：实现运行时反射，常使用 TypeOf 来解析值的动态类型信息

  ​



### 进阶

#### 指针

Go 中的指针不像 C 一样支持指针运算（加减整数），如：

```go
var p *int
var x = 1
p = &x
*p++	// Go: 获取 p 指向的值并加 1	// C: p + sizeof(int)
fmt.Printf("%v\n", x)	// x == 2
```

#### 内存分配

- `func new(Type) *Type`：返回 `*Type` 并指向一个 `Type` 零值
- `func make(t Type, size ...IntegerType) Type`：返回初始化后的 `Type` 值， 只能用于创建 slice、map 和 channel

#### 自定义类型

若 struct 要实现某个接口，则实现的 func 必须是指定的方法。否则一般不区分函数和方法，实现功能即可。另外注意：

```go
type Person struct {
	Name string
	Age  int
}

// 方法
// 若 Person 要实现某个带 Intro 方法的接口，则必须使用方法实现 Intro()
func (p *Person) Intro() {
	fmt.Printf("Name: %s, Age: %d\n", p.Name, p.Age)
}

// 函数
func Intro2(p *Person) {
	fmt.Printf("Name: %s, Age: %d\n", p.Name, p.Age)
}

func main() {
    me := new(Person)
    me.Name = "wuYin"
    me.Age  =  20
    fmt.Printf("%v\n", me)	// &{wuYin 20}	// *Person 类型    
   	me.Intro()				// me.Intro() 是 (*me).Intro1() 的简写
	Intro2(me)
}
```

#### 类型转换

转换函数：

![](https://contents.yinzige.com/convert.png)

注意：

- string 与 byte slice、 rune slice

  ```go
  name := "Aa吴"

  // 每个 byte 保存字符串对应字节的整数值，utf8 编码一个字符可能有 2~4 个字节
  fmt.Printf("%v", []byte(name))	// [65 97 229 144 180]

  // 每个 rune 保存指向该 Unicode 字符的指针，一个字符一个整数编号
  fmt.Printf("%v", []rune(name))	// [65 97 21556]

  // 直接转换
  fmt.Printf("%v\n", string([]rune{'m', 'e'}))
  fmt.Printf("%v\n", string([]byte{'m', 'e'}))
  fmt.Printf("%v\n", string([]rune{257, 1024, 65}))
  ```

- float32、float64 转 int 均会截断小数部分

  ```go
  f := 100.1
  fmt.Printf("%v\n", int(f))	// 100
  ```

- 自定义类型的转换

  ```go
  type Score struct{ int }
  type Grade Score

  func main() {
  	var s = Score{100}
      var g Grade = s		// cannot use s (type Score) as type Grade in assignment
      
      var g = Grade(s)	// 显式类型转换，两个结构体字段信息需一致
  }    
  ```

  ​




### 接口

#### 定义及优势

interface 类型仅是方法的集合：

```go
type I interface {
	Get() int
	Set(int)
}
```

结构体 S 实现了接口 I：

```go
type S struct {
	i int
}

func (s *S) Get() int {
	return s.i
}

func (s *S) Set(v int) {
	s.i = v
}
```

接口的优势，使用接口值（duck typing 模式）：

```go
// 所有实现了接口 I 的结构体均可被 f() 调用
func f(p I) {
	println(p.Get())
	p.Set(0)
}

func main() {
	var s = S{1}	
    f(s)	// error: cannot use s (type S) as type I in argument to f
    		// S does not implement I (Get method has pointer receiver)

    f(&s)	// S 是 I 具体实现的一种，想要 Set() 生效必须传递地址    	
}
```

命名：只含单个方法的接口一般加上 er 后缀，如：Writer、Reader、Formatter 等



#### 类型判断

再添加一个实现 I 接口的结构体：

```go
type P struct { i int }
func (p *P) Get() int {	return p.i }
func (p *P) Set(v int) { p.i = v }
```

在函数 f 中可通过 type-switch 获取参数类型：

```go
func f(p I) {
	switch p.(type) {	// 在 switch 中使用类型判断
        case *S:	// 参数是 *S 类型
        case *P:
	    default: 	// 参数是其他实现 I 接口的类型
	}
}
```





### 并发：

#### goroutine

使用 go 关键字将函数作为 goroutine 执行时，将作为占用资源很少的协程运行。

其中 goroutine 是否真正的并行运行，取决于设置使用的 CPU 数量，可使用 `runtime.GOMAXPROCS(n int)` 或环境变量 `$GOMAXPROCS` 来设置，如果不手动设置，同一时刻 CPU 上也只会有 1 个 goroutine 在执行。此时是并发而非并行

```go
var c chan int

func main() {
	i := 0
	c = make(chan int)
	go ready("two", 2)	// 
	go ready("one", 1)
	fmt.Println("zero ready")

	//<-c		// 从 c 中接收整数并丢弃
	//<-c
    
L:
    for {					// 若不等待，则 main() 执行结束，任何 goroutine 都将停止执行
		select {			// select 用于选择不同类型的 channel
		case x := <-c:		// 从 channel c 中接收整数并保存到 x
			fmt.Println(x)
			i++
			if i > 1 {
				break L
			}
		}
	}
}


// 等待 sec 秒后输出指定内容
func ready(str string, sec int) {
	time.Sleep(time.Duration(sec) * time.Second)
	fmt.Println(str, " ready")
	c <- 1	// 将整数 1 发送到 channel c
}
```

运行效果：

 ![](https://contents.yinzige.com/goroutine-run.png)



#### channel

根据 size 决定创建的 ch 是否有缓冲

```go
// make(chan int)
// 无 size 或 size == 0, 此时 ch 无缓冲

// make(chan int, 2)	
// size > 0, 前 2 个元素可以无阻塞写入，第 3 个元素写入会阻塞直到其他 goroutine 从 ch 中读取值

ch := make(chan type, size ...int)
```

判断 channel 状态

```go
v, ok := <- ch	// ch 未关闭时 ok 为 true，且可读取到值到 v 中；关闭后 ok 为 false
```



### 通讯

#### 执行外部命令

类似 PHP 的 `exec()、system()`，Go 也能调用外部命令并执行，如：

```go
func main() {
	cmd := exec.Command("/bin/ls", "-la")
	b, err := cmd.Output()	// 返回命令的输出 []byte, error
	if err != nil {
		return
	}
	println(string(b))	// 效果类似于 ls -la
}
```



---

后续读第二遍时再加以补充、提交习题。

#### 