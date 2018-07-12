---
title: 编写可读代码的艺术
date: 2018-07-11 09:23:43
tags: 规范
---

《编写可读代码的艺术》读书笔记与 Golang, Redis 编程规范。

<!-- more -->

# 前言

## 目标

在我的工作开发中，大部分时间都在写模块函数实现业务、阅读已有代码，常常需斟酌命名是否恰当、流程结构是否清晰等问题，遂读本书，目的在于：**使自己的代码更加容易理解**

衡量可读性的标准：别人理解代码块的作用、存在的问题、如何修改代码所需的最少时间。可读性高的代码，别人（or 隔月的自己）看了以后就像他自己刚刚写好的一样。

## 结构

本文分为两个章节简述何为 **"readable code"**

- 表面上的改进：有意义的命名、好的注释
- 循环与流程的简化：程序中循环、逻辑流程的简化

最后梳理了 Golang 编程规范、Redis 规范。



# 表面上的改进

改进：使用有意义的命名、言简意赅的注释、严格的代码缩进（为 gofmt 点赞）

## 命名

### 1. 使用专业词汇

| 概括性词汇 |                   精确清晰的词汇                   |
| :--------: | :------------------------------------------------: |
|    send    |   deliver, dispatch, announce, distribute, route   |
|    find    |          search, extract, locate, recover          |
|   start    |            launch, create, begin, open             |
|    make    | create, set up, build, generate, compose, add, new |

为变量、函数和类命名时，尽量用清晰且精确的词，避免使用概括性的词。可多阅读优秀开源项目的源码，或 [谷歌翻译](https://translate.google.cn/) 后慢慢积累。



### 2. 避免使用 tmp, result, foo 等泛词

在局部代码块写临时变量时，一时想不出好名字就会用 tmp, result, foo 来代替，实际上可根据变量 **存在目的**、**代表值的意义** 来命名更具描述性。

```c
void swap(int x,int y) {
	int tmp;	// tmp 恰当	// 灵活运用
	temp = x;
	x = y;
	y = temp;
}
```

循环可使用 `i, j, k` 来作为迭代器的命名，多层循环可将索引缩写：

```go
// 检查用户属于哪个俱乐部
for (int i = 0; i < clubs.size(); i++) {
    for (int j = 0; j < clubs[i].members.size(); j++) {
        for (int k = 0; k < users.size(); k++) {
            if (clubs[i].members[k] == users[j]) {}	// i, j, k 容易混淆
            if (clubs[ci].members[mi] == users[ui]) {}	// 缩写索引 ci, mi, ui 就很清晰 	
        }
    }
}    
```



### 3. 命名附加更多信息

```go
var id string = "af84ef845cd8";		// 十六进制 ID，用 hexID 或许更恰当

// 变量值带单位
curNanoSec := time.Now().Nanosecond() 	// 当下纳秒数，在变量名中附加 Nano 信息

txtPwd			// 明文密码
utf8HTML		// UTF8 编码的 HTML 源码
unescapedContent	// 未转义的内容
...
```



### 4. 命名长度恰当

建议代码一行不超过 80 字符，过长的命名会导致过多的换行、难以识记，应当避免（没有“黑” Zend 风格）

- 在 **小作用域 **内用 **短小** 的命名：变量类型、初始化、引用处一目了然
- 在 **大作用域** 内用 **包含足够信息** 的命名：可包含多个单词释义

恰当使用缩略词，大家公认的如：

|      缩略词       |             原词              |
| :---------------: | :---------------------------: |
|        avg        |            averge             |
|     bg / buf      |      background / buffer      |
|     del / doc     |       delete / document       |
|     err / esc     |        error / escape         |
| info / init / img | infomation / initial / image  |
|     len / lib     |       length / library        |
|        msg        |            message            |
|  pwd / pic / pos  | password / picture / position |
|  srv / src / str  |   server / source / string    |
|     tmp / txt     |          temp / text          |

可参考 HTML 各种缩略标签名，其他缩略名尽量不使用（保证半年后自己看还能看懂？）



### 5. 命名格式传递含义

```php
<?php

class User
{
	const DB_USER_NAME = "root";	// 全大写为常量
	public $name;	
	private $_age;			// 约定私有变量 _ 开头
}
```





## 无歧义的名字

### 三组表示区间的词

#### 使用 [min, max] 表示包含的极限值

常使用 min 表示下限，max 表示上限。一般命名会加 `max_`和`min_` 前缀。如：

```go
const PAGE_UNIT_LIMIT = 10	// limit 后缀未表明是上限还是下限
const MAX_PAGE_UNIT = 10	
if n > MAX_PAGE_UNIT {
    return 
}
```

#### 使用 [first, last] 表示包含的范围值

#### 使用 [begin, end) 表示排除 / 包含的范围值



### 布尔值命名

给布尔变量、返回值为布尔类型的函数命名时，加上以下前缀意义可能更明确：

- `isXxx`：是不是
- `canXxx`：能不能
- `hasXxx`：有没有
- `xxxable`：是否可以



## 规整简洁的代码

### 使用列对齐（可选）

```php
public function getUserInfo() {
	// 设置等号对齐，方便在列之间切换阅读
    $age        = 10;	
    $mergedName = $this->_firstName . ' ' . $this->_lastName;
}
```

PHPStorm 可做如下设置，在格式化代码时自动对齐（给 gofmt 点个踩）：

 <img src="http://p7f8yck57.bkt.clouddn.com/2018-07-11-135520.png" width=600 />

注：在团队开发中，若有人不遵循列对齐，那改动并格式化他的文件后，Git 需提交很多格式上的改动。





# 循环与流程的简化

## 简化流程控制

### 1. 条件语句中参数的顺序

```go
if len(arr) <= 1{
	//  if 条件语句：[变化值 比较运算符 恒定值]	
}
if 1 <= len(arr) {
	// 不符合阅读习惯
}
```



### 2. `if/else` 语句块的顺序

```go
user, err := getUserInfo(id)
if err != nil {		// if 语句尽量先处理“负”逻辑
    return 
} 
// 尽量省略 else 语句
// 处理 “正逻辑”
```



### 3. 从函数中提前 return

在函数中进行正常的逻辑处理前，尽量多使用 `return` 处理异常情况：

```go
// 对比数组全部元素是否一致
func IsEqual(arr1, arr2 []int) bool {
	if arr1 == nil || arr2 == nil {
		return false
	}
	if len(arr1) != len(arr2) {
		return false
	}

	for i := range arr1 {
		if arr1[i] != arr2[i] {
			return false
		}
	}
	return true
}
```

提前 return 还可以减少 `if-else` 嵌套的数量。使代码变得 **线性** 整洁。



---



# Golang 编程规范

## 命名

### var 变量

- 驼峰式，一般在简单上下文使用简写，如 user 简写为 u
- 一般布尔类型使用 is、has、can 等前缀
- 专有名词使用完整的原有写法，如 curURL，userID

### const 常量

全大写，使用 `_` 分隔

### func 函数

驼峰式，一般是动词短语。常使用 get、set、is 等前缀， with、and 等过渡词

### struct 结构体

驼峰式，一般是名词短语，声明和初始化都使用多行对齐

```go
type User struct{
    Username  string
    Email     string
}

u := User{
    Username: "astaxie",
    Email:    "astaxie@gmail.com",
}
```

​

## 编码

### package

- package 名与目录名一致，在任一文件中写清这个  package 的注释，描述包的功能
- import package  统一使用 `package (...)` 分组
- 包的排序：标准库、程序内部库、第三方库。包的路径：使用全局路径



### 函数

命名返回参数

- 函数短小：不使用命名参数
- 有多个同类型的返回参数：可使用命名参数来区分

函数 receiver  使用值还是指针，视调用者在函数内部是否会被修改而定：

```go
func(w Win) Tally(playerPlayer)int    // w 不会有任何改变 
func(w *Win) Tally(playerPlayer)int   // w 会改变数据
```

格式化输出：长句使参数换行

```go
log.Printf( 
    “A long format string: %s %d %d %s”, 
    myStringParameter,
    len(a),
    expected.Size,
    defrobnicate(
        “Anotherlongstringparameter”,
        expected.Growth.Nanoseconds()/1e6, 
    ),
）
```



## 错误处理

不丢弃任何可能产生 errror 的返回，使用 logger 等处理。在逻辑处理中不使用 panic，除非会导致程序功能完全不能使用，应用 log.Fatal 等记录并退出

```go
// 好的：一个代码块单元只做一件事
if err != nil {
    // 错误处理
	return // 或者 logger 处理
}
// 正常逻辑


// 不好的
if err != nil {
	// 错误处理
} else {
    // 正常逻辑
}
```



## 工具

- 使用 `go vet` 分析代码的潜在的如局部变量覆盖问题
- 使用 `gofmt` 来格式化代码



---



# Redis 规范

## Key 规范

### 命名与长度

- 一般 key-value 存储设计：`表名 : mysql主键列名 : 主键值 : 列名`，存储该列的值。
- 命名：业务或 db、表名为前缀，冒号分隔。单词间用 `.` 连接。
- 长度：过长的 key 内存占用多
- 缩写：如 `user:{uid}:friends:messages:{mid} ` 简化为 `u:{uid}:fr:m:{mid}`

### 过期时间

- 建议使用 expire 设置过期时间，可打散 keys 避免集中过期
- 不过期的 key 使用 `object idletime key` 检查 key 距上次使用的空闲时间


### value 规范

- 建议：选择合适的数据类型来存储
- 强制：不使用 big key
  - string类型控制在 10KB 以内
  - hash、list、set、zset 元素个数不要超过 5000


## 命令规范

### 需注意元素个数的命令

```php
HGETALL key		# 返回哈希表 key 中，所有的域和值
LRANGE key start stop 	# 返回列表 key 中指定区间内的元素，区间以偏移量 start 和 stop 指定
ZRANGE key start stop [WITHSCORES]	# 返回有序集 key 中，指定区间内的成员，按 score 值递增排序
SINTER key [key ...]			# 返回一个集合的全部成员，或给定多个集合的交集
```

以上命令复杂度为 `O(N)`，在使用时需关注 N 的大小，否则将造成阻塞。

各种数据集合的遍历操作，可使用 hscan、sscan、zscan 等来渐进式的获取集合中的元素，每次 10 个。

### 线上禁止的命令

redis 是单线程，一个命令在处理过程中无法处理其他命令，将造成阻塞。可在 redis.conf 配置 `rename-command FLUSHALL ""` 禁用：

- keys：禁止使用正则操作，key 数量多时效率很低。
- flushall、flushdb：清空数据库与记录的危险操作。



### 批量操作

使用原子操作 mget、mset 批量的设置值和取值可提高效率，或使用 pioeline 批量执行多种多个命令。一次批量操作的元素数也要控制，如 500 以内。

### Redis 事务与监控

使用 multi、exec 实现。redis 中命令是原子性的，但事务不是，如果任一命令中途执行失败，则事务失败，并不会回滚，慎用。monitor 避免长时间运行：返回服务器处理的所有命令，会明显消耗性能。

### 删除大键

big key：占用内存大或含有元素个数多。如string 键的值占用接近最大 521 MB、set/zset/hash/list 有1kw 元素

- 直接删除（错误）：`DEL` 的复杂度为 `O(N)`，元素个数 N 在 100w 个时删除耗时 1s，阻塞其他命令。
- 渐进删除（正确）：`scan` 类的复杂度为 `O(1)` 的命令，如 `hscan` 获取 500 个元素使用 `hdel` 删除。



# 总结

无意中在公司书架上找到了《ARC》，粗略看了一遍，学到了一些好的编程实践，特此总结。顺带把以前的笔记也放到博客里，慢慢养成良好的编程习惯。

译者严哲在序中写到，那些做市场推广的人穿着整洁，表示对客户和工作的尊重。同样，对我们软件工程师群体，对于工作尊重和责任体现在自己写的代码中。