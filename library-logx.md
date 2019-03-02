---
title: 高效简单的 logx 日志库
date: 2019-03-02 10:20:03
tags: Golang
---

简要介绍 [wuYin/logx](https://github.com/wuYin/logx) 的设计思路。

<!-- more -->

## 日志

在后端项目中，日志系统常用来记录代码执行中的关键信息，如客户端某次请求处理异常，则需记录请求参数以便后续分析。

### log 标准库

[log](https://golang.org/pkg/log/) 标准库提供简洁的日志记录方式，如：

```go
func main() {
	f, _ := os.OpenFile("service.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	log.SetOutput(f)
	log.Printf("invalid uid: %s", "10086")
	log.Fatalf("disk usage 100%")
}
```

日志会被记录到 `service.log`，下边日志文件以此为例：

```
2019/03/02 11:43:23 invalid uid: 10086
2019/03/02 11:43:23 disk usage full
```

### log 改进

**日志分级**：在开发中，日志往往需要分级，如常见的 6  个日志等级

```go
FINE	// 最低级别，记录所有日志
INFO	// 收集程序运行的有用信息
DEBUG	// 记录程序调试信息
WARN	// 输出潜在的错误
ERROR	// 发生的错误但不影响程序的运行，用易读易定位的方式收集好错误信息，方便后期排查
FATAL	// 最高级别，发生严重错误时像 panic 一样直接退出程序，如 disk full 等错误
```

其中，高级别日志会同时输出低级别日志中，如 WARN 的日志会同时写到 DEBUG 日志，等级写关系如下：

![image-20190302120553244](https://images.yinzige.com/2019-03-02-040553.png)

**切割备份：**

- 一般程序日志需按大小分割，便于管理和打包，比如配置每个 `service.log` 最多有 100MB
- 当写超过 100MB 就将 service.log 备份为 `service.2019-03-02-001.log` 类似格式的日志



## logx 日志库

源码：[wuYin/logx](https://github.com/wuYin/logx)，使用详见 [README.md](https://github.com/wuYin/logx/blob/master/README.md)

### 日志格式

可参考 PHP $Exception 的输出，日志行最少需包括：

- 写入时间
- 日志等级
- 写日志的文件行
- 格式化后的日志数据

于是将日志行内容抽象成结构 `LogRecord`

```go
type LogRecord struct {
	Level   Level     // 日志级别
	Created time.Time // 记录时间
	Source  string    // 源文件位置
	Message string    // 日志信息
}
```

最后输出日志类似于：

```
[2019/03/01 16:42:20 CST] [FATL] (logx.TestLogger_DebugLog:17) System|disk full
```



### 结构关系

![image-20190302132722215](https://images.yinzige.com/2019-03-02-052722.png)



日志可输出到终端（标准输出），日志文件，甚至是日志服务器的 socket 连接。

将写操作抽象成接口 `LogWriter` 

```go
type LogWriter interface {
	LogWrite(rec *LogRecord) // 写日志
	Close()                  // 日志写完毕后的资源清理工作
}
```

定义终端输出结构：

```go
type ConsoleLogWriter struct {
	format  string
	writeCh chan *LogRecord
}
```

定义文件日志输出：

```go
type FileLogWriter struct {
	format  string
	fName   string
	file    *os.File
	writeCh chan *LogRecord
}
```



### 核心模块

- 写数据

  如上实现了两个 `LogWriter` 接口的 writer，它们都有接收 `LogRecord` 的缓冲 channel，当 channel 中有日志行数据时，接收写入到对应的 io.Writer 输出流中

- writer 分类

  程序可能需要将不同业务的日志写入到不同名的文件，比如接口层写到 handler.log，服务层写到 service.log。这里将每个日志实体抽象成 Filter

  ```go
  type Filter struct {
  	MinLevel Level
  	LogWriter
  }
  ```

- 集中管理 Filter

  filter 是实际写入日志的程序，集中管理成 logger

  ```go
  type Logger map[string]*Filter
  ```

  



## 切割备份

对于文件类型的日志，可定义 maxSize 和 maxLine 来限制单个日志的最大空间和最大行数，因为每次向文件写入数据时写入的字节数是可知的，所以可监控日志增长。

当日志空间满或行数满后，需要将当前日志备份成日期-序号的命名，具体备份细节直接看源码和注释即可。



## 总结

logx 是照着我们内部生产环境使用的日志库仿写的，整体流程不是很复杂，不过也比较完善，够用。此外，logx 在 FATAL 日志使程序 panic 后可选 recover 等方面还有改进空间，放到 Todo list 吧，感谢关注。











 