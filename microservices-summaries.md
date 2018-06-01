---
title: Golang 微服务系列教程总结
date: 2018-06-01 14:34:01
tags: 微服务
---

回顾一下 Golang 微服务系列教程的翻译学习过程。

<!-- more -->

## 前言

### 起因

今年三月份换了工作，方向从 PHP RESTful API 开发转到 Golang 微服务开发，Google 搜到排名靠前的 [Ewan Valentine](https://ewanvalentine.io/) 微服务教程博客，细看了下第一篇提及到的技术栈，正合口味，在邮件和 Twitter 与原作者取得联系后便开始边学习边翻译，受益匪浅。

### 技术栈

Golang、Protobuf、gRPC 与 go-micro 微服务框架、Docker 容器、JWT 认证、NATS 消息系统

### 致谢

感谢 [Ewan Valentine](https://ewanvalentine.io/) 的高质量博客，感谢 [dingkewz](https://blog.dingkewz.com/categories/%E6%8A%80%E6%9C%AF/) 豆友提供的参考，在此表示感谢。



---





## 内容

### 说明

Ewan 写的文章排版比较乱，我用 Markdown 进行了归纳重排，并加入了运行效果 Gif 图、结构导图等，让读者更易理解。

### 章节

[第一节](https://github.com/wuYin/blog/blob/master/microservices-part-1-introduction-and-consignment-service.md)：使用 gRPC 与 Protobuf  搭建完成了第一个微服务 consignment-service

[第二节](https://github.com/wuYin/blog/blob/master/microservices-part-2-use-go-micro-and-dockerising.md)：使用 go-micro 代替 gRPC，并进行了微服务的 Docker 化，新加了微服务 vessel-service

[第三节](https://github.com/wuYin/blog/blob/master/microservices-part-3-docker-compose-and-mongodb-with-orm.md)：引入 docker-compose 统一管理容器并新加微服务 user-service，使用 GORM 库与存储用户数据的 Postgres 进行了交互。

[第四节](https://github.com/wuYin/blog/blob/master/microservices-part-4-auth-user-by-jwt.md)：使用哈希处理用户的明文密码，引入 JWT 在微服务之间做用户的认证。

[第五节](https://github.com/wuYin/blog/blob/master/microservices-part-5-event-brokering-with-go-micro.md)：引入 go-micro 的 NATS 插件将三个微服务重构为事件驱动模式，最后替代使用自带的 pubsub 层。

[第六节](https://github.com/wuYin/blog/blob/master/microservices-part-6-web-clients.md)：引入 go-micro 的 API 工具为 web 端提供接口，还在浏览器 JS 中完成调用。

七八九节用到了的 Kubernetes、Terraform 等技术都是基于 Google Cloud，实属译者能力不足，有需要的读者可 [阅读原文](https://ewanvalentine.io)，后续有机会再更新。

### 源码

[shippy](https://github.com/wuYin/shippy)：一个章节对应一个分支，注释都写的比较清楚。



---



## 最后

由于译者只是应届生工作经验有限，文中不足之处恳请读者指正，直接在文章下的 Disqus 评论区留言、提 issue均可。有什么问题可随时给我发邮件：wuYinPost@gmail.com

博客持续更新高质量文章，感谢阅读 :)

