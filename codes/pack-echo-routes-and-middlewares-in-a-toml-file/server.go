package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/labstack/echo"

	"github.com/naoina/toml"
)

// 路由的处理器
type uriHandler struct {
	handlerName string
	httpMethod  string
	method      reflect.Value
}

// 验证中间件
type validator struct {
	uriPrefix      string
	methodName     string
	skipUris       map[string]*interface{}
	validateMethod echo.MiddlewareFunc
}

// 封装后的 Echo Server
type MyEchoServer struct {
	server           *echo.Echo
	uri2Handler      map[string]*uriHandler  // uri handler
	method2Uris      map[string][]string     // method []uri
	uri2Validators   map[string][]*validator // uri []validator
	method2Validator map[string]*validator   // method []validator
	validatorPrefix  []string
}

// 路由和中间件配置
type Conf struct {
	Routes     map[string]string   // 路由及其 handler
	Validators map[string][]string // 路由及其 validator 群组
}

func initEnv() *MyEchoServer {
	// 读取路由文件
	rfile, err := os.Open("./conf.toml")
	checkErr("open routes.toml error:", err)
	defer rfile.Close()

	rdata, err := ioutil.ReadAll(rfile)
	checkErr("read routes.toml error: ", err)

	var conf Conf
	err = toml.Unmarshal(rdata, &conf)
	checkErr("unmarshal toml data error: ", err)

	// 遍历处理路由
	routes := make(map[string]*uriHandler, 10)
	method2Uris := make(map[string][]string, 10)
	for route, handler := range conf.Routes {
		mURI := strings.SplitN(route, ":", 2) // Method:URL
		method := http.MethodPost
		if len(mURI) > 1 {
			reqMethod := strings.ToUpper(mURI[0])
			switch reqMethod {
			case "FILE":
				method = "FILE"
			case "STATIC":
				method = "STATIC"
			default:
				method = reqMethod
			}
			route = mURI[1]
		}
		// 建立 uri 及其 handler 的映射关系
		routes[route] = &uriHandler{handlerName: handler, httpMethod: method}

		// 建立 handler 及其 uris 的映射关系
		uris, ok := method2Uris[handler]
		if !ok {
			uris = make([]string, 0, 2)
		}
		uris = append(uris, route)
		sort.Strings(uris)
		method2Uris[handler] = uris
	}

	// 遍历处理验证 uri 及其需要验证的中间件群组
	validatorPrefix := make([]string, 0, len(conf.Validators))
	uri2Validators := make(map[string][]*validator, 10)
	method2Validator := make(map[string]*validator)
	for route, handlers := range conf.Validators {
		verifier, ok := uri2Validators[route]
		if !ok {
			verifier = make([]*validator, 0, len(handlers))
			validatorPrefix = append(validatorPrefix, route)
		}
		// 遍历中间件
		for _, handler := range handlers {
			method := strings.TrimPrefix(handler, "!")
			vali, ok := method2Validator[method]
			// 如果有这个函数就使用
			if !ok {
				vali = &validator{methodName: method, skipUris: make(map[string]*interface{}, 2)}
				method2Validator[method] = vali
			}
			// 方法以 ! 开头则不使用该中间件过滤
			if strings.HasPrefix(handler, "!") {
				vali.skipUris[route] = nil
			}
			verifier = append(verifier, vali)
		}
		uri2Validators[route] = verifier
	}
	sort.Strings(validatorPrefix)

	// 创建封装后的 echo server
	return &MyEchoServer{
		server:           echo.New(),
		uri2Handler:      routes,
		method2Uris:      method2Uris,
		uri2Validators:   uri2Validators,
		method2Validator: method2Validator,
		validatorPrefix:  validatorPrefix,
	}
}

func main() {
	echoServer := initEnv()
	fmt.Printf("%+v", echoServer) // 读取成功
}

func checkErr(info string, err error) {
	if err != nil {
		log.Fatalln(info, err)
	}
}
