package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/labstack/echo"

	"github.com/naoina/toml"
)

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

// 封装后的 Echo Server
type MyEchoServer struct {
	server            *echo.Echo               // 封装后的 server 实例
	route2Handler     map[string]*routeHandler // 一条 route 对应一个 handler
	route2Validators  map[string][]*validator  // 一个 route 对应多种 validator
	handler2Routes    map[string][]string      // 一个 handler 对应多条 route
	handler2Validator map[string]*validator    // 一个 handler 对应一种 validator
	verifyPrefixs     []string                 // 需要中间件验证的 route 前缀
}

// 路由和中间件配置
type Conf struct {
	Routes     map[string]string   // route 及其 handler
	Validators map[string][]string // route prefix 及其 validators
}

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

func main() {
	echoServer := initEnv()
	echoServer.registerHandler(&UserInfoHandler{})
	echoServer.registerValidator(&CommonValidator{})
	echoServer.start()
	// fmt.Printf("%+v", echoServer) // 读取成功
}

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

type UserInfoHandler struct{}

func (u *UserInfoHandler) GetUserInfo(ctx echo.Context) {
	println("调用 GetUserInfo 处理业务逻辑")
	println("获取请求的 name 参数，值为: ", ctx.FormValue("name"))
}

type CommonValidator struct{}

func (v *CommonValidator) CheckSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		println("调用 CheckSession 通过 Session 中间件检测")
		next(ctx)
		return nil
	}
}

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

func checkErr(info string, err error) {
	if err != nil {
		log.Fatalln(info, err)
	}
}
