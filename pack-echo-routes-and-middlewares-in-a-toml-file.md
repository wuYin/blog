---
title: 封装 Echo 框架的路由与中间件
date: 2018-03-22 20:38:54
tags: Golang
---

将 [Echo](https://github.com/labstack/echo) 的路由与中间件配置封装到一个 [toml](https://github.com/toml-lang/toml) 文件中，统一管理。

<!-- more -->

### 问题

使用 echo 框架，你会发现 URI 路由需要手动写在 `main()` 中并为其指定 `HandlerFunc`，混合了路由和业务逻辑，当业务变得繁杂时，管理路由将很麻烦。

反观 [beego](https://github.com/beego/beedoc/blob/master/zh-CN/mvc/controller/router.md) 或 [laravel](https://laravel-china.org/docs/laravel/5.5/routing) 会集中管理路由到一个文件中的方式，分离了路由与业务，方便修改和新增路由。



### 解决方案

将 echo 的路由和中间件封装到一个配置文件中，在文件中指定路由名称、请求方法及其对应的 `HandlerFunc`，在 server 启动时使用 `Add(method, path string, handler HandlerFunc)` 注册所有的路由即可。



### 写入路由

路由配置文件有两种格式可选：JSON 和 toml

#### 使用 JSON

 `Routes()` 返回 echo 框架加载的全部路由，导出为 JSON 时格式如下：

```json
[
  {
    "method": "POST",
    "path": "/users",
    "handler": "main.CreateUser"
  },
  {
    "method": "GET",
    "path": "/users",
    "handler": "main.FindUser"
  }
]    
```

同样的 JSON 加载回去，依次注册即可。

优点：结构清晰明了

缺点：有很多路由时，同样会有很多 method、path 和 handler 等字段名，浪费大量的存储空间。



#### 使用 toml

toml 格式的文件能完美解决 JSON 存在的存储浪费的问题，同样的 2 个 users 的路由，可直接存为：

```Toml
[routes]
"POST:/users"="main.CreateUser"
"GET:/users"="main.FindUser"
```



### 读取路由





封装后的完整代码：[GitHub](https://github.com/wuYinBest/blog/tree/master/codes/pack-echo-routes-and-middlewares-in-a-toml-file)

Last updated at 2018.03.23 21:45































