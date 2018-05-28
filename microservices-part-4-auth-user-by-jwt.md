---
title: Golang 微服务教程（四）
date: 2018-05-27 14:08:21
tags: 微服务
---

原文链接：[ewanvalentine.io](https://ewanvalentine.io/microservices-in-golang-part-4)，翻译已获作者 [Ewan Valentine ](https://twitter.com/Ewan_Valentine)授权。

本文完整代码：[GitHub](https://github.com/wuYin/shippy/tree/feature/part4)

<!-- more -->

上节引入 user-service 微服务并在 Postgres 中存储了用户数据，包括明文密码。本节将对密码进行安全的加密处理，并使用唯一的 token 来在各微服务之间识别用户。

在开始之前，需要手动运行数据库容器：

```shell
$ docker run -d -p 5432:5432 postgres
$ docker run -d -p 27017:27017 mongo
```



## 密码的哈希处理

### 安全原则

遵循 ”即使发生数据泄露，密码等敏感数据也不能被还原“ 的原则，永远都不要明文存储用户的密码。尽管一直这么说，但仍有项目是明文存储，比如以前的 CSDN

### 哈希处理

现在更新一下 user-service/handler.go 中处理密码的逻辑，将密码进行哈希处理，再进行存储：

```go
// user-service/hander.go

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
	return nil
}

func (h *handler) Auth(ctx context.Context, req *pb.User, resp *pb.Token) error {
	u, err := h.repo.GetByEmailAndPassword(req)
	if err != nil {
		return err
	}

	// 进行密码验证
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return err
	}
	t, err := h.tokenService.Encode(u)
	if err != nil {
		return err
	}
	resp.Token = t
	return nil
}
```

只在两个函数上有改动：在 `Create()` 中添加了密码哈希处理的逻辑，在 `Auth()` 中用密码的哈希值做验证。重新创建用户，密码如下：

![image-20180527150635158](http://p7f8yck57.bkt.clouddn.com/2018-05-27-070635.png)

现在已经成功结合数据库完成用户的密码验证，在多个微服务之间进行用户验证有很多选择方案，本文使用 JWT



## JWT

### 简介

JWT 是 JSON web tokens 的缩写，是一种类似 OAuth 的分布式安全协议。理解起来很简单，JWT 协议算法能为每个用户生成独特的哈希字符串，并以此来做校验和识别。此外，用户的 metadata（信息数据）也能作为加密字符串的一部分。比如：

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ
```

可以看到 token 是被 "." 分割为三部分的字符串：`header.payload.signature`

#### header

JWT 头部包含两部分：声明 token 类型、加密 token 的算法，其进行 base64 加密后作为 token 的第一部分：

```json
{
  "typ": "JWT",
  "alg": "HS256"
}
```

用于告知客户端如何解码 token

#### payload

存放 metadata 的地方，比如用户数据、token 过期时间等。

#### signature

将 header、payload 及密钥共同加密后获取，用于客户端做数据校验，保证 token 在传输过程中没有被更改。当然，JWT 也有其不足之处，可参考：[Stop using JWT for sessions](http://cryto.net/~joepie91/blog/2016/06/13/stop-using-jwt-for-sessions/)

我建议你把创建用户信息的 IP 也放到 payload 中一起生成 token，这样能有效避免其他人盗用 token 后在其他设备伪造用户身份使用，此外也可使用 HTTPS 加密传输来避免中间人攻击。

JWT 的哈希算法大致可分为两类，对称加密和非对称加密。前者使用同一个私钥进行加解密，后者使用公钥加密私钥解密，非对称加密算法在微服务间做认证用得比较多。可参考：[JWT Signing Algorithms Overview](https://auth0.com/blog/json-web-token-signing-algorithms-overview/) 、[JSON Web Algorithms](https://tools.ietf.org/html/rfc7518#section-3)



### 使用

我们使用第三方开源库：[dgrijalva/jwt-go](https://github.com/dgrijalva/jwt-go)，使用示例直接参考文档。

#### JWT 加密与解密

根据用户的信息生成 JWT token 字符串，修改 user-service/token_service.go

```go
// user-service/token_service.go
package main
import (...)

type Authable interface {
	Decode(tokenStr string) (*CustomClaims, error)
	Encode(user *pb.User) (string, error)
}

// 定义加盐哈希密码时所用的盐，要保证其生成和保存都足够安全，比如使用 md5 来生成
var privateKey = []byte("`xs#a_1-!")

// 自定义的 metadata，在加密后作为 JWT 的第二部分返回给客户端
type CustomClaims struct {
	User *pb.User
	// 使用标准的 payload
	jwt.StandardClaims
}

type TokenService struct {
	repo Repository
}

// 将 JWT 字符串解密为 CustomClaims 对象
func (srv *TokenService) Decode(tokenStr string) (*CustomClaims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return privateKey, nil
	})
	// 解密转换类型并返回
	if claims, ok := t.Claims.(*CustomClaims); ok && t.Valid {
		return claims, nil
	} else {
		return nil, err
	}
}

// 将 User 用户信息加密为 JWT 字符串
func (srv *TokenService) Encode(user *pb.User) (string, error) {
	// 三天后过期
	expireTime := time.Now().Add(time.Hour * 24 * 3).Unix()
	claims := CustomClaims{
		user,
		jwt.StandardClaims{
			Issuer:    "go.micro.srv.user", // 签发者
			ExpiresAt: expireTime,
		},
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return jwtToken.SignedString(privateKey)
}
```

代码中我们使用了 `privateKey` 作为 JWT 加密的盐，在生产环境中一定要使用更安全的值而且要妥善保管。上边代码的注释比较详细，大致内容是：

`Decode()` 接受一个 string 参数，将其解析为 jwt token 对象，并验证其信息是不是用户信息，是的话则取出 metadata 获取用户信息。

`Encode()` 接受一个 user metadata 数据，哈希处理为 JWT token 字符串后返回。



#### token 生成

现在我们有了验证 token 的  service，来更新一下 user-cli 的代码，在这里只是为了跑一遍 token_service 的验证过程，用户数据先写死在代码中。

```go
package main
import (...)

func main() {

	cmd.Init()
	// 创建 user-service 微服务的客户端
	client := pb.NewUserServiceClient("go.micro.srv.user", microclient.DefaultClient)

	// 暂时将用户信息写死在代码中
	name := "Ewan Valentine"
	email := "ewan.valentine89@gmail.com"
	password := "test123"
	company := "BBC"

	resp, err := client.Create(context.TODO(), &pb.User{
		Name:     name,
		Email:    email,
		Password: password,
		Company:  company,
	})
	if err != nil {
		log.Fatalf("call Create error: %v", err)
	}
	log.Println("created: ", resp.User.Id)

	allResp, err := client.GetAll(context.Background(), &pb.Request{})
	if err != nil {
		log.Fatalf("call GetAll error: %v", err)
	}
	for _, u := range allResp.Users {
		log.Printf("%v\n", u)
	}

	authResp, err := client.Auth(context.TODO(), &pb.User{
		Email:    email,
		Password: password,
	})
	if err != nil {
		log.Fatalf("auth failed: %v", err)
	}
	log.Println("token: ", authResp.Token)

	// 直接退出即可
	os.Exit(0)
}
```

对 user-service 和 user-cli 都重新进行 `make build` 后，生成 token 的效果如下：

![4.1](http://p7f8yck57.bkt.clouddn.com/2018-05-28-003642.gif)

把这个 token 保存下来，后边有用。



### token 验证

现在为用户生成 token 的逻辑已经有了，可将其使用在 consignment-service 微服务中了，使 consignment-cli 与 consignment-service 之间通过 token 来识别用户。

```go
// consignment-cli/cli.go
// ...

func main() {

	cmd.Init()
	// 创建微服务的客户端，简化了手动 Dial 连接服务端的步骤
	client := pb.NewShippingServiceClient("go.micro.srv.consignment", microclient.DefaultClient)

	// 在命令行中指定新的货物信息 json 件
	if len(os.Args) < 3 {
		log.Fatalln("Not enough arguments, expecing file and token.")
	}
	infoFile := os.Args[1]
	token := os.Args[2]

	// 解析货物信息
	consignment, err := parseFile(infoFile)
	if err != nil {
		log.Fatalf("parse info file error: %v", err)
	}

	// 创建带有用户 token 的 context
	// consignment-service 服务端将从中取出 token，解密取出用户身份
	tokenContext := metadata.NewContext(context.Background(), map[string]string{
		"token": token,
	})

	// 调用 RPC
	// 将货物存储到指定用户的仓库里
	resp, err := client.CreateConsignment(tokenContext, consignment)
	if err != nil {
		log.Fatalf("create consignment error: %v", err)
	}
	log.Printf("created: %t", resp.Created)

	// 列出目前所有托运的货物
	resp, err = client.GetConsignments(tokenContext, &pb.GetRequest{})
	if err != nil {
		log.Fatalf("failed to list consignments: %v", err)
	}
	for i, c := range resp.Consignments {
		log.Printf("consignment_%d: %v\n", i, c)
	}
}
```



修改 consignment-service 监听请求，从中取出 token 并调用 user-service 进行验证：

```go
package main
import (...)

const (
	DEFAULT_HOST = "127.0.0.1:27017"
)


func main() {

	// ...
	srv := micro.NewService(
		// 必须和 consignment.proto 中的 package 一致
		micro.Name("go.micro.srv.consignment"),
		micro.Version("latest"),
		micro.WrapHandler(AuthWrapper),
	)
	// ...
}

// AuthWrapper 是一个高阶函数，入参是 ”下一步“ 函数，出参是认证函数
// 在返回的函数内部处理完认证逻辑后，再手动调用 fn() 进行下一步处理
// token 是从 consignment-ci 上下文中取出的，再调用 user-service 将其做验证
// 认证通过则 fn() 继续执行，否则报错
func AuthWrapper(fn server.HandlerFunc) server.HandlerFunc {
	return func(ctx context.Context, req server.Request, resp interface{}) error {
		meta, ok := metadata.FromContext(ctx)
		if !ok {
			return errors.New("no auth meta-data found in request")
		}

		// Note this is now uppercase (not entirely sure why this is...)
		token := meta["Token"]

		// Auth here
		authClient := userPb.NewUserServiceClient("go.micro.srv.user", client.DefaultClient)
		authResp, err := authClient.ValidateToken(context.Background(), &userPb.Token{
			Token: token,
		})
		log.Println("Auth Resp:", authResp)
		if err != nil {
			return err
		}
		err = fn(ctx, req, resp)
		return err
	}
}
```

分别在 consignment-service 和 consignment-cli 下均重新 `make build` 使修改生效。重新构建镜像：

```shell
$ docker run --net="host" \
	-e MICRO_REGISTRY=mdns \
	consignment-cli consignment.json \
	<TOKEN_HERE>
```

这里使用了 `--net="host"` 来指定容器运行在宿主主机的本地网络，比如 127.0.0.1 或 localhost，而非 docker 自己的内部网络。如此便不再需要再进行端口映射，直接使用 `-p 8080` 代替 `-p 8080:8080` 即可。更多参考：[Docker 手册](https://docs.docker.com/engine/userguide/networking/)

现在运行上述命令，将看到创建了新的货物托运，效果如下：

![4.2](http://p7f8yck57.bkt.clouddn.com/2018-05-28-021533.gif)

如果删掉 token 中的几个字符，将会收到 `illegal base64 data at input byte 41` 类似的认证错误。

到目前为止，我们在 user-service 中实现了 JWT 的加解密，并在 consignment-cli 与 consignment-service 之间作为中间层认证用户使用。整个运行流程如下：

![image-20180528105549866](http://p7f8yck57.bkt.clouddn.com/2018-05-28-025550.png)

所以在运行之前，要把 user-service 和 vessel-service 同样运行起来。在 MongoDB 中也可看到货轮与货物数据存储成功：

![image-20180528105927785](http://p7f8yck57.bkt.clouddn.com/2018-05-28-025928.png)



### gRPC 实现

如果你还在用 gRPC 而不是 go-mirco，那你的认证中间件应该如下实现，搞复杂了：

```go
func main() {
    ... 
    myServer := grpc.NewServer(
        grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(AuthInterceptor),
    )
    ... 
}

func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

    // Set up a connection to the server.
    conn, err := grpc.Dial(authAddress, grpc.WithInsecure())
    if err != nil {
        log.Fatalf("did not connect: %v", err)
    }
    defer conn.Close()
    c := pb.NewAuthClient(conn)
    r, err := c.ValidateToken(ctx, &pb.ValidateToken{Token: token})

    if err != nil {
	    log.Fatalf("could not authenticate: %v", err)
    }

    return handler(ctx, req)
}
```



### 调用切换

此外，我们不需要在本地把所有微服务都运行起来，微服务之间应该相互独立，能在隔离的环境中进行测试。比如在当前的项目中，如果只要测试 consignment-service 自己的 RPC，那就没有必要调用 user-service 做认证，我觉得在代码能切换是否调用其他服务的做法是比较好的。

更新 consignment-service 的认证中间件：

```go
// shippy-user-service/main.go
...
func AuthWrapper(fn server.HandlerFunc) server.HandlerFunc {
	return func(ctx context.Context, req server.Request, resp interface{}) error {
		// consignment-service 独立测试时不进行认证
		if os.Getenv("DISABLE_AUTH") == "true" {
			return fn(ctx, req, resp)
		}
		...
	}
}
```

在我们的 Makefile 中就可设置：

```makefile
// shippy-user-service/Makefile
...
run:
	docker run -d --net="host" \
		-p 50052 \
		-e MICRO_SERVER_ADDRESS=:50052 \
		-e MICRO_REGISTRY=mdns \
		-e DISABLE_AUTH=true \
		consignment-service
```

使用环境变量来决定是否调用其他服务，从而使你的微服务能隔离的进行测试，我认为这么做最为简单。





## 总结

本节对密码进行哈希处理后存储，并引入 JWT 进行了用户数据的加解密，使用其生成的 token 在微服务之间做用户身份的验证。从第一节到第四节，我们把 consignment-service、vessel-service、user-service 三个微服务作为一个完整系统实现了，完成了货物管理系统的基本功能，后续章节进行完善。下节将使用 go-micro 的 NATS 插件进行消息发布。















  























