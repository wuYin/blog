---
title: Golang 微服务教程（三）
date: 2018-05-22 21:39:34
tags: 微服务
---

原文链接：[ewanvalentine.io](https://ewanvalentine.io/)，翻译已获作者 [Ewan Valentine ](https://twitter.com/Ewan_Valentine) 授权。

<!-- more -->

在上节中，我们使用 go-micro 重新实现了微服务并进行了 Docker 化，但是每个微服务都要单独维护自己的 Makefile 未免过于繁琐。本节将学习 docker-compose 来统一管理和部署微服务，并引入第三个微服务 user-service



## 数据库

到目前为止，consignment-cli 要托运的货物数据直接存储在 consignment-service 管理的内存中，当服务重启时这些数据将会丢失。为了便于管理和搜索货物信息，需将其存储到数据库中。

可以为每个独立运行的微服务提供独立的数据库，不过因为管理繁琐少有人这么做。如何为不同的微服务选择合适的数据库，可参考：[How to choose a database for your microservices](https://www.infoworld.com/article/3236291/database/how-to-choose-a-database-for-your-microservices.html)

如果对存储数据的可靠性、一致性要求不那么高，那 NoSQL 将是很好的选择，因为它能存储的数据格式十分灵活，比如常常将数据存为 JSON 进行处理。在本节中选用性能和生态俱佳的MongoDB

如果要存储的数据本身就比较完整，数据之间关系也有较强关联性的话，可以选用关系型数据库。可以事先捋一下要存储数据的结构，根据业务看一下是读更多还是写更多？高频查询的复杂度如何？…，鉴于本文的较小的数据量与操作，作者选用了 Postgres，读者可自行更换为 MySQL 等。

更多参考：[如何选择NoSQL数据库](https://github.com/ruanyf/articles/blob/master/dev/database/index.md)



## docker-compose

上节把微服务 Docker 化后，使其运行在轻量级、服务运行必需依赖的容器中。到目前为止，要想启动微服务的容器，均在其 Makefile 中 `docker run …`，这样服务增多了以后管理十分麻烦。

[docker-compose](http://dockone.io/article/834) 工具能直接用一个 `docker-compose.yaml` 来管理多个容器，同时设置各容器的 metadata 和 run-time 环境（设置环境变量），文件的 `service` 配置项来像先前 `docker run` 命令一样来启动容器。举个例子：

docker 命令管理容器

```shell
$ docker run -p 50052:50051 \
  -e MICRO_SERVER_ADDRESS=:50051 \
  -e MICRO_REGISTRY=mdns \
  vessel-service
```

等效于 docker-compose 来管理

```yaml
version: '3.1'
vessel-service:
  build: ./vessel-service
  ports:
    - 50052:50051
  environment:
    MICRO_ADRESS: ":50051"
    MICRO_REGISTRY: "mdns"
```

想加减、配置微服务，直接修改 docker-compose.yaml，是十分方便的。



针对当前项目，使用 docker-compose 管理 3 个容器，在项目根目录下新建文件：

```yaml
# docker-compose.yaml
# 同样遵循严格的缩进
version: '3.1'

# services 定义容器列表
services:
   consignment-cli:
    build: ./consignment-cli
    environment:
      MICRO_REGISTRY: "mdns"

  consignment-service:
    build: ./consignment-service
    ports:
      - 50051:50051
    environment:
      MICRO_ADRESS: ":50051"
      MICRO_REGISTRY: "mdns"
      DB_HOST: "datastore:27017"

  vessel-service:
    build: ./vessel-service
    ports:
      - 50052:50051
    environment:
      MICRO_ADRESS: ":50051"
      MICRO_REGISTRY: "mdns"
```

首先，我们指定了要使用的 docker-compose 的版本是 3.1，然后使用 services 来列出了三个待管理的容器。

每个微服务都定义了自己容器的名字， `build` 指定目录下的 Dockerfile 将会用来编译镜像，也可以直接使用 `image` 选项直接指向已编译好的镜像（后边会用到）；其他选项则指定了容器的端口映射规则、环境变量等。

可使用 `docker-compose build` 来编译生成三个对应的镜像；使用 `docker-compose run` 来运行指定的容器， `docker-compose up -d` 可在后台运行；使用 `docker stop $(docker ps -aq )` 来停止所有正在运行的容器。

使用 docker-compose 的运行效果如下：

![3.1](http://p7f8yck57.bkt.clouddn.com/2018-05-24-115021.gif)





## Entities & Protobufs

 到目前为止，我们的 protobuf 协议文件，定义了微服务客户端与服务端数据请求、响应的数据结构。由于 protobuf 的规范性，也可将其生成的 struct 作为数据库表 Model 进行数据操作。这种复用有其局限性。比如 protobuf 中数据类型与数据库的不一致，二者高耦合，很多人并不赞将 protobuf 数据结构作为数据库中的表结构：[Do you use Protobufs in place of structs ?](https://www.reddit.com/r/golang/comments/77yd72/question_do_you_use_protobufs_in_place_of_structs/)

一般来说，在 protobuf 和数据库实体之间会有一层做转换。比如：

```go
func (service *Service) (ctx context.Context, req *proto.User, res *proto.Response) error {
  entity := &models.User{
    Name: req.Name.
    Email: req.Email,
    Password: req.Password, 
  }
  err := service.repo.Create(entity)
    
  // 无中间转换层
  // err := service.repo.Create(req)
  ... 
}
```

这样隔离数据库实体 models 和 proto.* 结构体，似乎很方便。但当 .proto 中定义 message 各种嵌套时，models 也要对应嵌套，比较麻烦。

上边隔不隔离由读者自行决定，就我个人而言，中间用 models 做转换是不太有必要的，protobuf 已足够规范，直接使用即可。



## 代码结构

回头看第一个微服务 consignment-service，会发现服务端实现、接口实现等都往 main.go 里边塞了，功能跑通了，现在要拆分代码，使项目结构更加清晰，更易维护。

在 consignmet-service/ 下创建三个文件：handler.go、datastore.go 和 repository.go

```
.
├── Dockerfile
├── Makefile
├── datastore.go	# 创建与 MongoDB 的会话
├── handler.go		# 实现微服务的服务端：使用现成的 CURD 来处理请求及业务逻辑
├── main.go			# 注册并启动服务
├── proto
└── repository.go	# 实现数据库的基本 CURD 操作
```

对于熟悉 MVC 开发模式的同学来说，可能会把代码按功能拆分到不同目录中，比如：

```
main.go
models/
  user.go
handlers/
  auth.go 
  user.go
services/
  auth.go 
```

不过这种组织方式并不是 Golang 的风格，因为微服务是切割出来独立的。对于大型 Golang 项目，应该如下组织：

```
main.go
users/
  services/
    auth.go
  handlers/
    auth.go
    user.go
  users/
    user.go
containers/
  services/
    manage.go
  models/
    container.go
```

这种组织方式叫类别（domain）驱动，而不是 MVC 的功能驱动

由于微服务的简洁性，我们会把该服务相关的代码全放到一个文件夹下，同时为每个文件起一个合适的名字。

#### datastore.go

```go
package main

import "gopkg.in/mgo.v2"

// 创建与 MongoDB 交互的主回话
func CreateSession(host string) (*mgo.Session, error) {
	s, err := mgo.Dial(host)
	if err != nil {
		return nil, err
	}
	s.SetMode(mgo.Monotonic, true)
	return s, nil
}
```

#### repository.go

```go
package main

import (
	pb "shippy/consignment-service/proto/consignment"
	"gopkg.in/mgo.v2"
)

const (
	DB_NAME        = "shippy"
	CON_COLLECTION = "consignments"
)

type Repository interface {
	Create(*pb.Consignment) error
	GetAll() ([]*pb.Consignment, error)
	Close()
}

type ConsignmentRepository struct {
	session *mgo.Session
}

// 接口实现
func (repo *ConsignmentRepository) Create(c *pb.Consignment) error {
	return repo.collection().Insert(c)
}

// 获取全部数据
func (repo *ConsignmentRepository) GetAll() ([]*pb.Consignment, error) {
	var cons []*pb.Consignment
	err := repo.collection().Find(nil).All(&cons)
	return cons, err
}

// 关闭连接
func (repo *ConsignmentRepository) Close() {
	repo.session.Close()
}

// 返回所有货物信息
func (repo *ConsignmentRepository) collection() *mgo.Collection {
	return repo.session.DB(DB_NAME).C(CON_COLLECTION)
}
```



#### handler.go

```go
package main

import (
	pb "shippy/consignment-service/proto/consignment"
	vesselPb "shippy/vessel-service/proto/vessel"
	"context"
	"gopkg.in/mgo.v2"
	"log"
)

type handler struct {
	session *mgo.Session
	vesselClient vesselPb.VesselServiceClient
}

// 为什么不直接用 session
func (h *handler)GetRepo()Repository  {
	return &ConsignmentRepository{h.session.Clone()}
}

func (h *handler)CreateConsignment(ctx context.Context, req *pb.Consignment, resp *pb.Response) error {
	defer h.GetRepo().Close()

	// 检查是否有适合的货轮
	vReq := &vesselPb.Specification{
		Capacity:  int32(len(req.Containers)),
		MaxWeight: req.Weight,
	}
	vResp, err := h.vesselClient.FindAvailable(context.Background(), vReq)
	if err != nil {
		return err
	}

	// 货物被承运
	log.Printf("found vessel: %s\n", vResp.Vessel.Name)
	req.VesselId = vResp.Vessel.Id
	//consignment, err := h.repo.Create(req)
	err = h.GetRepo().Create(req)
	if err != nil {
		return err
	}
	resp.Created = true
	resp.Consignment = req
	return nil
}

func (h *handler)GetConsignments(ctx context.Context, req *pb.GetRequest, resp *pb.Response) error {
	defer h.GetRepo().Close()
	consignments, err := h.GetRepo().GetAll()
	if err != nil {
		return err
	}
	resp.Consignments = consignments
	return nil
}
```



#### main.go

```go
package main

import (
	pb "shippy/consignment-service/proto/consignment"
	vesselPb "shippy/vessel-service/proto/vessel"
	"log"
	"github.com/micro/go-micro"
	"os"
)

const (
	DEFAULT_HOST = "localhost:27017"
)


func main() {

	dbHost := os.Getenv("DB_HOST")
	if dbHost == ""{
		 dbHost = DEFAULT_HOST
	}
	session, err := CreateSession(dbHost)
	defer session.Close()
	if err != nil {
		log.Fatalf("create session error: %v\n", err)
	}

	server := micro.NewService(
		// 必须和 consignment.proto 中的 package 一致
		micro.Name("go.micro.srv.consignment"),
		micro.Version("latest"),
	)

	// 解析命令行参数
	server.Init()
	// 作为 vessel-service 的客户端
	vClient := vesselPb.NewVesselServiceClient("go.micro.srv.vessel", server.Client())
	pb.RegisterShippingServiceHandler(server.Server(), &handler{session, vClient})

	if err := server.Run(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```