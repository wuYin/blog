---
title: gRPC 使用 protobuf 构建微服务
date: 2018-05-02 21:48:38
tags: 微服务
---

gRPC 使用 protobuf 通信构建微服务，本文代码：[GitHub](https://github.com/wuYin/blog/tree/master/codes/protobuf-with-grpc-in-golang)

<!-- more -->

本文目录：![image-20180504110112053](http://p7f8yck57.bkt.clouddn.com/2018-05-04-030112.png)



## 微服务架构

### 单一的代码库

以前使用 Laravel 做 web 项目时，是根据 MVC 去划分目录结构的，即 Controller 层处理业务逻辑，Model 层处理数据库的 CURD，View 层处理数据渲染与页面交互。以及 MVP、MVVM 都是将整个项目的代码是集中在一个代码库中，进行业务处理。这种单一聚合代码的方式在前期实现业务的速度很快，但在后期会暴露很多问题：

- 开发与维护困难：随着业务复杂度的增加，代码的耦合度往往会变高，多个模块相互耦合后不易横向扩展
- 效率和可靠性低：过大的代码量将降低响应速度，应用潜在的安全问题也会累积

### 拆分的代码库

微服务是一种软件架构，它将一个大且聚合的业务项目拆解为多个小且独立的业务模块，模块即服务，各服务间使用高效的协议（protobuf、JSON 等）相互调用即是 RPC。这种拆分代码库的方式有以下特点：

- 每个服务应作为小规模的、独立的业务模块在运行，类似 Unix 的 Do one thing and do it well
- 每个服务应在进行自动化测试和（分布式）部署时，不影响其他服务
- 每个服务内部进行细致的错误检查和处理，提高了健壮性

### 二者对比

本质上，二者只是聚合与拆分代码的方式不同。

 ![image-20180427190322810](http://p7f8yck57.bkt.clouddn.com/2018-04-27-image-20180427190322810.png)

参考：[微服务架构的优势与不足](http://dockone.io/article/394)





## 构建微服务

### UserInfoService 微服务

接下来创建一个处理用户信息的微服务：UserInfoService，客户端通过 name 向服务端查询用户的年龄、职位等详细信息，需先安装 gRPC 与 protoc 编译器：

```shell
go get -u google.golang.org/grpc
go get -u github.com/golang/protobuf/protoc-gen-go
```

#### 目录结构

```
├── proto
│   ├── user.proto		// 定义客户端请求、服务端响应的数据格式
│   └── user.pb.go		// protoc 为 gRPC 生成的读写数据的函数
├── server.go			// 实现微服务的服务端
└── client.go			// 调用微服务的客户端
```

#### 调用流程

 ![image-20180503174554852](http://p7f8yck57.bkt.clouddn.com/2018-05-03-094555.png)



### Protobuf 协议

每个微服务有自己独立的代码库，各自之间在通信时需要高效的协议，要遵循一定的数据结构来解析和编码要传输的数据，在微服务中常使用 protobuf 来定义。

[Protobuf](https://developers.google.com/protocol-buffers/)（protocal buffers）是谷歌推出的一种**二进制数据编码**格式，相比 XML 和 JSON 的**文本数据编码**格式更有优势：

#### 读写更快、文件体积更小

它没有 XML 的标签名或 JSON 的字段名，更为轻量，[更多参考](https://damienbod.com/2014/01/09/comparing-protobuf-json-bson-xml-with-net-for-file-streams/)

 ![](http://p7f8yck57.bkt.clouddn.com/2018-05-04-1.png)

#### 语言中立

只需定义一份 .proto 文件，即可使用各语言对应的 protobuf 编译器对其编译，生成的文件中有对 message 编码、解码的函数

##### 对于  JSON

- 在 PHP 中需使用 `json_encode()` 和 `json_decode()` 去编解码，在 Golang 中需使用 json 标准库的 `Marshal()` 和 `Unmarshal()` … 每次解析和编码比较繁琐
- 优点：可读性好、开发成本低
- 缺点：相比 protobuf 的读写速度更慢、存储空间更多

##### 对于 Protobuf

- *.proto 可生成 *.php 或 *.pb.go … 在项目中可直接引用该文件中编译器生成的编码、解码函数
- 优点：高效轻量、一处定义多处使用
- 缺点：可读性差、开发成本高



#### 定义微服务的 user.proto 文件

```protobuf
syntax = "proto3";	// 指定语法格式，注意 proto3 不再支持 proto2 的 required 和 optional
package proto;		// 指定生成的 user.pb.go 的包名，防止命名冲突


// service 定义开放调用的服务，即 UserInfoService 微服务
service UserInfoService {
    // rpc 定义服务内的 GetUserInfo 远程调用
    rpc GetUserInfo (UserRequest) returns (UserResponse) {
    }
}


// message 对应生成代码的 struct
// 定义客户端请求的数据格式
message UserRequest {
	// [修饰符] 类型 字段名 = 标识符;
	string name = 1;
}


// 定义服务端响应的数据格式
message UserResponse {
    int32 id = 1;
    string name = 2;
    int32 age = 3;
    repeated string title = 4;	// repeated 修饰符表示字段是可变数组，即 slice 类型
}
```

#### 编译 user.proto 文件

```shell
# protoc 编译器的 grpc 插件会处理 service 字段定义的 UserInfoService
# 使 service 能编码、解码 message
$ protoc -I . --go_out=plugins=grpc:. ./user.proto
```

#### 生成 user.pb.go

```go
package proto

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// 请求结构
type UserRequest struct {
	Name string `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
}

// 为字段自动生成的 Getter
func (m *UserRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

// 响应结构
type UserResponse struct {
	Id    int32    `protobuf:"varint,1,opt,name=id" json:"id,omitempty"`
	Name  string   `protobuf:"bytes,2,opt,name=name" json:"name,omitempty"`
	Age   int32    `protobuf:"varint,3,opt,name=age" json:"age,omitempty"`
	Title []string `protobuf:"bytes,4,rep,name=title" json:"title,omitempty"`
}
// ...

// 客户端需实现的接口
type UserInfoServiceClient interface {
	GetUserInfo(ctx context.Context, in *UserRequest, opts ...grpc.CallOption) (*UserResponse, error)
}


// 服务端需实现的接口
type UserInfoServiceServer interface {
	GetUserInfo(context.Context, *UserRequest) (*UserResponse, error)
}

// 将微服务注册到 grpc 
func RegisterUserInfoServiceServer(s *grpc.Server, srv UserInfoServiceServer) {
	s.RegisterService(&_UserInfoService_serviceDesc, srv)
}
// 处理请求
func _UserInfoService_GetUserInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {...}
```



### 服务端实现微服务

#### 实现流程

 ![image-20180504095727827](http://p7f8yck57.bkt.clouddn.com/2018-05-04-015728.png)

#### 代码参考

```go
package main
import (...)

// 定义服务端实现约定的接口
type UserInfoService struct{}
var u = UserInfoService{}

// 实现 interface
func (s *UserInfoService) GetUserInfo(ctx context.Context, req *pb.UserRequest) (resp *pb.UserResponse, err error) {
	name := req.Name

	// 模拟在数据库中查找用户信息
	// ...
	if name == "wuYin" {
		resp = &pb.UserResponse{
			Id:    233,
			Name:  name,
			Age:   20,
			Title: []string{"Gopher", "PHPer"}, // repeated 字段是 slice 类型
		}
	}
	err = nil
	return
}

func main() {
	port := ":2333"
	l, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("listen error: %v\n", err)
	}
	fmt.Printf("listen %s\n", port)
	s := grpc.NewServer()

    // 将 UserInfoService 注册到 gRPC
    // 注意第二个参数 UserInfoServiceServer 是接口类型的变量
	// 需要取地址传参
	pb.RegisterUserInfoServiceServer(s, &u)
	s.Serve(l)
}
```

运行监听：

 ![image-20180504094201953](http://p7f8yck57.bkt.clouddn.com/2018-05-04-014202.png)



### 客户端调用

#### 实现流程

![image-20180504100357221](http://p7f8yck57.bkt.clouddn.com/2018-05-04-020400.png)

#### 代码参考

```go
package main
import (...)

func main() {
	conn, err := grpc.Dial(":2333", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial error: %v\n", err)
	}
	defer conn.Close()

    // 实例化 UserInfoService 微服务的客户端
	client := pb.NewUserInfoServiceClient(conn)

	// 调用服务
	req := new(pb.UserRequest)
	req.Name = "wuYin"
	resp, err := client.GetUserInfo(context.Background(), req)
	if err != nil {
		log.Fatalf("resp error: %v\n", err)
	}

	fmt.Printf("Recevied: %v\n", resp)
}
```

运行调用成功： ![image-20180504094246792](http://p7f8yck57.bkt.clouddn.com/2018-05-04-014247.png)



## 总结

在上边 UserInfoService 微服务的实现过程中，会发现每个微服务都需要自己管理服务端监听端口，客户端连接后调用，当有很多个微服务时端口的管理会比较麻烦，相比 gRPC，[go-micro](https://github.com/micro/go-micro) 实现了服务发现（Service Discovery）来方便的管理微服务，下节将随服务的 Docker 化一起学习。

更多参考：[Nginx 的微服务系列教程](https://www.nginx.com/blog/introduction-to-microservices/)

















