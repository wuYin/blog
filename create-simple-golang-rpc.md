---
title: 使用反射实现简易的 RPC 框架
date: 2018-09-11 17:22:49
tags: RPC
---

经过两个月的迷茫和爬坑，我小吴又回来了，本文总结下 RPC 的简单实现。

本文代码：[GitHub](https://github.com/wuYin/simple_rpc)

<!-- more -->

## 背景

微服务架构下数据交互一般是对内 RPC，对外 REST，拿笔者所在的社交 App 后端业务举例：用户注册时客户端会带上输入的手机号请求 API 层，API 将手机号传递给短信微服务，短信微服务再调用阿里大鱼的短信接口，下发验证码。

![image-20180911184600689](http://p7f8yck57.bkt.clouddn.com/2018-09-11-104601.png)

其实短信发送的业务完全可以放到 API 层直接做，session 和 profile 的业务同理。但这么做有 3 个缺点：

- 部署效率低：如果加上 websocket（保持与客户端长连接）、goexif（用户头像解码）... 等各种第三方依赖，API 项目下的 `vendor/` 将会变得臃肿，上辄几百 MB，每次编译、部署和测试过程都需要大量时间等待。

- 开发成本高：当业务繁杂模块较多时，每个模块添加新功能或 fix bug 都要重新完整发布 API 项目，重新测试，测试不通过还得重新发布。

- 系统可用性差：所有模块功能都编译到一个可执行文件中，若某一模块代码出现问题，将可能导致整个 API 项目挂掉，所有服务不可用。比如在用户位置模块中有经纬度转城市的功能，需要调用高德地图的 API，使用 [gopool](https://github.com/go-playground/pool) 库批量并发的去请求转换，忘记调用 `batch.QueueComplete()` 结果导致 pool 中 goroutine 的数量只增不减，可能拖垮整个 API 项目。

  ![1](http://p7f8yck57.bkt.clouddn.com/2018-09-11-112426.jpg)

将业务按功能模块拆分到各个微服务，具有提高项目协作效率、降低模块耦合度、提高系统可用性等优点，但是开发门槛比较高，比如 RPC 框架的使用、后期的服务监控等工作。

本文实现一个极简的 RPC 框架，完成 Client 远程调用 Server 的核心功能，姑且不考虑超时重连、心跳保活等网络层机制。

## 本地调用

在程序中，常常将代码段封装成函数执行。如：

```go
package main

import "fmt"

type User struct {
	Name string
	Age  int
}

func main() {
	u, err := queryUser(6)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("name: %s, age: %d\n", u.Name, u.Age)
}

// 模拟数据库查询
func queryUser(uid int) (User, error) {
	userDB := make(map[int]User)
	userDB[0] = User{"Dennis", 70}
	userDB[1] = User{"Ken", 75}
	userDB[2] = User{"Rob", 62}
	if u, ok := userDB[uid]; ok {
		return u, nil
	}
	return User{}, fmt.Errorf("id %d not in user db", uid)
}
```

函数 `queryUser()` 在本地代码库中直接调用，就能查询到想要的用户信息。 

## RPC 调用

现将模拟的用户数据作为单独的服务运行，客户端通过网络实现调用。大致流程图如下：

![2](http://p7f8yck57.bkt.clouddn.com/2018-09-12-084302.jpg)

注：client 和 server 可以是两台不同 IP 的主机，也可以是本机上两个端口不同的程序。

如上图，实现调用的前提是 server 能解析请求数据，client 能解析响应数据，即两端要约定好数据包的格式。

### 网络传输数据格式

成熟的 RPC 框架会有自定义 TLV 协议（固定长度消息头 + 变长消息体）等。在 [simple_rpc](https://github.com/wuYin/simple_rpc) 中尽量简化，包的格式如下：

![image-20180912092854182](http://p7f8yck57.bkt.clouddn.com/2018-09-12-012854.png)

读取网络字节流时，需要知道要读取多少字节作为的数据部分，故在头部中使用 4 字节长的 header 部分来标识 data 的长度。读写如下：

```go
package simple_rpc

import (
	"encoding/binary"
	"io"
	"net"
)

type Session struct {
	conn net.Conn
}

// 向连接中写数据
func (s *Session) Write(data []byte) error {
	buf := make([]byte, 4+len(data))                       // 4 字节头部 + 数据长度
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data))) // 写入头部
	copy(buf[4:], data)                                    // 写入数据
	_, err := s.conn.Write(buf)
	if err != nil {
		return err
	}
	return nil
}

// 从连接中读数据
func (s *Session) Read() ([]byte, error) {
	header := make([]byte, 4)
	_, err := io.ReadFull(s.conn, header)
	if err != nil {
		return nil, err
	}
	dataLen := binary.BigEndian.Uint32(header)
	data := make([]byte, dataLen)
	_, err = io.ReadFull(s.conn, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
```

注：binary 包只认固定长度的类型，故 header 使用 uint32 而非 int

```go
func TestSession_ReadWrite(t *testing.T) {
	addr := "0.0.0.0:2333"
	cont := "yep"
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		conn, _ := l.Accept()
		s := Session{conn: conn}
		err = s.Write([]byte(cont))
		if err != nil {
			t.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		s := Session{conn: conn}
		data, err := s.Read()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != cont {
			t.FailNow()
		}
	}()

	wg.Wait()
}
```

测试读写正常：

![image-20180912094311165](http://p7f8yck57.bkt.clouddn.com/2018-09-12-014311.png)



## 反射与 RPC

server 端接收到的数据需要包括：调用的函数名、参数列表。一般我们会约定第二个返回值是 error 类型，表示 RPC 调用结结果（gRPC 标准）

### Call 执行调用

RPC Server 需解决 2 个问题：

- Client 调用时只传过来函数名，需要维护函数名到函数之间的 map，才能知道 Client 想要执行什么函数
- 从 reflect.Value 到函数调用，使用 `Value.Call()` 函数

```go
package main

import (
	"fmt"
	"reflect"
)

func main() {
	funcs := make(map[string]reflect.Value) // server 端维护 funcName => func 的 map
	funcs["incr"] = reflect.ValueOf(incr)
	args := []reflect.Value{reflect.ValueOf(1)} // 构建参数（client 传递上来）
	vals := funcs["incr"].Call(args)            // 调用执行
	var res []interface{}
	for _, val := range vals {
		res = append(res, val.Interface()) // 处理返回值
	}
    fmt.Println(res)	// [2, <nil>]
}

func incr(n int) (int, error) {
	return n + 1, nil
}
```

看到这里，RPC Server 端的核心工作如下：

- 维护函数名到函数反射值的 map
- client 端传递函数名、参数列表后，解析为反射值，调用执行
- 函数的返回值打包通过网络返回给客户端

### MakeFunc 生成调用

RPC Client 需解决问题：函数的具体实现在 Server 端，Client 只有该函数的原型。使用 `MakeFunc()` 完成原型到函数的调用。

```go
package main

import (
	"fmt"
	"reflect"
)

func main() {
	swap := func(args []reflect.Value) []reflect.Value {
		return []reflect.Value{args[1], args[0]}
	}

	var intSwap func(int, int) (int, int)
	fn := reflect.ValueOf(&intSwap).Elem() // 获取 intSwap 未初始化的函数原型
	v := reflect.MakeFunc(fn.Type(), swap) // MakeFunc 使用传入的函数原型创建一个绑定 swap 的新函数
	fn.Set(v)                              // 为函数 intSwap 赋值

	fmt.Println(intSwap(1, 2)) // 2 1
}
```



### RPC 数据

我们定义 RPC 交互的数据格式，即要存储到上边网络字节流中 `data` 部分的数据：

```go
type RPCData struct {
	Name string
	Args []interface{}
}
```

定义其对应的编码解码函数：

```go
func encode(data RPCData) ([]byte, error) {
	var buf bytes.Buffer
	bufEnc := gob.NewEncoder(&buf)
	if err := bufEnc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode(b []byte) (RPCData, error) {
	buf := bytes.NewBuffer(b)
	bufDec := gob.NewDecoder(buf)
	var data RPCData
	if err := bufDec.Decode(&data); err != nil {
		return data, err
	}
	return data, nil
}
```



## Server 端

### 结构

server 端需要维护连接与 RPC 函数名到 RPC 函数本身的映射，结构如下：

```go
type Server struct {
	addr  string
	funcs map[string]reflect.Value
}
```

### 注册函数

将函数名与函数的真正实现对应起来：

```go
func (s *Server) Register(rpcName string, f interface{}) {
	if _, ok := s.funcs[rpcName]; ok {
		return
	}
	fVal := reflect.ValueOf(f)
	s.funcs[rpcName] = fVal
}
```

### 执行调用

为了看清楚服务端的工作流程，暂且忽略错误处理：

```go
// 等待
func (s *Server) Run() {
	l, _ := net.Listen("tcp", s.addr)
	for {
		conn, _ := l.Accept()
		srvSession := NewSession(conn)

		// 读取 RPC 调用数据
		b, _ := srvSession.Read()

		// 解码 RPC 调用数据
		rpcData, _ := decode(b)

		f, ok := s.funcs[rpcData.Name]
		if !ok {
			fmt.Printf("func %s not exists", rpcData.Name)
			return
		}
        
		// 构造函数的参数
		inArgs := make([]reflect.Value, 0, len(rpcData.Args))
		for _, arg := range rpcData.Args {
			inArgs = append(inArgs, reflect.ValueOf(arg))
		}

		// 执行调用
		out := f.Call(inArgs)
		outArgs := make([]interface{}, 0, len(out))
		for _, o := range out {
			outArgs = append(outArgs, o.Interface())
		}

		// 包装数据返回给客户端
		respRPCData := RPCData{rpcData.Name, outArgs}
		respBytes, _ := encode(respRPCData)
		srvSession.Write(respBytes)
	}
}
```



## Client 端

直接调用即可：

```go
// fPtr 指向函数原型
func (c *Client) callRPC(rpcName string, fPtr interface{}) {
	fn := reflect.ValueOf(fPtr).Elem()

    // 完成与 Server 的交互
	f := func(args []reflect.Value) []reflect.Value {
		// 处理输入参数
		inArgs := make([]interface{}, 0, len(args))
		for _, arg := range args {
			inArgs = append(inArgs, arg.Interface())
		}

		// 编码 RPC 数据并请求
		cliSession := NewSession(c.conn)
		reqRPC := RPCData{Name: rpcName, Args: inArgs}	
		b, _ := encode(reqRPC)
		cliSession.Write(b)
		
		// 解码响应数据，得到返回参数
		respBytes, _ := cliSession.Read()
		respRPC, _ := decode(respBytes)

		outArgs := make([]reflect.Value, 0, len(respRPC.Args))
		for i, arg := range respRPC.Args {
			// 必须进行 nil 转换
			if arg == nil {
				outArgs = append(outArgs, reflect.Zero(fn.Type().Out(i)))
				continue
			}
			outArgs = append(outArgs, reflect.ValueOf(arg))
		}
		return outArgs
	}
	v := reflect.MakeFunc(fn.Type(), f)
	fn.Set(v)
}
```

`MakeFunc` 是 Client 从函数原型到网络调用的关键。



## 测试

```go
func TestRPC(t *testing.T) {
	gob.Register(User{})

	addr := "0.0.0.0:2333"
	srv := NewServer(addr)
	srv.Register("queryUser", queryUser)
	go srv.Run()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Error(err)
	}
	cli := NewClient(conn)

	var query func(int) (User, error)
	cli.callRPC("queryUser", &query)
	
	// RPC 调用
	u, err := query(1)
	fmt.Println(err, u)
}

type User struct {
	Name string
	Age  int
}

func queryUser(uid int) (User, error) {
	userDB := make(map[int]User)
	userDB[0] = User{"Dennis", 70}
	userDB[1] = User{"Ken", 75}
	userDB[2] = User{"Rob", 62}
	if u, ok := userDB[uid]; ok {
		return u, nil
	}
	return User{}, fmt.Errorf("id %d not in user db", uid)
}
```

RPC 调用成功，测试通过：

![1](http://p7f8yck57.bkt.clouddn.com/2018-09-12-082752.jpg)



## 总结

如测试文件中所示，`queryUser()` 没有在 server.go 中实现，所以本文的 demo 并不是完全意义上的 RPC 框架，不过阐释清楚了 RPC 的核心点：反射调用。

上边的 demo 使用裸 `net.Conn` 进行阻塞式的读写。投入生产环境的 RPC 框架往往有着健壮的底层网络机制，比如使用非阻塞式 IO 读写、实现 Client 与 Server 端保持超时重连、心跳检测等等复杂的机制。