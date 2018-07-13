---
title: Golang 微服务教程（三）
date: 2018-05-22 21:39:34
tags: 微服务
---

原文链接：[ewanvalentine.io](https://ewanvalentine.io/microservices-in-golang-part-3)，翻译已获作者 [Ewan Valentine ](https://twitter.com/Ewan_Valentine) 授权。

本文完整代码：[GitHub](https://github.com/wuYin/shippy/tree/feature/part3)

<!-- more -->

在上节中，我们使用 go-micro 重新实现了微服务并进行了 Docker 化，但是每个微服务都要单独维护自己的 Makefile 未免过于繁琐。本节将学习 docker-compose 来统一管理和部署微服务，引入第三个微服务 user-service 并进行存储数据。



## MongoDB 与 Postgres

### 微服务的数据存储

到目前为止，consignment-cli 要托运的货物数据直接存储在 consignment-service 管理的内存中，当服务重启时这些数据将会丢失。为了便于管理和搜索货物信息，需将其存储到数据库中。

可以为每个独立运行的微服务提供独立的数据库，不过因为管理繁琐少有人这么做。如何为不同的微服务选择合适的数据库，可参考：[How to choose a database for your microservices](https://www.infoworld.com/article/3236291/database/how-to-choose-a-database-for-your-microservices.html)

### 选择关系型数据库与 NoSQL

如果对存储数据的可靠性、一致性要求不那么高，那 NoSQL 将是很好的选择，因为它能存储的数据格式十分灵活，比如常常将数据存为 JSON 进行处理，在本节中选用性能和生态俱佳的MongoDB

如果要存储的数据本身就比较完整，数据之间关系也有较强关联性的话，可以选用关系型数据库。事先捋一下要存储数据的结构，根据业务看一下是读更多还是写更多？高频查询的复不复杂？… 鉴于本文的较小的数据量与操作，作者选用了 Postgres，读者可自行更换为 MySQL 等。

更多参考：[如何选择NoSQL数据库](https://github.com/ruanyf/articles/blob/master/dev/database/index.md)、[梳理关系型数据库和NoSQL的使用情景](http://dbaplus.cn/news-21-269-1.html)



## docker-compose

### 引入原因

上节把微服务 Docker 化后，使其运行在轻量级、只包含服务必需依赖的容器中。到目前为止，要想启动微服务的容器，均在其 Makefile 中 `docker run` 的同时设置其环境变量，服务多了以后管理起来十分麻烦。

### 基本使用

[docker-compose](http://dockone.io/article/834) 工具能直接用一个 `docker-compose.yaml` 来编排管理多个容器，同时设置各容器的 metadata 和 run-time 环境（环境变量），文件的 `service` 配置项来像先前 `docker run` 命令一样来启动容器。举个例子：

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

想加减和配置微服务，直接修改 docker-compose.yaml，是十分方便的。

更多参考：[使用 docker-compose 编排容器](http://www.dockerinfo.net/4257.html)

### 编排当前项目的容器

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

首先，我们指定了要使用的 docker-compose 的版本是 3.1，然后使用 `services` 来列出了三个待管理的容器。

每个微服务都定义了自己容器的名字， `build` 指定目录下的 Dockerfile 将会用来编译镜像，也可以直接使用 `image` 选项直接指向已编译好的镜像（后边会用到）；其他选项则指定了容器的端口映射规则、环境变量等。

可使用 `docker-compose build` 来编译生成三个对应的镜像；使用 `docker-compose run` 来运行指定的容器， `docker-compose up -d` 可在后台运行；使用 `docker stop $(docker ps -aq )` 来停止所有正在运行的容器。

### 运行效果

使用 docker-compose 的运行效果如下：

![3.1](http://p7f8yck57.bkt.clouddn.com/2018-05-24-115021.gif)





## Protobuf 与数据库操作

### 复用及其局限性

到目前为止，我们的两个 protobuf 协议文件，定义了微服务客户端与服务端数据请求、响应的数据结构。由于 protobuf 的规范性，也可将其生成的 struct 作为数据库表 Model 进行数据操作。这种复用有其局限性，比如 protobuf 中数据类型必须与数据库表字段严格一致，二者是高耦合的。很多人并不赞将 protobuf 数据结构作为数据库中的表结构：[Do you use Protobufs in place of structs ?](https://www.reddit.com/r/golang/comments/77yd72/question_do_you_use_protobufs_in_place_of_structs/)

### 中间层逻辑转换

一般来说，在表结构变化后与 protobuf 不一致，需要在二者之间做一层逻辑转换，处理差异字段：

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



## consignment-service 重构

回头看第一个微服务 consignment-service，会发现服务端实现、接口实现等都往 main.go 里边塞，功能跑通了，现在要拆分代码，使项目结构更加清晰，更易维护。

### MVC 代码结构

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

### 微服务代码结构

不过这种组织方式并不是 Golang 的 style，因为微服务是切割出来独立的，要做到简洁明了。对于大型 Golang 项目，应该如下组织：

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

这种组织方式叫领域（domain）驱动，而不是 MVC 的功能驱动。

### consignment-service 的重构

由于微服务的简洁性，我们会把该服务相关的代码全放到一个文件夹下，同时为每个文件起一个合适的名字。

在 consignmet-service/ 下创建三个文件：handler.go、datastore.go 和 repository.go

```
consignmet-service/ 
    ├── Dockerfile
    ├── Makefile
    ├── datastore.go	# 创建与 MongoDB 的主会话
    ├── handler.go		# 实现微服务的服务端，处理业务逻辑
    ├── main.go			# 注册并启动服务
    ├── proto
    └── repository.go	# 实现数据库的基本 CURD 操作
```



#### 负责连接 MongoDB 的 datastore.go

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

连接 MongoDB 的代码够精简，传参是数据库地址，返回数据库会话以及可能发生的错误，在微服务启动的时候就会去连接数据库。



#### 负责与 MongoDB 交互的 repository.go

现在让我们来将 main.go 与数据库交互的代码拆解出来，可以参考注释加以理解：

```go
package main
import (...)

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
	// Find() 一般用来执行查询，如果想执行 select * 则直接传入 nil 即可
	// 通过 .All() 将查询结果绑定到 cons 变量上
	// 对应的 .One() 则只取第一行记录
	err := repo.collection().Find(nil).All(&cons)
	return cons, err
}

// 关闭连接
func (repo *ConsignmentRepository) Close() {
	// Close() 会在每次查询结束的时候关闭会话
	// Mgo 会在启动的时候生成一个 "主" 会话
    // 你可以使用 Copy() 直接从主会话复制出新会话来执行，即每个查询都会有自己的数据库会话
	// 同时每个会话都有自己连接到数据库的 socket 及错误处理，这么做既安全又高效
	// 如果只使用一个连接到数据库的主 socket 来执行查询，那很多请求处理都会阻塞
	// Mgo 因此能在不使用锁的情况下完美处理并发请求
	// 不过弊端就是，每次查询结束之后，必须确保数据库会话要手动 Close
	// 否则将建立过多无用的连接，白白浪费数据库资源
	repo.session.Close()
}

// 返回所有货物信息
func (repo *ConsignmentRepository) collection() *mgo.Collection {
	return repo.session.DB(DB_NAME).C(CON_COLLECTION)
}
```



#### 拆分后的 main.go

```go
package main
import (...)

const (
	DEFAULT_HOST = "localhost:27017"
)

func main() {

	// 获取容器设置的数据库地址环境变量的值
	dbHost := os.Getenv("DB_HOST")
	if dbHost == ""{
		 dbHost = DEFAULT_HOST
	}
	session, err := CreateSession(dbHost)
	// 创建于 MongoDB 的主会话，需在退出 main() 时候手动释放连接
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
	// 将 server 作为微服务的服务端
	pb.RegisterShippingServiceHandler(server.Server(), &handler{session, vClient})

	if err := server.Run(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```



#### 实现服务端的 handler.go

将 main.go 中实现微服务服务端 interface 的代码单独拆解到 handler.go，实现业务逻辑的处理。

```go
package main
import (...)

// 微服务服务端 struct handler 必须实现 protobuf 中定义的 rpc 方法
// 实现方法的传参等可参考生成的 consignment.pb.go
type handler struct {
	session *mgo.Session
	vesselClient vesselPb.VesselServiceClient
}

// 从主会话中 Clone() 出新会话处理查询
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

至此，main.go 拆分完毕，代码文件分工明确，十分清爽。



### mgo 库的 Copy() 与 Clone()

在 handler.go 的 GetRepo() 中我们使用 Clone() 来创建新的数据库连接。

可看到在 main.go 中创建主会话后我们就再也没用到它，反而使用 `session.Clonse()` 来创建新的会话进行查询处理，可以看 repository.go 中 `Close()` 的注释，如果每次查询都用主会话，那所有请求都是同一个底层 socket 执行查询，后边的请求将会阻塞，不能发挥 Go 天生支持并发的优势。

为了避免请求的阻塞，mgo 库提供了 `Copy()` 和 `Clone()` 函数来创建新会话，二者在功能上相差无几，但在细微之处却有重要的区别。Clone 出来的新会话重用了主会话的 socket，避免了创建 socket 在三次握手时间、资源上的开销，尤其适合那些快速写入的请求。如果进行了复杂查询、大数据量操作时依旧会阻塞 socket 导致后边的请求阻塞。Copy 为会话创建新的 socket，开销大。

应当根据应用场景不同来选择二者，本文的查询既不复杂数据量也不大，就直接复用主会话的 socket 即可。不过用完都要 Close()，谨记。





## vessel-service 重构 

拆解完 consignment-service/main.go 的代码，现在用同样的方式重构 vessel-service

### 新增货轮

我们在此添加一个方法：添加新的货轮，更改 protobuf 文件如下：

```protobuf
syntax = "proto3";
package go.micro.srv.vessel;

service VesselService {
    // 检查是否有能运送货物的轮船
    rpc FindAvailable (Specification) returns (Response) {}
    // 创建货轮
    rpc Create(Vessel) returns (Response){}
}

// ...

// 货轮装得下的话
// 返回的多条货轮信息
message Response {
    Vessel vessel = 1;
    repeated Vessel vessels = 2;
    bool created = 3;
}
```

我们创建了一个 `Create()` 方法来创建新的货轮，参数是 Vessel 返回 Response，注意 Response 中添加了 created 字段，标识是否创建成功。使用 `make build` 生成新的 vessel.pb.go 文件。

### 拆分数据库操作与业务逻辑处理

之后在对应的 repository.go 和 handler.go 中实现 `Create()`

```go
// vesell-service/repository.go
// 完成与数据库交互的创建动作
func (repo *VesselRepository) Create(v *pb.Vessel) error {
	return repo.collection().Insert(v)
}
```

```go
// vesell-service/handler.go
func (h *handler) GetRepo() Repository {
	return &VesselRepository{h.session.Clone()}
}

// 实现微服务的服务端
func (h *handler) Create(ctx context.Context, req *pb.Vessel, resp *pb.Response) error {
	defer h.GetRepo().Close()
	if err := h.GetRepo().Create(req); err != nil {
		return err
	}
	resp.Vessel = req
	resp.Created = true
	return nil
}
```



### 引入 MongoDB

两个微服务均已重构完毕，是时候在容器中引入 MongoDB 了。在 docker-compose.yaml 添加 datastore 选项：

```yaml
services:
  ...
  datastore:
    image: mongo
    ports:
      - 27017:27017
```

同时更新两个微服务的环境变量，增加 `DB_HOST: "datastore:27017"`，在这里我们使用 datastore 做主机名而不是 localhost，是因为 docker 有内置强大的 DNS 机制。参考：[docker内置dnsserver工作机制](http://dockone.io/article/2316)

修改完毕后的 docker-compose.yaml：

```yaml
# docker-compose.yaml
version: '3.1'

services:
  consigment-cli:
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
      DB_HOST: "datastore:27017"

  datastore:
    image: mongo
    ports:
      - 27017:27017
```

修改完代码需重新 `make build`，构建镜像时需 `docker-compose build --no-cache` 来全部重新编译。



## user-service

### 引入 Postgres

现在来创建第三个微服务，在 `docker-compose.yaml` 中引入 Postgres：

```yaml
...
  user-service:
    build: ./user-service
    ports:
      - 50053:50051
    environment:
      MICRO_ADDRESS: ":50051"
      MICRO_REGISTRY: "mdns"

  ...
  database:
    image: postgres
    ports:
      - 5432:5432
```

在项目根目录下创建 user-service 目录，并且像前两个服务那样依次创建下列文件：

```
handler.go, main.go, repository.go, database.go, Dockerfile, Makefile,
```

### 定义 protobuf 文件

创建 proto/user/user.proto 且内容如下：

```protobuf
// user-service/user/user.proto
syntax = "proto3";

package go.micro.srv.user;

service UserService {
    rpc Create (User) returns (Response) {}
    rpc Get (User) returns (Response) {}
    rpc GetAll (Request) returns (Response) {}
    rpc Auth (User) returns (Token) {}
    rpc ValidateToken (Token) returns (Token) {}
}

// 用户信息
message User {
    string id = 1;
    string name = 2;
    string company = 3;
    string email = 4;
    string password = 5;
}

message Request {
}

message Response {
    User user = 1;
    repeated User users = 2;
    repeated Error errors = 3;
}

message Token {
    string token = 1;
    bool valid = 2;
    Error errors = 3;
}

message Error {
    int32 code = 1;
    string description = 2;
}
```

确保你的 user-service 有像类似前两个微服务的 Makefile，使用 `make build` 来生成 gRPC 代码。

### 实现业务逻辑处理的 handler.go

在 handler.go 实现的服务端代码中，认证模块将在下一节使用 JWT 做认证。

```go
// user-service/handler.go

package main

import (
	"context"
	pb "shippy/user-service/proto/user"
)

type handler struct {
	repo Repository
}

func (h *handler) Create(ctx context.Context, req *pb.User, resp *pb.Response) error {
	if err := h.repo.Create(req); err != nil {
		return nil
	}
	resp.User = req
	return nil
}

func (h *handler) Get(ctx context.Context, req *pb.User, resp *pb.Response) error {
	u, err := h.repo.Get(req.Id);
	if err != nil {
		return err
	}
	resp.User = u
	return nil
}

func (h *handler) GetAll(ctx context.Context, req *pb.Request, resp *pb.Response) error {
	users, err := h.repo.GetAll()
	if err != nil {
		return err
	}
	resp.Users = users
	return nil
}

func (h *handler) Auth(ctx context.Context, req *pb.User, resp *pb.Token) error {
	_, err := h.repo.GetByEmailAndPassword(req)
	if err != nil {
		return err
	}
	resp.Token = "`x_2nam"
	return nil
}

func (h *handler) ValidateToken(ctx context.Context, req *pb.Token, resp *pb.Token) error {
	return nil
}
```



### 实现数据库交互的 repository.go

```go
package main

import (
	"github.com/jinzhu/gorm"
	pb "shippy/user-service/proto/user"
)

type Repository interface {
	Get(id string) (*pb.User, error)
	GetAll() ([]*pb.User, error)
	Create(*pb.User) error
	GetByEmailAndPassword(*pb.User) (*pb.User, error)
}

type UserRepository struct {
	db *gorm.DB
}

func (repo *UserRepository) Get(id string) (*pb.User, error) {
	var u *pb.User
	u.Id = id
	if err := repo.db.First(&u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

func (repo *UserRepository) GetAll() ([]*pb.User, error) {
	var users []*pb.User
	if err := repo.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (repo *UserRepository) Create(u *pb.User) error {
	if err := repo.db.Create(&u).Error; err != nil {
		return err
	}
	return nil
}

func (repo *UserRepository) GetByEmailAndPassword(u *pb.User) (*pb.User, error) {
	if err := repo.db.Find(&u).Error; err != nil {
		return nil, err
	}
	return u, nil
}
```

### 使用 UUID

我们将 ORM 创建的 UUID 字符串修改为一个整数，用来作为表的主键或 ID 是比较安全的。MongoDB 使用了类似的技术，但是 Postgres 需要我们使用第三方库手动来生成。在 `user-service/proto/user` 目录下创建 extension.go 文件：

```go
package go_micro_srv_user

import (
	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
	"github.com/labstack/gommon/log"
)

func (user *User) BeforeCreate(scope *gorm.Scope) error {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalf("created uuid error: %v\n", err)
	}
	return scope.SetColumn("Id", uuid.String())
}
```

函数 `BeforeCreate()` 指定了 GORM 库使用 uuid 作为 ID 列值。参考：[doc.gorm.io/callbacks](http://doc.gorm.io/callbacks.html)



## GORM

[Gorm](http://jinzhu.me/gorm/) 是一个简单易用轻量级的 ORM 框架，支持  Postgres, MySQL, Sqlite 等数据库。

到目前三个微服务涉及到的数据量小、操作也少，用原生 SQL 完全可以 hold 住，所以是不是要 ORM 取决于你自己。

### user-cli

类比 consignment-service 的测试，现在创建 user-cli 命令行应用来测试 user-service

在项目根目录下创建 user-cli 目录，并创建 cli.go 文件：

```go
package main

import (
	"log"
	"os"

	pb "shippy/user-service/proto/user"
	microclient "github.com/micro/go-micro/client"
	"github.com/micro/go-micro/cmd"
	"golang.org/x/net/context"
	"github.com/micro/cli"
	"github.com/micro/go-micro"
)


func main() {

	cmd.Init()

	// 创建 user-service 微服务的客户端
	client := pb.NewUserServiceClient("go.micro.srv.user", microclient.DefaultClient)

	// 设置命令行参数
	service := micro.NewService(
		micro.Flags(
			cli.StringFlag{
				Name:  "name",
				Usage: "You full name",
			},
			cli.StringFlag{
				Name:  "email",
				Usage: "Your email",
			},
			cli.StringFlag{
				Name:  "password",
				Usage: "Your password",
			},
			cli.StringFlag{
				Name: "company",
				Usage: "Your company",
			},
		),
	)

	service.Init(
		micro.Action(func(c *cli.Context) {
			name := c.String("name")
			email := c.String("email")
			password := c.String("password")
			company := c.String("company")

			r, err := client.Create(context.TODO(), &pb.User{
				Name: name,
				Email: email,
				Password: password,
				Company: company,
			})
			if err != nil {
				log.Fatalf("Could not create: %v", err)
			}
			log.Printf("Created: %v", r.User.Id)

			getAll, err := client.GetAll(context.Background(), &pb.Request{})
			if err != nil {
				log.Fatalf("Could not list users: %v", err)
			}
			for _, v := range getAll.Users {
				log.Println(v)
			}

			os.Exit(0)
		}),
	)

	// 启动客户端
	if err := service.Run(); err != nil {
		log.Println(err)
	}
}
```



### 测试

#### 运行成功

![3.3](http://p7f8yck57.bkt.clouddn.com/2018-05-26-123101.gif)

在此之前，需要手动拉取 Postgres 镜像并运行：

```shell
$ docker pull postgres
$ docker run --name postgres -e POSTGRES_PASSWORD=postgres -d -p 5432:5432 postgres
```

#### 用户数据创建并存储成功：

![image-20180526203001681](http://p7f8yck57.bkt.clouddn.com/2018-05-26-123002.png)



## 总结

到目前为止，我们创建了三个微服务：consignment-service、vessel-service 和 user-service，它们均使用 go-micro 实现并进行了 Docker 化，使用 docker-compose 进行统一管理。此外，我们还使用 GORM 库与 Postgres 数据库进行交互，并将命令行的数据存储进去。

上边的 user-cli 仅是测试使用，明文保存密码一点也不安全。在本节完成基本功能的基础上，下节将引入 JWT 做验证。



































