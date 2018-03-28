---
title: learning-golang-reflection
date: 2018-03-28 18:58:31
tags: Golang
---

上篇使用 toml 统一管理 echo 的路由和中间件，核心的映射操作就是使用 reflect 完成的，这篇文章就来深究一下反射。

<!-- more -->

## 定义

在 Golang 中，定义变量、新类型、函数都很直接：

```Go
// 定义类型
type User struct {
	Name string
	Age  int
}

// 定义变量
var me User

// 定义函数
func (u User) GetIntro() string {
	return fmt.Sprintf("Name is %s, age is %d", u.Name, u.Age)
}

func main() {
	me.Name = "wuYin"
	me.Age = 20
    fmt.Print(me)	// {wuYin 20}	// 内部调用的 printArg() 大量使用了 reflect
}
```

`fmt.Print(a ...interface{})` 的参数类型是 `interface{}`，并不是具体的类型，但它依旧能打印出变量的具体值。查看源码，会发现它调用的 `printArg()` 先使用了 reflect 来推断 `a` 的类型，再输出。

不难看出反射的用武之地：编译时不知道值的具体类型，可在运行时使用反射来动态的操作变量。



## 三个概念

对于反射，有 3 个组成部分：`Types`、`Kinds`、`Values`

### Types

使用 `reflect.TypeOf(v)` 可在运行时动态的获取变量的类型：

```go
func TypeOf(i interface{}) Type {
	// 返回 reflect.Type 类型，定义了各种与 i 的具体类型相关的方法
}    
```

reflect.Type 类型的变量可调用的方法很多，如：



#### `Name() string`

返回具体类型的名字。如果类型没有具体的名字，比如 slice、pointer 等则返回空字符串：

```go
t := reflect.TypeOf(me)
fmt.Printf("%v", t.Name())	// User

s := []string{"str"}
fmt.Printf("%v\n", reflect.TypeOf(s).Name() == "")	// true
```



#### `Kind() Kind`

返回分类的值类型：基础类型 bool、string，数字类型，聚合类型array、struct，引用类型 chan、ptr、func、slice、map，接口类型 interface，无任何值的 Invalid 类型：fa

```go
fmt.Printf("%v\n", reflect.TypeOf(s).Kind())				// slice
fmt.Printf("%v\n", reflect.TypeOf(me.GetIntro).Kind())		// func
```













