---
title: Golang 微服务教程（五）
date: 2018-05-28 18:21:13
tags: 微服务
---

原文链接：[ewanvalentine.io](https://ewanvalentine.io/microservices-in-golang-part-5)，翻译已获作者 [Ewan Valentine ](https://twitter.com/Ewan_Valentine)授权。

本文完整代码：[GitHub](https://github.com/wuYin/shippy/tree/feature/part5)

<!-- more -->

在上节中，我们使用 JWT 在微服务之间进行了用户的认证。在本节中，我们将使用 go-micro 结合 nats 插件来完成用户创建事件的发布与订阅。

正如前几节所说，go-micro 是一个拔插式的框架，能与很多优秀的开源软件进行对接，可参考插件列表：[go-plugins](https://github.com/micro/go-plugins)，可看到已支持很多优秀组件。



## 事件驱动

### 概念

[事件驱动架构](https://en.wikipedia.org/wiki/Event-driven_architecture) 理解起来比较简单，普遍认为好的软件架构都是解耦的，微服务之间不应该相互耦合或依赖。举个例子，我们在代码中调用微服务 `go.srv.user-service` 的函数，会先通过服务发现找到微服务的地址再调用，我们的代码与该微服务有了直接性的调用交互，并不算是完全的解耦。更多参考：[软件架构入门](http://www.ruanyifeng.com/blog/2016/09/software-architecture.html)

### 发布与订阅模式

为了理解事件驱动架构为何能使代码完全解耦，先了解事件的发布、订阅流程。微服务 X 完成任务 x 后通知消息系统说 “x 已完成”，它并不关心有哪些微服务正在监听这个事件、事件发生后会产生哪些影响。如果系统发生了某个事件，随之其他微服务都要做出动作是很容易的。

举个例子，user-service 创建了一个新用户，email-service 要给该用户发一封注册成功的邮件，message-service 要给网站管理员发一条用户注册的通知短信。

#### 一般实现

在 user-service 的代码中实例化另两个微服务 Client 后，调用函数发邮件和短信，代码耦合度很高。如下图：

![image-20180529201004910](http://p7f8yck57.bkt.clouddn.com/2018-05-29-121004.png)

#### 事件驱动

在事件驱动的架构下，user-service 只需向消息系统发布一条 topic 为 “user.created” 的消息，其他两个订阅了此 topic 的 service 能知道有用户注册了，拿到用户信息后他们自行发邮件、发短信。如下图：

![image-20180529200406346](http://p7f8yck57.bkt.clouddn.com/2018-05-29-120407.png)





本节中，我们将在 user-service 创建一个新用户时发布一个事件，使得 email-service 给用户发送邮件。

## 代码实现

### go-micro NATS 插件

我们先将 [NATS](https://nats.io/) 插件集成到我们的代码中：

```go
// user-service/main.go

func main() {
	...
	// 初始化命令行环境
	srv.Init()
    
	// 获取 broker 实例
	pubSub := srv.Server().Options().Broker
	
	// 注册 handler
	pb.RegisterUserServiceHandler(srv.Server(), &handler{repo, &t, pubSub})
	...
}
```

需要注意的是，当 go-micro 创建微服务时，`srv.Init()` 会加载该微服务的所有配置，比如使用到的插件、设置的环境变量、命令行参数等，这些配置项会作为微服务的一部分来运行。可使用 `s.Server().Options()` 来获取这些配置。

我们在 Makefile 中设置了 `GO_MICRO_BROKER` 环境变量，go-micro 会使用该地址指定的 NATS 消息系统做事件的订阅和发布。

### Publish 事件发布

当创建一个新用户时我们发布一个事件，完整代码见：[GitHub](https://github.com/EwanValentine/shippy-user-service/tree/tutorial-5)

```go
// user-service/handler.go

const topic = "user.created"

type handler struct {
	repo         Repository
	tokenService Authable
	PubSub       broker.Broker
}

func (h *handler) Create(ctx context.Context, req *pb.User, resp *pb.Response) error {
	// 哈希处理用户输入的密码
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	req.Password = string(hashedPwd)
	if err := h.repo.Create(req); err != nil {
		return nil
	}
	resp.User = req

	// 发布带有用户所有信息的消息
	if err := h.publishEvent(req); err != nil {
		return err
	}
	return nil
}

// 发送消息通知
func (h *handler) publishEvent(user *pb.User) error {
	body, err := json.Marshal(user)
	if err != nil {
		return err
	}

	msg := &broker.Message{
		Header: map[string]string{
			"id": user.Id,
		},
		Body: body,
	}

	// 发布 user.created topic 消息
	if err := h.PubSub.Publish(topic, msg); err != nil {
		log.Fatalf("[pub] failed: %v\n", err)
	}
	return nil
}

...
```

在运行前请确保你的 Postgres 容器正常运行：

```shell
$ docker run -d -p 5432:5432 postgres
$ make build
$ make run
```



### Subscribe 事件订阅

现在创建新的邮件服务：[email-service](https://github.com/EwanValentine/shippy-email-service)，创建新用户时将通知它发邮件。

```go
package main

import (
	userPb "shippy/user-service/proto/user"
	"github.com/micro/go-micro"
	"log"
	"github.com/micro/go-micro/broker"
	_ "github.com/micro/go-plugins/broker/nats"
	"encoding/json"
)

const topic = "user.created"

func main() {
	srv := micro.NewService(
		micro.Name("go.micro.srv.email"),
		micro.Version("latest"),
	)
	srv.Init()

	pubSub := srv.Server().Options().Broker
	if err := pubSub.Connect(); err != nil {
		log.Fatalf("broker connect error: %v\n", err)
	}

	// 订阅消息
	_, err := pubSub.Subscribe(topic, func(pub broker.Publication) error {
		var user *userPb.User
		if err := json.Unmarshal(pub.Message().Body, &user); err != nil {
			return err
		}
		log.Printf("[Create User]: %v\n", user)
		go senEmail(user)
		return nil
	})

	if err != nil {
		log.Printf("sub error: %v\n", err)
	}

	if err := srv.Run(); err != nil {
		log.Fatalf("srv run error: %v\n", err)
	}
}

func senEmail(user *userPb.User) error {
	log.Printf("[SENDING A EMAIL TO %s...]", user.Name)
	return nil
}
```

在运行邮件服务之前确保 NATS 已运行：

```shell
$ docker run -d -p 4222:4222 nats
```

现在把 user-service 和 email-service 都运行起来，使用 user-cli 创建新用户 Ewan，能看到给用户 Ewan 发了一封邮件：

![5.1](http://p7f8yck57.bkt.clouddn.com/2018-05-29-091321.gif)



### 切换消息代理插件

值得一提的是基于 JSON 的 NATS 性能消耗会比 gRPC 更高一点，需要额外处理  JSON 字符串，只在少数应用场景十分适合。go-micro 也支持很多已被广泛使用的消息队列 / 发布订阅技术，参考：[消息代理插件列表](https://github.com/micro/go-plugins/tree/master/broker)，因为 go-micro 做了抽象，在它们之间进行切换是十分容易的。比如你想将 nats 换为 googlepubsub：

```go
// 修改容器的环境变量
// MICRO_BROKER=nats
MICRO_BROKER=googlepubsub	

// 修改 user-service 导入的包
// _ "github.com/micro/go-plugins/broker/nats"
_"github.com/micro/go-plugins/broker/googlepubsub"
```

如果你不使用 go-micro，可使用 Go 实现的 [NATS](https://github.com/nats-io/go-nats)，事件发布：

```go
nc, _ := nats.Connect(nats.DefaultURL)

// Simple Publisher
nc.Publish("user.created", userJsonString)
```

事件订阅：

```go
// Simple Async Subscriber
nc.Subscribe("user.created", func(m *nats.Msg) {
    user := convertUserString(m.Data)
    go sendEmail(user)
})
```

使用如 NATS 的第三方消息代理插件，会让微服务失去使用 protobuf 进行二进制数据通信的优势（微服务之间不再直接调用），反而多了处理 JSON 数据的开销，不过 go-micro 对此早有对策。



### Pubsub 层

go-micro 内置有 pubsub 层，其位于代理层的顶端，无需第三方 NATS 消息代理，恰如其分的使用了我们定义好的 protobuf，更新 user-service 使用 pubsub 代替 NATS：

```go
// user-service/main.go

func main() {
    ... 
    publisher := micro.NewPublisher(topic, srv.Client())
	pb.RegisterUserServiceHandler(srv.Server(), &service{repo, tokenService, publisher})
    ... 
}
```

```go
// user-service/handler.go

func (h *handler) Create(ctx context.Context, req *pb.User, resp *pb.Response) error {
	// 哈希处理用户输入的密码
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	req.Password = string(hashedPwd)
	if err := h.repo.Create(req); err != nil {
		return nil
	}
	resp.User = req

	// 发布带有用户所有信息的消息
	if err := h.Publisher.Publish(ctx, req); err != nil {
		return err
	}
	return nil
}
```



更新邮件微服务：

```go
// email-service/main.go

type Subscriber struct{}

func main() {
    ...
    micro.RegisterSubscriber(topic, srv.Server(), new(Subscriber))
    ...
}

func (sub *Subscriber) Process(ctx context.Context, user *userPb.User) error {
	log.Println("[Picked up a new message]")
	log.Println("[Sending email to]:", user.Name)
	return nil
}
```

现在我们微服务底层就使用了 User Protobuf 定义，并没有使用第三方消息代理。运行效果如下：

![5.2](http://p7f8yck57.bkt.clouddn.com/2018-05-29-103213.gif)







## 总结

本节先使用 go-micro 的 NATS 消息代理插件，使 user-service 在创建新用户时发布一个带有用户信息且 topic 为 "user.created" 的消息事件，订阅了此 topic 的 email-service 接收到消息后取出用户信息来发送邮件。之后使用 go-micro 自带的 pubsub 层代替了 NATS 充分发挥 protobuf 通信的优势。

下节我们将使用 React 编写微服务的管理界面，并研究一下 web 端如何与微服务交互。

































