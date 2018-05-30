---
title: Golang 微服务教程（六）
date: 2018-05-30 15:35:03
tags: 微服务
---

原文链接：[ewanvalentine.io](https://ewanvalentine.io/microservices-in-golang-part-6)，翻译已获作者 [Ewan Valentine ](https://twitter.com/Ewan_Valentine)授权。

本文完整代码：[GitHub](https://github.com/wuYin/shippy/tree/feature/part6)

<!-- more -->

在上节中我们使用 go-micro 搭建了微服务的事件驱动架构。本节将揭晓从 web 客户端的角度出发如何与微服务进行调用交互。



## 微服务与 web 端交互

参考 [go-micro 文档](https://github.com/micro/micro)，可看到 go-micro 实现了为 web 客户端代理请求 RPC 方法的机制。

### 内部调用

微服务 A 调用微服务 B 的方法，需要先实例化再调用：`bClient.CallRPC(args...)`，数据作为参数传递，属于**内部调用**。

### 外部调用

web 端浏览器是通过 HTTP 请求去调用微服务的方法，go-micro 就做了中间层，调用方法的数据以 HTTP 请求的方式提交，属于**外部调用**。



## REST vs RPC

[REST](https://zh.wikipedia.org/zh-hans/%E8%A1%A8%E7%8E%B0%E5%B1%82%E7%8A%B6%E6%80%81%E8%BD%AC%E6%8D%A2) 风格多年来在 web 开发领域独领风骚，常用于客户端与服务端进行资源管理，应用场景比 RPC 和 SOAP 都要广得多。更多参考：[知乎：RPC 与 RESTful API 对比](https://www.zhihu.com/question/28570307)

### REST

REST 风格对资源的管理既简单又规范，它将 HTTP 请求方法对应到资源的增删改查上，同时还可以使用 HTTP 错误码来描述响应状态，在大多数 web 开发中 REST 都是优秀的解决方案。

### RPC

不过近年来 RPC 风格也乘着微服务的顺风车逐渐普及开来。REST 适合同时管理不同的资源，不过一般微服务只专注管理单一的资源，使用 RPC 风格能让 web 开发专注于各微服务的实现与交互。



## Micro 工具箱

我们从第二节开始就一直在使用 go-micro 框架，现在来看看它的 API 网关。go-micro 提供 API 网关给微服务做代理。API 网关把微服务 RPC 方法代理成 web 请求，将 web 端使用到的 URL 开放出来，更多参考：[go-micro toolkits](https://micro.mu/blog/2016/03/20/micro.html)，[go-micro API example](https://github.com/micro/examples/tree/master/greeter)

### 使用

```shell
# 安装 go-micro 的工具箱：
# $ go get -u github.com/micro/micro

# 我们直接使用它的 Docker 镜像
$ docker pull microhq/micro
```



现在修改一下 user-service 的代码：

```go
package main

import (
	"log"
	pb "shippy/user-service/proto/user"		// 作者用的另一个仓库
	"github.com/micro/go-micro"
)

func main() {
	// 连接到数据库
	db, err := CreateConnection()
	defer db.Close()

	if err != nil {
		log.Fatalf("connect error: %v\n", err)
	}

	repo := &UserRepository{db}

	// 自动检查 User 结构是否变化
	db.AutoMigrate(&pb.User{})

	// 作者使用了新仓库 shippy-user-service
	// 但 auth.proto 和 user.proto 定义的内容是一致的
	// 修改 shippy.auth 为 go.micro.srv.user 即可
	// 注意 API 调用参数也需对应修改
	srv := micro.NewService(
		micro.Name("go.micro.srv.user"),
		micro.Version("latest"),
	)

	srv.Init()

	// 获取 broker 实例
	// pubSub := s.Server().Options().Broker
	publisher := micro.NewPublisher(topic, srv.Client())

	t := TokenService{repo}
	pb.RegisterUserServiceHandler(srv.Server(), &handler{repo, &t, publisher})

	if err := srv.Run(); err != nil {
		log.Fatalf("user service error: %v\n", err)
	}
}
```

原代码仓库：[shippy-user-service/tree/tutorial-6](https://github.com/EwanValentine/shippy-user-service/tree/tutorial-6)

### API 网关

现在把 user-service 和 emil-service 像上节一样 `make run` 运行起来。之后再执行：

```shell
$ docker run -p 8080:8080 \ 
             -e MICRO_REGISTRY=mdns \
             microhq/micro api \
             --handler=rpc \
             --address=:8080 \
             --namespace=shippy
```

API 网关现在运行在 8080 端口，同时告诉它和其他微服务一样使用 mdns 做服务发现，最后使用的命名空间是 shippy，它会作为我们服务名的前缀，比如 `shippy.auth`，`shippy.email`，默认值是 `go.micro.api`，如果不指定而使用默认值将无法生效。

#### web 端创建用户

现在外部可以像这样调用 user-service 创建用户的方法：

```shell
$ curl -XPOST -H 'Content-Type: application/json' \
    -d '{
            "service": "shippy.auth",
            "method": "Auth.Create",
            "request": {
                "user": {
                    "email": "ewan.valentine89@gmail.com",
                    "password": "testing123",
                    "name": "Ewan Valentine",
                    "company": "BBC"
                }
            }
	}' \ 
    http://localhost:8080/rpc
```

效果如下：

![image-20180530160604137](http://p7f8yck57.bkt.clouddn.com/2018-05-30-080604.png)

在这个 HTTP 请求中，我们把 user-service  `Create` 方法所需参数以 JSON 字段值的形式给出，API 网关会帮我们自动调用，并同样以 JSON 格式返回方法的处理结果。

#### web 端认证用户

```shell
$ curl -XPOST -H 'Content-Type: application/json' \ 
    -d '{
            "service": "shippy.auth",
            "method": "Auth.Auth",
            "request": {
                "email": "your@email.com",
                "password": "SomePass"
            }
	}' \
    http://localhost:8080/rpc
```

运行效果如下：

![image-20180530200732645](http://p7f8yck57.bkt.clouddn.com/2018-05-30-120733.png)





## 用户界面

现在将上边的 API 做成 web 端调用，我们这里使用 React 的 `react-create-app` 库。先安装：`$ npm install -g react-create-app`，最后创建项目：`$ react-create-app shippy-ui`

### App.js 

```js
// shippy-ui/src/App.js
import React, { Component } from 'react';
import './App.css';
import CreateConsignment from './CreateConsignment';
import Authenticate from './Authenticate';

class App extends Component {

  state = {
    err: null,
    authenticated: false,
  }

  onAuth = (token) => {
    this.setState({
      authenticated: true,
    });
  }

  renderLogin = () => {
    return (
      <Authenticate onAuth={this.onAuth} />
    );
  }

  renderAuthenticated = () => {
    return (
      <CreateConsignment />
    );
  }

  getToken = () => {
    return localStorage.getItem('token') || false;
  }

  isAuthenticated = () => {
    return this.state.authenticated || this.getToken() || false;
  }

  render() {
    const authenticated = this.isAuthenticated();
    return (
      <div className="App">
        <div className="App-header">
          <h2>Shippy</h2>
        </div>
        <div className='App-intro container'>
          {(authenticated ? this.renderAuthenticated() : this.renderLogin())}
        </div>
      </div>
    );
  }
}

export default App;
```

接下来添加用户认证、货物托运的两个组件。



### Authenticate 用户认证组件

```js
// shippy-ui/src/Authenticate.js
import React from 'react';

class Authenticate extends React.Component {

  constructor(props) {
    super(props);
  }

  state = {
    authenticated: false,
    email: '',
    password: '',
    err: '',
  }

  login = () => {
    fetch(`http://localhost:8080/rpc`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        request: {
          email: this.state.email,
          password: this.state.password,
        },
        // 注意
        // 作者的把 Auth 认证作为了独立的项目
        // Auth 其实和 go.micro.srv.user 是一样的
        // 这里作者和译者的代码略有不同
        service: 'shippy.auth',
        method: 'Auth.Auth',
      }),
    })
    .then(res => res.json())
    .then(res => {
      this.props.onAuth(res.token);
      this.setState({
        token: res.token,
        authenticated: true,
      });
    })
    .catch(err => this.setState({ err, authenticated: false, }));
  }

  signup = () => {
    fetch(`http://localhost:8080/rpc`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        request: {
          email: this.state.email,
          password: this.state.password,
          name: this.state.name,
        },
        method: 'Auth.Create',
        service: 'shippy.auth',
      }),
    })
    .then((res) => res.json())
    .then((res) => {
      this.props.onAuth(res.token.token);
      this.setState({
        token: res.token.token,
        authenticated: true,
      });
      localStorage.setItem('token', res.token.token);
    })
    .catch(err => this.setState({ err, authenticated: false, }));
  }

  setEmail = e => {
    this.setState({
      email: e.target.value,
    });
  }

  setPassword = e => {
    this.setState({
      password: e.target.value,
    });
  }

  setName = e => {
    this.setState({
      name: e.target.value,
    });
  }

  render() {
    return (
      <div className='Authenticate'>
        <div className='Login'>
          <div className='form-group'>
            <input
              type="email"
              onChange={this.setEmail}
              placeholder='E-Mail'
              className='form-control' />
          </div>
          <div className='form-group'>
            <input
              type="password"
              onChange={this.setPassword}
              placeholder='Password'
              className='form-control' />
          </div>
          <button className='btn btn-primary' onClick={this.login}>Login</button>
          <br /><br />
        </div>
        <div className='Sign-up'>
          <div className='form-group'>
            <input
              type='input'
              onChange={this.setName}
              placeholder='Name'
              className='form-control' />
          </div>
          <div className='form-group'>
            <input
              type='email'
              onChange={this.setEmail}
              placeholder='E-Mail'
              className='form-control' />
          </div>
          <div className='form-group'>
            <input
              type='password'
              onChange={this.setPassword}
              placeholder='Password'
              className='form-control' />
          </div>
          <button className='btn btn-primary' onClick={this.signup}>Sign-up</button>
        </div>
      </div>
    );
  }
}

export default Authenticate;
```



### CreateConsignment 货物托运组件

```js
// shippy-ui/src/CreateConsignment.js
import React from 'react';
import _ from 'lodash';

class CreateConsignment extends React.Component {

  constructor(props) {
    super(props);
  }

  state = {
    created: false,
    description: '',
    weight: 0,
    containers: [],
    consignments: [],
  }

  componentWillMount() {
    fetch(`http://localhost:8080/rpc`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        service: 'shippy.consignment',
        method: 'ConsignmentService.Get',
        request: {},
      })
    })
    .then(req => req.json())
    .then((res) => {
      this.setState({
        consignments: res.consignments,
      });
    });
  }

  create = () => {
    const consignment = this.state;
    fetch(`http://localhost:8080/rpc`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        service: 'shippy.consignment',
        method: 'ConsignmentService.Create',
        request: _.omit(consignment, 'created', 'consignments'),
      }),
    })
    .then((res) => res.json())
    .then((res) => {
      this.setState({
        created: res.created,
        consignments: [...this.state.consignments, consignment],
      });
    });
  }

  addContainer = e => {
    this.setState({
      containers: [...this.state.containers, e.target.value],
    });
  }

  setDescription = e => {
    this.setState({
      description: e.target.value,
    });
  }

  setWeight = e => {
    this.setState({
      weight: Number(e.target.value),
    });
  }

  render() {
    const { consignments, } = this.state;
    return (
      <div className='consignment-screen'>
        <div className='consignment-form container'>
          <br />
          <div className='form-group'>
            <textarea onChange={this.setDescription} className='form-control' placeholder='Description'></textarea>
          </div>
          <div className='form-group'>
            <input onChange={this.setWeight} type='number' placeholder='Weight' className='form-control' />
          </div>
          <div className='form-control'>
            Add containers...
          </div>
          <br />
          <button onClick={this.create} className='btn btn-primary'>Create</button>
          <br />
          <hr />
        </div>
        {(consignments && consignments.length > 0
          ? <div className='consignment-list'>
              <h2>Consignments</h2>
              {consignments.map((item) => (
                <div>
                  <p>Vessel id: {item.vessel_id}</p>
                  <p>Consignment id: {item.id}</p>
                  <p>Description: {item.description}</p>
                  <p>Weight: {item.weight}</p>
                  <hr />
                </div>
              ))}
            </div>
          : false)}
      </div>
    );
  }
}

export default CreateConsignment;
```

UI 的完整代码可见：[shippy-ui](https://github.com/EwanValentine/shippy-ui)

现在执行 `npm start`，效果如下：

![image-20180530180558510](http://p7f8yck57.bkt.clouddn.com/2018-05-30-100558.png)



打开 Chrome 的 Application 能看到在注册或登录时，RPC 成功调用：

![image-20180530210325184](http://p7f8yck57.bkt.clouddn.com/2018-05-30-130325.png)





## 总结

本节使用 go-micro 自己的 API 网关，完成了 web 端对微服务函数的调用，可看出函数参数的入参出参都是以 JSON 给出的，对应于第一节 Protobuf 部分说与浏览器交互 JSON 是只选的。此外，作者代码与译者代码有少数出入，望读者注意，感谢。

下节我们将引入 Google Cloud 云平台来托管我们的微服务项目，并使用 Terraform 进行管理。