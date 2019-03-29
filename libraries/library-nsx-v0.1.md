---
title: nsx RPC 框架
date: 2019-03-29 09:09:27
tags: Golang
---

总结下 RPC 框架 [wuYin/nsx](<https://github.com/wuYin/nsx>) v0.1 版的设计思路和工作流程。

## 前言

不久前内部做了 RPC 的技术分享，参考分享的设计思路写了个简化的 RPC 框架：[nsx](https://github.com/wuYin/nsx)，目前为 v0.1 版，实现了基于 zookeeper 为注册中心的 RPC 服务调用，使用可参考 [examples](https://github.com/wuYin/nsx-cli/tree/master/examples)，后续完善和优化更多 feature

## 关于 RPC

我的简单理解是：将本地函数调用网络化，通过网络传递参数、执行并返回调用结果。网络调用会面临以下问题：

### 1. 通信协议

RPC 调用双方的进程是指 socket（IP:Port）唯一标识的进程，二者一般是指在同一局域网内不同主机上的两个进程，当然也可以只是本地系统中两个端口不一样的进程。

为保证网络调用的可靠性，传输层协议选择 TCP，但相比 UDP 每个数据报独立传输界线分明的特点，TCP 流式传输的二进制数据包之间是没有分界的。既然选择 TCP，那包的分割和拼接操作就需要由应用层的程序来实现。可参考  Redis Server 和 Client 间的通信协议 [protocol](<http://redisdoc.com/topic/protocol.html>)

### 2. 执行网络调用

本地调用：多个函数代码都在进程的代码段内存区域中，相互调用成本低，但耦合度高。
网络调用：进程 A 想调用进程 B 中的函数，需要像本地调用一样，将函数名、函数实参值通过网络发送给进程 B，等待函数执行完毕后将返回值通过网络响应给进程 A，交互的数据即通信的协议包中的包内容。那进程 A 如何调用非本地的函数呢？答案是使用  **反射**。

RPC 调用的核心代码如下，在进程 A 中被执行 add 调用的看似只是初始值为 `nil` 的 fakeAdd，但其实在 `MakeFunc` 中包含了复杂的网络调用过程：

```go
package main

import (
	"fmt"
	"reflect"
)

// 进程 A
func main() {
	var fakeAdd func(a, b int64) int64
	fakeType := reflect.TypeOf(fakeAdd)
	fakeAddV := reflect.MakeFunc(fakeType, addCaller)
	resps := fakeAddV.Call([]reflect.Value{ // 执行网络调用，并将返回值响应给调用方
		reflect.ValueOf(int64(1)),
		reflect.ValueOf(int64(1)),
	})

	fmt.Println("1+1 =", resps[0].Int()) // 1+1=2
}

func addCaller(in []reflect.Value) []reflect.Value {
	sum := add(in[0].Int(), in[1].Int()) // 此处省略了进程 A 向进程 B 发起网络调用请求并等待响应
	sumV := reflect.ValueOf(sum)
	return []reflect.Value{sumV}
}

// 进程 B
func add(base, diff int64) int64 {
	return base + diff
}
```



### 3. 服务注册

假设现有服务 add-service 实现了加法功能，即实现了 `Add` 函数能传入 2 个整数计算后返回和，假设已经是运行在主机 1 上的进程 A。现在主机 2 上的进程 B 要调用 `Add` 函数进行计算，那就需要先知道 add-service 的网络地址（IP:Port）是什么？

#### 3.1 初步方案：调用方和被调用方都要维护全部对方的地址

在主机 2 的内存中维护 [add-service : addr] 的哈希映射，进程 B 直接取地址调用即可。这就是最简单的 Registry，虽然实现了取地址功能，但缺陷也很明显：

- 发布低效
  每次 add-service 更新功能发布后需通知主机 2 更新 map 中的 addr，如果只有一个调用方更新一次还好，如果有 100 个调用方呢？那每次发布都需要逐一告知其他 100 台主机更新服务地址…效率很低，如果想改善可使用 UDP 广播机制一次性通知完毕，但就算你实现了，可靠性肯定不如 zk 的 broadcast。因此服务方和调用方都要维护对方的地址。服务方每次发布都要通知调用方地址变化，调用方地址变更也要通知服务方：

   <img src="https://images.yinzige.com/2019-03-29-031517.png" width=50%/>

- 机制可靠性低
  万一 add-service 因为某个操作 panic 了或 disk full 导致服务不可用，其他调用方每次都要傻傻的等待调用超时返回，而不是被告知说服务下线了不必再调用。如果 add-service 是多主机的，其他调用方还要实现服务切换功能。在分布式环境中，想要实现服务的高度可靠性并不容易。

#### 3.2 可靠方案：使用分布式协调服务 zookeeper 来实现注册中心

参考 dubbo 注册中心的结构：[Introduction to Dubbo](https://www.baeldung.com/dubbo)

 <img src="https://www.baeldung.com/wp-content/uploads/2017/12/dubbo-architecture-1024x639.png" width=60% /> 

参与对象：
- Provider：服务提供者，即被调用方，如主机 1 上 add-service 的进程 A
- Consumer: 服务消费者，即调用方，如主机 2  上的进程 B
- Registry：注册中心，add-service 注册，进程 B 获取服务调用地址的地方

工作流程：
- Register: 每次服务发布 Provider 都会向 Registry 发起注册，将它的服务名称和网络地址存储在注册中心
- Subscribe：很多调用方 Consumer 将想调用的服务订阅到 Registry 中，当某个关心的服务上线或下线时都会 Notify Consumer
- Invoke：Consumer 获取到服务地址后，直接发起调用并阻塞等待返回响应或调用超时

现在的 nsx 支持上边 2 种方案：简单 Registry 和分布式 zk Registry，但是没有 Notify 和 Monitor 功能，后续使用 zk 的 event 通知机制继续完善。



## nsx 架构

nsx 框架分为了三个子项目：[nsx](https://github.com/wuYin/nsx) 服务方，[nsx-cli](https://github.com/wuYin/nsx-cli) 调用方，二者共同依赖底层网络框架 [tron](https://github.com/wuYin/tron) 来完成通信。

### Tron 网络框架

现在的 v0.2 相比 v0.1 只加入了包序列管理功能。文档地址：[docs/v0.1.md](https://github.com/wuYin/tron/blob/master/docs/v0.1.md)
Tron 定义了自己的 Server 和 Client，并将二者的每次连接都抽象成 Session，它们数据交互如下：

 <img src="https://images.yinzige.com/2019-03-29-041223.png" width=80%/>

注：虚线的 connect / dispatch 过程只会进行一次，实线的 packet 交互过程会进行多次。
其中 Client 和 ServerWorker 使用相同的代码，作为通信双方都能在连接超时后进行二次规避策略的重连直到超过指定次数，此外对于 packet 的读写提供了 Codec 接口，Tron 提供了默认的 codec 实现，同时也让使用 Tron 框架的第三方能够自定义 packet 的读写格式，如 nsx 和 nsx-cli 就是复用了 Redis Protocol 进行调用交互。

### nsx: Service Provider 

调用流程：每个 Provider 在实例化时都会启动一个底层 Tron Server 运行并监听，并且会将自身服务的实现托管到 ServiceManager 中，在 Manager 内部会将服务实现的各个方法取反射值并记录方法 in 和 out 的参数 Type 以便在被调用时做校验。当接收到调用请求后，使用自定义的 Codec 解码出调用参数，再告诉 manager 执行调用并将调用结果异步写回给调用方。

注册中心：nsx 内部实现了一个默认的 Registry Proxy，它本身只是发起注册/下线请求的代理，至于注册中心本身是由 nsx Server 实例内部来维护的，它就是上边说的注册中心初步方案。此外还提供了 zk 实现的注册中心功能，能进行服务的注册和调用，但基于 event 的服务发布功能还待实现。

### nsx-cli: Service Consumer

调用流程：每个 Consumer 在实例化时会定时从配置好的服务中心拉取自己关心的服务地址，并且逐一启动底层的 Tron Client 进行连接，连接成功后待命等待被调用。此外会对所有服务的方法进行网络调用的包装，当调用执行时阻塞等待调用响应或超时。



## 总结

整个 nsx RPC 框架的大概工作流程如上，其实还有很多可以改进的地方，如：
- 服务通知：通过 zk event 实现服务订阅的 Notify
- worker 池化：Server 维护一定数量 worker goroutine pool 用于请求处理，节省内存资源

对了，nsx 项目名来源于 Grand Tour S02 E01 中的很酷的 Honda NSX 混动跑车，Tron 是上海迪士尼乐园极速光轮项目的名字，我也很喜欢电影《TRON》中 Draft Punk 的配乐，项目都很酷，期待 v0.2 更多特性。