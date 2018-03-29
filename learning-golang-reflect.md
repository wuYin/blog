---
title: Golang 中的反射
date: 2018-03-28 18:58:31
tags: Golang
---

上篇使用 toml 统一管理 echo 的路由和中间件，核心的映射操作就是使用 reflect 完成的，这篇文章就来深究一下反射。

<!-- more -->



## 定义

Golang 中变量的类型分为 2 种：

- static type：静态类型，在编译前后都是确定的，比如 int、string 等
- concrete type：具体类型，在程序运行时才知道的类型，比如与反射密切相关的  interface{}

`interface{}` 类型的变量由 2 个部分组成：变量的实际类型、变量的实际值

反射：在程序运行时用来检测类型、值的一种机制



## 接口变量的两个组成部分

### Type

使用 `reflect.TypeOf(v)` 可在运行时动态的获取接口变量的类型：

```go
// 返回 i 的实际类型，如果 i 是 nil 则返回 nil
func TypeOf(i interface{}) Type {
}    
```

使用 `Kind() Kind` 返回分类的值类型：基础类型 bool、string，数字类型，聚合类型array、struct，引用类型 chan、ptr、func、slice、map，接口类型 interface，无任何值的 Invalid 类型：

```go
fmt.Printf("%v\n", reflect.TypeOf(s).Kind())		// slice
fmt.Printf("%v\n", reflect.TypeOf(me.GetIntro).Kind())	// func
```



### Value

使用 `reflect.ValueOf(v)` 获取到变量的值，它是只读的。若想修改 v 的值，需使用 `reflect.ValueOf(&v)`

#### 获取、修改静态类型的变量值

使用 `Interface()` 能将变量的值以 `interface{}` 类型返回，再强制转换即可获取变量的实际值：

可使用 `Elem()` 来获取它们指向或存储的元素值:

```go
func main() {
	str := "old"
	strVal := reflect.ValueOf(str)		// 只读，不可修改
	fmt.Println(strVal.Interface())		// "old"
	// strVal.Elem().SetString("new")	// panic: reflect: call of reflect.Value.Elem on string Value	

	strPtrVal := reflect.ValueOf(&str)	// 取址，可修改
	// strPtrVal.Elem().SetInt(1)		// 不能设置不同类型的值 // panic: reflect: call of reflect.Value.SetInt on string Value	
	strPtrVal.Elem().SetString("new")	// Set 指定类型
	fmt.Println(str) 				// "new"
	strPtrVal.Elem().Set(reflect.ValueOf("newNew"))	// Set Value 类型
	fmt.Println(str) 				// "newNew"
}
```

可以看出， reflect 的大量方法使用不当会直接 panic，需小心使用。



#### 获取、修改未知的 struct 类型字段的值、调用方法

如果变量是 struct，可使用 `NumField()` 返回字段数量，再遍历获取、修改字段的值：

```go
type User struct {
	FirstName string `tag_name:"front"`
	LastName  string `tag_name:"back"`
	Age       int    `tag_name:"young"`
}

func main() {
	u := User{"wu", "Yin", 20}
	represent(u)

	uType := reflect.TypeOf(u)
	newU := reflect.New(uType) // 创建已知类型的变量
	newU.Elem().Field(0).SetString("Frank")
	newU.Elem().Field(1).SetString("Underwood")
	newU.Elem().Field(2).SetInt(50)

	newUser := newU.Elem().Interface().(User)	// newUser 是 User 类型，断言不会 panic
	fmt.Printf("%+v", newUser)
}

func represent(i interface{}) {
	t := reflect.TypeOf(i)
	v := reflect.ValueOf(i)
    
    
	// 使用 NumField() 来遍历探测结构体的字段值
	for i := 0; i < t.NumField(); i++ {
		fieldVal := v.Field(i)		// 注意调用者是 reflect.Value
		fieldType := t.Field(i)		// 注意调用者是 reflect.Type
		fieldTag := fieldType.Tag

		fmt.Printf("Field Name: %s\t Field Value: %v \tTag Value: %s\t\n",
			fieldType.Name,
			fieldVal,
			fieldTag.Get("tag_name"))
	}

	// 使用 NumMethod() 来遍历探测结构体的方法
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Printf("%s :%v\n", m.Name, m.Type)
	}
    
	// 通过方法名来调用
	// 如果方法不存在，则 panic: reflect: call of reflect.Value.Call on zero Value
	m := v.MethodByName("Intro")
	
	// args := make([]reflect.Valuye, 0)	// 方法无参数时
	
	// 有参数时，参数类型是 reflect.Value
	args := []reflect.Value{reflect.ValueOf("Beijing"), reflect.ValueOf("Xian")}
	m.Call(args)
}

func (u User) Intro(workLoc string, studyLoc string) {
	fmt.Printf("My name is %s%s, age %d, working in %s and study in %s\n",
		u.FirstName, u.LastName, u.Age, workLoc, studyLoc)
}
```

输出：

```
Field Name: FirstName	 Field Value: wu 	Tag Value: front	
Field Name: LastName	 Field Value: Yin 	Tag Value: back	
Field Name: Age	 	 Field Value: 20 	Tag Value: young	
Intro :func(main.User, string, string)
My name is wuYin, age 20, working in Beijing and study in Xian
{FirstName:Frank LastName:Underwood Age:50}
```

如果 struct 组合嵌套了，可以使用递归来处理。







## Make 系列方法

除使用 `make()` 来为 slice、map 和 channel 分配空间，还能用反射包中的 Make 系列方法：

```go
func MakeSlice(typ Type, len, cap int) Value {}
func MakeMap(typ Type) Value {}
func MakeMapWithSize(typ Type, n int) Value {}
func MakeChan(typ Type, buffer int) Value {}
```

拿 slice 和 map 举个例子：

```go
func main() {

	intSlice := make([]int, 0)
	strIntMap := make(map[string]int)

	sliceType := reflect.TypeOf(intSlice)
	mapType := reflect.TypeOf(strIntMap)
	reflectSlice := reflect.MakeSlice(sliceType, 0, 0)	// 创建 reflect 自己的 slice 和 map
	reflectMap := reflect.MakeMap(mapType)

	i := 233
	iVal := reflect.ValueOf(i)
	reflectSlice = reflect.Append(reflectSlice, iVal)	// reflect 有自己实现的 Append()
	originSlice := reflectSlice.Interface().([]int)		// 使用类型断言来转换值
	fmt.Println(originSlice)				// [233]

	s := "str"
	sVal := reflect.ValueOf(s)
	reflectMap.SetMapIndex(sVal, iVal)
	originMap := reflectMap.Interface().(map[string]int)
	fmt.Println(originMap)					// map[str:233]
}    
```



reflect 还能创建函数：

```go
func MakeFunc(typ Type, fn func(args []Value) (results []Value)) Value {}
```

例子：

```go
func main() {
	newFunc := MyMakeFunc(beYounger).(func(i int) int)	// 传入函数类型
	res := newFunc(20)
	print(res)
}

func MyMakeFunc(fun interface{}) interface{} {
	funcVal := reflect.ValueOf(fun) // funVal.Kind() 必须是 reflect.Func
	funcType := funcVal.Type()

	newFun := reflect.MakeFunc(funcType, func(in []reflect.Value) []reflect.Value {
		println("创建的新函数被调用")
		return funcVal.Call(in)
	})

	return newFun.Interface()
}

func beYounger(age int) int {
	return age - 10
}
```

输出：

```
创建的新函数被调用
10
```





## 总结

Go 的反射经常操作 `interface{}` 类型的变量，在程序运行前是不知道这个变量的具体类型和值的，反射就提供了这种在运行时检测和操作 `interface{}` 类型变量的机制，比如经常用来调试的 `fmt.Printf("%v", v)`函数，内部实现就使用了大量的反射。

初学容易混淆 `reflect.Type` 和 `reflect.Value` ，尤其是

```
		没有方法 // 有类型不代表有值
 		------>
reflect.Type			reflect.Value
		<------
		.Type()	// 有值肯定有类型
```

另外，在使用反射时，一定要熟悉方法的参数类型等，否则容易造成 panic























