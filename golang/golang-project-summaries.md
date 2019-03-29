---
title: Golang API 项目总结
date: 2018-02-17 23:14:07
tags: Golang
---

年前最后一周，斗鱼户外第一主播远洋君，也就是我们公司的大客户要跳槽某大厂直播，需要和大厂做数据对接，我们能拿到他直播时房间所有的弹幕、礼物完整信息，比起原来在斗鱼抓取弹幕和礼物数据时会出现丢包、积分计算误差等的情况，显然精确数据能带来更佳的用户体验。

<!-- more -->

### 前言

#### 产品背景

我们的小程序产品是做主播与粉丝间有效交流的工具。简单来说，是粉丝在主播的直播间行为变现，粉丝在直播间的发弹幕、送礼物的行为，能在小程序中折算为积分，积分加微信支付即可购买主播准备好的实惠精美礼物，类似饿了么的金币商城。站在主播角度，更多的弹幕和礼物能给自己带来更高的热度和收入，站在用户的角度，能通过定量的弹幕自动换算为积分，加现金购买实惠且精美的礼物，双方互利共赢。从远洋君的最高月流水 10w+ RMB 说明这个 MVP 产品的想法是验证成功的。

#### 技术要求

粉丝在远洋君直播间每发一条弹幕、每送一份礼物，都要调用我们的 API 为该直播用户的微信身份换算积分、记录历史弹幕和礼物等。这对 API 实时性要求十分高，所以我没有用熟悉的 PHP 去写业务逻辑，Laravel 过于繁重，尽管使用框架缓存、开启Opcache等优化过，每次请求处理花在框架加载的时间依旧在 60ms 左右，所以选择使用 Golang 来做业务处理。

#### API 性能

最后使用 [Siege](https://github.com/JoeDog/siege) 做 API 压力测试时，单个请求处理的时间在 1~1.5ms，QPS 在 700~1000 之间，如下图：

 ![](https://ws2.sinaimg.cn/large/006tNc79gy1fojvvj6qdwj30dc04laax.jpg)



### 实现

#### 框架与工具

使用 Golang 写 Web API 可选择的框架可以参考 [7 frameworks to build a rest-api in go](https://nordicapis.com/7-frameworks-to-build-a-rest-api-in-go/)，由于高性能、易使用我选择 [Gin](https://gin-gonic.github.io/gin/)，注意发布代码时使用 release 版本能提高性能。另外与数据库交互的表只有 4 张，直接使用 SQL 语句没使用 ORM

这里立一个Flag，4 月份前学习 [qb](https://github.com/aacanakin/qb) 等项目，由简到繁自己实现 Go 的 ORM framework

#### 缓存与文件

弹幕处理流程如下：直播平台使用 JSON 格式传递用户的弹幕数据，在接收到请求后解析获取弹幕内容，使用 Redis 的 2 个 key 记录弹幕：

- 弹幕内容 key：发送弹幕的用户 UID 作为 key 标识的一部分如 `barrage:UID:contents`，不断将弹幕 `APPEND` 到该值中，使用指定分隔符隔开
- 弹幕数量 key：如 `barrage:UID:counts`，每收到该 UID 用户发送一条弹幕的请求就执行 `INCR`

当弹幕每超过一定数量（取余）时，就取出弹幕内容写回文件。礼物数据的处理类似。



### 总结

#### 配置文件

在项目中需要连接数据库、Redis，配置弹幕、礼物数据写回文件的数量单位等，于是将这些配置项拆为单独的配置文件 `config.json`，在 `func init() {...}`  中读取配置到全局变量。

#### 错误处理

```go
func checkError(err error, info string) {
	if err != nil {
		log2.Println(info)
	    // log2.Fatal(err)
	}
}
```

先来看我项目中进行了多少次错误处理，占了代码量的 8%，显然出现了代码冗余，仿佛回到了 C 的年代。

 ![](https://contents.yinzige.com/check-errors.png)

不过这也是 Golang 设计的优异之处，相比 PHP 的异常处理机制：

```php
try {
	// 大量的业务代码   
} catch (\Exception $e) {
	// $e->getMessage()、$e->getLine() 等获取异常信息
}    
```

Golang 则将错误作为单独的 `error interface{}`数据类型的值返回，在调用者处做错误处理。这样做能分离 error 与产生 error 的代码上下文，相比 try-catch 机制能自定义错误输出、精确的进行异常处理。

#### SQL 优化

项目的 SQL 大部分是 Query、Update 操作，即涉及大量的查询操作，优化了表之间的关联结构、建立索引等，最近在重读《高性能MySQL》，更多优化方法正在学习中。

#### 编码技巧

第一步写业务逻辑，应该熟悉完整的业务流程、做好异常处理。

第二步抽离代码，保证每个 `func` 在 40~50 行代码之间。

第三步做好测试，保证正常数据、异常数据均可正常处理。



### 最后

这是第一个使用 Golang 独立完成的项目，性能是真的惊艳，师父也说虽不如 C++ 但对一般 Web 项目性能足矣。另外，ORM 的轮子二月下旬回去就开始造 233