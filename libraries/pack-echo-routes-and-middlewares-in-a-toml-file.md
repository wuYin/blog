---
title: 封装 Echo 框架的路由与中间件
date: 2018-03-22 20:38:54
tags: Golang
---

将 [Echo](https://github.com/labstack/echo) 的路由与中间件配置封装到一个 [toml](https://github.com/toml-lang/toml) 文件中，统一管理。

<!-- more -->

本文完整代码：[GitHub](https://github.com/wuYinBest/blog/tree/master/codes/pack-echo-routes-and-middlewares-in-a-toml-file)

## 问题

使用 echo 框架，会发现路由需手动写在 `main()` 中并指定 `HandlerFunc`，混合了路由和业务逻辑，当业务变得繁杂时，有上百个路由时，管理起来很麻烦。反观 [beego](https://github.com/beego/beedoc/blob/master/zh-CN/mvc/controller/router.md) 或 [Laravel](https://laravel-china.org/docs/laravel/5.5/routing) 会集中将路由配置到一个文件，分离了路由与业务，方便新增和修改。



## 解决方案

### 手动注册处理器和中间件

将路由和中间件封装到一个配置文件中，在文件中指定路由名称、请求方法及其对应 `HandlerFunc`、`MiddlewareFunc` 的完整路径，在 server 启动时为路由注册处理器和中间件即可。

### 实现效果

将路由和中间件写入 conf.toml 中的 `[routes]` `[validators]` 两个数组，在请求 `/user` 路由时：

- main 包下 recevier 为 `CommonValidator` 的 `CheckSession()` 中间件，做 Session 检测等验证
- main 包下 recevier 为 `UserInfoHandler` 的 `GetUserInfo()` 处理器，做请求的业务处理

  <img width="60%" src="https://contents.yinzige.com/re-echo.png"/>

若真只有一个 `/user` 路由，直接在 server.go 中实现处理器和中间件即可。但在实际项目中往往有上百个路由和中间件，此时使用 conf.toml 来集中管理，将十分的方便。



## 写入路由

路由配置文件有两种格式可选：JSON 和 toml

### 使用 JSON

 `Routes()` 返回 echo 框架加载的全部路由，导出为 JSON 时格式如下：

```json
[
  {
    "method": "POST",
    "path": "/users",
    "handler": "main.GetUserInfo"
  }
]    
```

优点：结构清晰明了；缺点：有很多路由时，同样会有很多 method、path 和 handler 等字段名，浪费大量的存储空间。



### 使用 toml

toml 格式的文件能完美解决 JSON 存在的存储浪费的问题，同样的 /users 的路由，可直接存为：

```Toml
[routes]
"POST:/users"="main.UserInfoHandler.GetUserInfo"
```



## 实现

### 封装后的 echo.Echo

```go
// 封装后的 Echo Server
type MyEchoServer struct {
	server            *echo.Echo               // 封装后的 server 实例
	route2Handler     map[string]*routeHandler // 一条 route 对应一个 handler
	route2Validators  map[string][]*validator  // 一个 route 对应多种 validator
	handler2Routes    map[string][]string      // 一个 handler 对应多条 route
	handler2Validator map[string]*validator    // 一个 handler 对应一种 validator
	verifyPrefixs     []string                 // 需要中间件验证的 route 前缀
}

// 路由的处理器
type routeHandler struct {
	handlerName string
	httpMethod  string
	method      reflect.Value
}

// 路由的中间件
type validator struct {
	routePrefix    string
	handlerName    string
	skipRoutes     map[string]*interface{}
	validateMethod echo.MiddlewareFunc
}

// toml 路由和中间件配置
type Conf struct {
	Routes     map[string]string   // route 及其 handler
	Validators map[string][]string // route prefix 及其 validators
}
```



### 读取配置文件并初始化 MyEchoServer

```go
// 读取配置
func initEnv() *MyEchoServer {
	// 读取路由文件
	rdata, err := ioutil.ReadFile("./conf.toml")
	checkErr("read routes.toml error: ", err)

	var conf Conf
	err = toml.Unmarshal(rdata, &conf)
	checkErr("unmarshal toml data error: ", err)

	// 初始化 MyEchoServer 实例
	// 生成 route 与 handler 的双向映射关系
	route2Handler := make(map[string]*routeHandler, 10)
	handler2Routes := make(map[string][]string, 10)
	for route, handler := range conf.Routes {
		mr := strings.SplitN(route, ":", 2) // 分割 "POST:/user"
		method := http.MethodPost
		if len(mr) > 1 {
			reqMethod := strings.ToUpper(mr[0])
			switch reqMethod {
			case "FILE":
				method = "FILE"
			case "STATIC":
				method = "STATIC"
			default:
				method = reqMethod
			}
			route = mr[1]
		}
		// 建立 route 与 handler 的一对一关系
		route2Handler[route] = &routeHandler{handlerName: handler, httpMethod: method}

		// 建立 handler 与 routes 的一对多关系
		routes, ok := handler2Routes[handler]
		if !ok {
			routes = make([]string, 0, 2)
		}
		routes = append(routes, route)
		sort.Strings(routes)
		handler2Routes[handler] = routes
	}

	// 遍历生成 route 与 validator、handler 的双向映射关系
	verifyPrefixs := make([]string, 0, len(conf.Validators))
	route2Validators := make(map[string][]*validator, 10)
	handler2Validator := make(map[string]*validator)
	for route, handlers := range conf.Validators {
		// 建立 route 与 validators 的一对多关系
		verifier, ok := route2Validators[route]
		if !ok {
			verifier = make([]*validator, 0, len(handlers))
			verifyPrefixs = append(verifyPrefixs, route)
		}
		// 遍历多个中间件
		for _, handler := range handlers {
			h := strings.TrimPrefix(handler, "!")
			v, ok := handler2Validator[h]
			// 建立 handler 与 validator 的一对一关系
			if !ok {
				v = &validator{handlerName: h, skipRoutes: make(map[string]*interface{}, 2)}
				handler2Validator[h] = v
			}
			// 以 ! 开头的 handler 失效，对该路由不使用该中间件
			if strings.HasPrefix(handler, "!") {
				v.skipRoutes[route] = nil
			}
			verifier = append(verifier, v)
		}
		route2Validators[route] = verifier
	}
	sort.Strings(verifyPrefixs)

	// 创建封装后的 echo server
	return &MyEchoServer{
		server:            echo.New(),
		route2Handler:     route2Handler,
		route2Validators:  route2Validators,
		handler2Routes:    handler2Routes,
		handler2Validator: handler2Validator,
		verifyPrefixs:     verifyPrefixs,
	}
}
```



### 将 route handler 反射到 HandlerFunc 实例

```Go
// 为 server 注册路由的处理器
func (s *MyEchoServer) registerHandler(h interface{}) *MyEchoServer {
	rHVal := reflect.ValueOf(h)
	rHType := rHVal.Elem().Type()
	rHPath := rHType.String() // package.Struct.HandlerFunc

	// 路由的所有 handler
	handlers := make([]string, 0, 10)
	for handler := range s.handler2Routes {
		handlers = append(handlers, handler)
	}
	sort.Strings(handlers)

	used := false
	// 遍历所有 handler 下的所有 route
	for _, handler := range handlers {
		routes := s.handler2Routes[handler]
		for _, route := range routes {
			// 当前 handler 处理当前的 route
			if strings.HasPrefix(handler, rHPath) {
				handlerName := strings.TrimPrefix(strings.TrimPrefix(handler, rHPath), ".")
				method := rHVal.MethodByName(handlerName)
				if method.Kind() == reflect.Invalid || method.IsNil() {
					log.Panicf("ERROR:\nMethod %s Not Exist In %s", method, rHPath)
				}

				// 建立一对一的映射关系
				s.route2Handler[route].method = method
				used = true
				log.Printf("ROUTE INFO:\nRegister Succeed: %s -> %s.%s", route, rHPath, handlerName)
			}
		}
	}
	if !used {
		log.Printf("WARN:\nNot Used: %s", rHPath)
	}

	return s
}
```





### 将 validator handler 反射到 MiddwareFunc 实例

```go
// 为 server 注册中间件的处理器
func (s *MyEchoServer) registerValidator(v interface{}) *MyEchoServer {
	vVal := reflect.ValueOf(v)
	vType := vVal.Elem().Type()
	vName := vType.String()

	// 所有待验证的路由
	routes := make([]string, 0, 10)
	for route := range s.route2Validators {
		routes = append(routes, route)
	}
	sort.Strings(routes)

	// 遍历所有需要处理的路由
	used := false
	for _, route := range routes {
		// 该路由需要处理的所有中间件
		validators := s.route2Validators[route]

		for _, v := range validators {
			// 当前中间件合处理当前路由
			if strings.HasPrefix(v.handlerName, vName) {
				handlerName := strings.TrimPrefix(strings.TrimPrefix(v.handlerName, vName), ".")
				method := vVal.MethodByName(handlerName)
				// 检查 handler 是否可用
				if method.Kind() == reflect.Invalid || method.IsNil() {
					log.Panicf("ERROR:\nMethod %s Not Exist In %s", method, vName)
				} else {
					// 检查 handler 的类型
					ok := method.Type().ConvertibleTo(reflect.TypeOf((func(echo.HandlerFunc) echo.HandlerFunc)(nil)))
					if !ok {
						log.Panicf("ERROR:\nMethod %s Not MiddlewareFunc", handlerName)
					}
					// 建立中间件与处理器的映射关系
					v.validateMethod = method.Interface().(func(echo.HandlerFunc) echo.HandlerFunc)
					used = true
				}
				log.Printf("VALIDATOR INFO:\nRegister Succeed: %s -> %s.%s", route, vName, handlerName)
			}
		}
	}
	if !used {
		log.Printf("WARN:\nNot Used: %s", vName)
	}
	return s
}
```



### 启动 Server

```go
// 启动 Server
func (s *MyEchoServer) start() {
	// 检查中间件是否都注入成功
	for r, vs := range s.route2Validators {
		for _, v := range vs {
			if nil == v.validateMethod {
				panic(r + " -> " + v.handlerName + " is NOT INJECT")
			}
		}
	}

	// 取出所有路由
	routes := make([]string, 0, 10)
	for r := range s.route2Handler {
		routes = append(routes, r)
	}
	sort.Strings(routes)

	// 为所有路由注册处理器
	for _, route := range routes {
		handler := s.route2Handler[route]
		// 检查 handler 是否注入
		m := strings.ToUpper(handler.httpMethod)
		if (handler.method.Kind() == reflect.Invalid) && m != "FILE" && m != "STATIC" {
			log.Panicf("ERROR:\nHandler Not Exist: %s -> %s", route, handler.handlerName)
		}

		// 注册路由的处理器
		handleFunc := func(ctx echo.Context) error {
			context := reflect.ValueOf(ctx)
			handler.method.Call([]reflect.Value{context})
			return nil
		}

		// 注册路由的中间件
		usedValidators := make([]echo.MiddlewareFunc, 0, 10)
		for _, prefix := range s.verifyPrefixs {
			if strings.HasPrefix(route, prefix) {
				validators, ok := s.route2Validators[prefix]
				if ok {
				FLAG:
					for _, v := range validators {
						// 检查当前路由是否要跳过当前中间件
						for skipPrefix := range v.skipRoutes {
							if strings.HasPrefix(route, skipPrefix) {
								log.Printf("INFO:\nRoute Skipped Vlidator: %s -x-> %s", route, v.handlerName)
								continue FLAG
							}
						}
						// 为当前路由添加中间件
						usedValidators = append(usedValidators, v.validateMethod)
					}
				}
			}
		}

		// 根据 handler 类型来发布 route
		switch m {
		case http.MethodGet:
			s.server.GET(route, handleFunc, usedValidators...)
		case http.MethodPost:
			s.server.POST(route, handleFunc, usedValidators...)
		case http.MethodHead:
			s.server.HEAD(route, handleFunc, usedValidators...)
		case "FILE":
			s.server.File(route, handler.handlerName)
		case "STATIC":
			s.server.Static(route, handler.handlerName)
		default:
			s.server.GET(route, handleFunc, usedValidators...)
		}
		log.Fatalln(s.server.Start(":2333"))
	}
}
```

至此就完成了封装的全部工作。



## 总结

其实封装仅三步：读取配置数组，使用反射生成为路由指定的 `HandlerFunc` 和 `MiddlewareFunc` 实例，手动发布。

倒是反射用得不太熟，下篇文章学习下就学习下它吧 :)





