---
title: Golang 微服务教程（二）
date: 2018-05-12 14:54:54
tags: 微服务
---

Golang 实现微服务系列教程，共计 10 篇高质量文章。原文链接：[ewanvalentine.io](https://ewanvalentine.io/)，翻译已获作者授权。

<!-- more -->

> 注：本节未细致介绍 Docker，更多可参考：[《第一本Docker书 修订版》](https://book.douban.com/subject/26780404/)



## 前言

在上一篇中，我们使用 gRPC 初步实现了我们的微服务，本节将 Docker 化该微服务并引入 go-micro 框架代替 gRPC 简化服务的实现。



## Docker

### 背景

占据着云计算的优势，微服务架构越来越流行，同时它的云端分布式的运行环境也对我们的开发、测试和部署提出了很高的要求，容器（container）便是一项解决方案。

在传统软件开发中，应用直接部署在环境和依赖都准备好的系统上，或在一台物理服务器上部署在由 Chef 或 Puppet 管理的虚拟集群里。这种部署方案不利于横向扩展，比如要部署多台物理服务器，需要都安装相同的依赖，再部署，很是麻烦。

[vagrant](https://www.vagrantup.com/) 这类管理多个虚拟机的工具，虽然使项目的部署更为遍历，但每个虚拟机都运行有一个完整的操作系统，十分耗费宿主主机的资源，并不适合微服务的开发和部署。

### 容器

#### 特性

[容器](https://en.wikipedia.org/wiki/Operating-system-level_virtualization) 是精简版的操作系统，但并不运行一个 kernel 或系统底层相关的驱动，它只包含一些 run-time 必需的库，多个容器共享宿主主机的 kernel，多个容器之间相互隔离，互补影响。可参考：[Redhat topic](https://www.redhat.com/en/topics/containers/whats-a-linux-container)

#### 优势

容器的运行环境只包含代码所需要的依赖，而不是使用完整的操作系统包含一大堆不需要的组件。此外，容器本身的体积相比虚拟机是比较小的，比如对比 ubuntu 16.04 优势不言而喻：

- 虚拟机大小

  ![image-20180512154621284](http://p7f8yck57.bkt.clouddn.com/2018-05-12-074621.png)

- 容器镜像大小

  ![image-20180512154904850](http://p7f8yck57.bkt.clouddn.com/2018-05-12-074905.png)



#### Docker 与容器

一般人会认为容器技术就是 Docker，实则不然，Docker 只是容器技术的一种实现，因为其操作简便且学习门槛低，所以如此流行。



### Docker 化微服务

#### Dockerfile

创建微服务部署的 Dockerfile

```dockerfile
# 若运行环境是 Linux 则需把 alpine 换成 debian
# 使用最新版 alpine 作为基础镜像
FROM alpine:latest

# 在容器的根目录下创建 app 目录
RUN mkdir /app

# 将工作目录切换到 /app 下
WORKDIR /app

# 将微服务的服务端运行文件拷贝到 /app 下
ADD consignment-service /app/consignment-service

# 运行服务端
CMD ["./consignment-service"]
```

alpine 是一个超轻量级 Linux 发行版本，专为 Docker 中 Web 应用而生。它能保证绝大多数 web 应用可以正常运行，即使它只包含必要的 run-time 文件和依赖，镜像大小只有 4 MB，相比上边 Ubuntu16.4 节约了 99.7% 的空间： 

![image-20180512162131600](http://p7f8yck57.bkt.clouddn.com/2018-05-12-082131.png)

由于 docker 镜像的超轻量级，在上边部署和运行微服务耗费的资源是很小的。

#### 编译项目

为了在 alpine 上运行我们的微服务，向 Makefile 追加命令：

```makefile
build:
	...
	# 告知 Go 编译器生成二进制文件的目标环境：amd64 CPU 的 Linux 系统
	GOOS=linux GOARCH=amd64 go build	
	# 根据当前目录下的 Dockerfile 生成名为 consignment-service 的镜像
	docker build -t consignment-service .
```

需手动指定 `GOOS` 和 `GOARCH` 的值，否则在 macOS 上编译出的文件是无法在 alpine 容器中运行的。

其中 `docker build` 将程序的执行文件 consignment-service 及其所需的 run-time 环境打包成了一个镜像，以后在 docker 中直接 `run` 镜像即可启动该微服务。

你可以把你的镜像分享到 [DockerHub](https://hub.docker.com/explore/)，二者的关系类比 npm 与 nodejs、composer 与 PHP，去 DockerHub 瞧一瞧，会发现很多优秀的开源软件都已 Docker 化，参考演讲：[Willy Wonka of Containers](https://www.youtube.com/watch?v=GsLZz8cZCzc)

关于 Docker 构建镜像的细节，请参考书籍《第一本 Docker 书》第四章

#### 运行 Docker 化后的微服务

继续在 Makefile 中追加命令：

```makefile
build:
	...
run:
	# 在 Docker alpine 容器的 50001 端口上运行 consignment-service 服务
	# 可添加 -d 参数将微服务放到后台运行
	docker run -p 50051:50051 consignment-service
```

由于 Docker 有自己独立的网络层，所以需要指定将容器的端口映射到本机的那个端口，使用 `-p` 参数即可指定，比如 `-p 8080:50051` 是将容器 50051端口映射到本机 8080 端口，注意顺序是反的。更多参考：[Docker 文档](https://docs.docker.com/network/)

现在运行 `make build && make run ` 即可在 docker 中运行我们的微服务，此时在本机执行微服务的客户端代码，将成功调用 docker 中的微服务：

![dockerd](http://p7f8yck57.bkt.clouddn.com/2018-05-12-092732.gif)



## Go-micro

### 为什么不继续使用 gRPC ?

#### 管理麻烦

在客户端代码（consignment-cli/cli.go）中，我们手动指定了服务端的地址和端口，在本地修改不是很麻烦。但在生产环境中，各服务可能不在同一台主机上（分布式独立运行），其中任一服务重新部署后 IP 或运行的端口发生变化，其他服务将无法再调用它。如果你有很多个服务，彼此指定 IP 和端口来相互调用，那管理起来很麻烦

#### 服务发现

为解决服务间的调用问题，服务发现（service discovery）出现了，它作为一个注册中心会记录每个微服务的 IP 和端口，各微服务上线时会在它那注册，下线时会注销，其他服务可通过名字或 ID 找到该服务类比门面模式。

为不重复造轮子，我们直接使用实现了服务注册的 go-micro 框架。



### 安装

```shell
go get -u github.com/micro/protobuf/proto
go get -u github.com/micro/protobuf/protoc-gen-go
```

使用 go-micro 自己的编译器插件，在 Makefile 中修改 protoc 命令：

```makefile
build:
	# 不再使用 grpc 插件
	protoc -I. --go_out=plugins=micro:$(GOPATH)/src/shippy/consignment-service proto/consignment/consignment.proto
```



### 服务端使用 go-micro

你会发现重新生成的 consignment.pb.go 大有不同。修改服务端代码 main.go 使用 go-micro

```go
package main

import (
	pb "shippy/consignment-service/proto/consignment"
	"context"
	"log"
	"github.com/micro/go-micro"
)

//
// 仓库接口
//
type IRepository interface {
	Create(consignment *pb.Consignment) (*pb.Consignment, error) // 存放新货物
	GetAll() []*pb.Consignment                                   // 获取仓库中所有的货物
}

//
// 我们存放多批货物的仓库，实现了 IRepository 接口
//
type Repository struct {
	consignments []*pb.Consignment
}

func (repo *Repository) Create(consignment *pb.Consignment) (*pb.Consignment, error) {
	repo.consignments = append(repo.consignments, consignment)
	return consignment, nil
}

func (repo *Repository) GetAll() []*pb.Consignment {
	return repo.consignments
}

//
// 定义微服务
//
type service struct {
	repo Repository
}

//
// 实现 consignment.pb.go 中的 ShippingServiceHandler 接口
// 使 service 作为 gRPC 的服务端
//
// 托运新的货物
// func (s *service) CreateConsignment(ctx context.Context, req *pb.Consignment) (*pb.Response, error) {
func (s *service) CreateConsignment(ctx context.Context, req *pb.Consignment, resp *pb.Response) error {
	// 接收承运的货物
	consignment, err := s.repo.Create(req)
	if err != nil {
		return err
	}
	resp = &pb.Response{Created: true, Consignment: consignment}
	return nil
}

// 获取目前所有托运的货物
// func (s *service) GetConsignments(ctx context.Context, req *pb.GetRequest) (*pb.Response, error) {
func (s *service) GetConsignments(ctx context.Context, req *pb.GetRequest, resp *pb.Response) error {
	allConsignments := s.repo.GetAll()
	resp = &pb.Response{Consignments: allConsignments}
	return nil
}

func main() {
	server := micro.NewService(
		// 必须和 consignment.proto 中的 package 一致
		micro.Name("go.micro.srv.consignment"),
		micro.Version("latest"),
	)

	// 解析命令行参数
	server.Init()
	repo := Repository{}
	pb.RegisterShippingServiceHandler(server.Server(), &service{repo})

	if err := server.Run(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```

go-micro 的实现相比 gRPC 有 3 个主要的变化：

#### 创建 RPC 服务器的流程 

`micro.NewService(...Option)` 简化了微服务的注册流程， `micro.Run()` 也简化了 `gRPCServer.Serve()`，不再需要手动创建 TCP 连接并监听。

#### 微服务的 interface

注意看代码中第 47、59 行，会发现 go-micro 将响应参数 Response 提到了入参，只返回 error，整合了 gRPC 的 [四种运行模式]()

#### 运行地址的管理

服务的监听端口没有在代码中写死，go-mirco 会自动使用系统或命令行中变量 `MICRO_SERVER_ADDRESS` 的地址

对应更新一下 Makefile

```makefile
run:
	docker run -p 50051:50051 \
	 -e MICRO_SERVER_ADDRESS=:50051 \
	 -e MICRO_REGISTRY=mdns \
	 consignment-service
```

-e 选项用于设置镜像中的环境变量，其中 `MICRO_REGISTRY=mdns` 会使 go-micro 在本地使用 [mdns](https://zh.wikipedia.org/wiki/%E5%A4%9A%E6%92%AD) 多播作为服务发现的中间层。在生产环境一般会使用 [Consul](https://www.consul.io/) 或 [Etcd](https://github.com/coreos/etcd) 代替 mdns 做服务发现，在本地开发先一切从简。

现在执行 `make build && make run`，你的 consignment-service 就有服务发现的功能了。



### 客户端使用 go-micro

我们需要更新一下客户端的代码，使用 go-micro 来调用微服务：

```go
func main() {
	cmd.Init()
	// 创建微服务的客户端，简化了手动 Dial 连接服务端的步骤
	client := pb.NewShippingServiceClient("go.micro.srv.consignment", microclient.DefaultClient)
    ...
}
```

现在运行 `go run cli.go` 会报错：

![image-20180513095911084](http://p7f8yck57.bkt.clouddn.com/2018-05-13-015911.png)

因为服务端运行在 Docker 中，而 Docker 有自己独立的 mdns，与宿主主机 Mac 的 mdns 不一致。把客户端也 Docker 化，这样服务端与客户端就在同一个网络层下，顺利使用 mdns 做服务发现。

#### Docker 化客户端

创建客户端的 Dokerfile

```dockerfile
FROM alpine:latest
RUN mkdir -p /app
WORKDIR /app

# 将当前目录下的货物信息文件 consignment.json 拷贝到 /app 目录下
ADD consignment.json /app/consignment.json
ADD consignment-cli /app/consignment-cli

CMD ["./consignment-cli"]
```

创建文件 `consignment-cli/Makefile`

```makefile
build:
	GOOS=linux GOARCH=amd64 go build
	docker build -t consignment-cli .
run:
	docker run -e MICRO_REGISTRY=mdns consignment-cli
```

#### 调用微服务

执行 `make build && make run`









