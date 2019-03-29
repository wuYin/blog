title: Laravel 5.5 入门指南
date: 2018-01-04 10:23:48
tags:

-   Laravel

---



细读 Laravel5.5 文档：安装与配置

<!-- more -->



#### 安装

##### 创建 Laravel 项目

-   laravel 可执行文件创建

    ```shell
    $ whereis laravel
    laravel: /home/vagrant/.composer/vendor/bin/laravel
    ```

-   composer  创建

    其中 `--prefer-dist` 会强制 composer 尽量使用各个扩展的压缩包，而不是再去 github 上拉取源码，加快安装速度。反之`--prefer-source` 会 clone 源码进行安装。

    ```shell
    $ composer create-project --prefer-dist laravel/laravel  project-name
    # create-project  Creates new project from a package into given directory.
    Installing laravel/laravel (v5.5.28)
      - Installing laravel/laravel (v5.5.28): Loading from cache

    $ ll /home/vagrant/.composer/cache/files/	# 压缩包目录
    laravel/
    phpunit/
    ...
    ```

##### 目录权限：保证日志、缓存目录可读写

```shell
$ tree -L 2 storage/ bootstrap/	# 以下文件夹在 Centos 上需手动创建并至少赋予 755 权限
storage/
├── app
│   └── public
├── framework
│   ├── cache
│   ├── sessions
│   ├── testing
│   └── views
└── logs

bootstrap/
└── cache
```

##### 应用密钥：用于加密 session 等，保证用于数据的安全

```shell
$ php artisan key:generate         # Set the application key
# --show # Print only the key (doesn't write the .env file)
```

##### 配置 web 服务器

```nginx
# nginx
# 将所有请求转发到 public/index.php 处理
location / {
    try_files $uri $uri/ /index.php?$query_string;
}
```



#### 配置

##### 环境配置

-   `.env` 文件

    不应提交给 VCS 跟踪，可以使用 `.env.example` 文件写明需自己配置的选项，且 `.env` 中的变量会被服务器级别的环境变量覆盖。

-   检查环境配置

    -   laravel 处理请求时，会将 `.env` 中的配置项加载到 `$_ENV` 全局变量中， `env($key, $default = null)` 方法取出配置项

    -   在不同主机环境上配置同一应用时，可使用 `APP_ENV` 来为不同的环境做不同的配置

        在本地开发、生产环境中 `APP_ENV` 的值可分别设置为 `local` `production` , 可通过 `App::environment()` 来获取环境位置。

-   获取和动态配置选项 

    ````php
    config('links.GOOGLE');		// 获取 config/links.php 中数组键值为 GOOGLE 的值
    config(['app.timezone' => 'Aisa/Shanghai']);	// 动态设置配置
    ````

##### 配置缓存

在不易变动配置的环境如生产环境中，可使用 `php artisan config:cache` 将所有配置文件合并为 `bootstrap/cache/config.php` 一个配置文件，减少运行时文件载入数量。

```
config:cache         Create a cache file for faster configuration loading
config:clear         Remove the configuration cache file
```

##### 维护模式

```shell
down                 Put the application into maintenance mode
up                   Bring the application out of maintenance mode

$ php artisan down --message="System Optimizing..."  --retry=60	
# http header Retry-After 设置为 60s
# response 503 (Service Unavailable)
Application is now in maintenance mode.
# 维护模式页面：resources/views/errors/503.blade.php 
```

注意：Laravel 的维护模式会导致 queue  的暂停处理，且伴随短时间的不响应（解决方案：[envoyer:Zero Downtime PHP Deployment](https://envoyer.io/)）



#### 文件夹结构

##### 根目录

```shell
laravel-project $ tree -L 1
.
├── app 			# 应用程序的核心代码
├── bootstrap		# 包含框架自动加载的文件，可使用配置缓存等优化加载性能
├── config			# 所有的配置文件
├── database		# 数据迁移脚本文件
├── public			# 入口文件 index.php 和静态资源文件
├── resources		# View 文件、语言文件等
├── routes			# 路由文件目录
├── server.php
└── storage			# 缓存文件目录
	├── app			# 存储任意生成的文件
	├── framework	# 存储框架生成的缓存
	└── logs		# 存储应用的日志如 laravel.log 等
├── tests	 		# PHPUnit 单元测试文件
└──  vendor			# Composer 依赖包目录，一般都添加到 .gitignore
```


##### App 目录

App 目录是应用的核心代码目录，很多 class 都可以使用 `php artisan make:command` 来生成，常用的有：

```
make:controller    Create a new controller class
make:job           Create a new job class
make:migration     Create a new migration file
make:model         Create a new Eloquent model class
```

目录结构：

```shell
app $ tree -L 2
.
├── Console
│   └── Kernel.php 	# 定义计划任务
├── Exceptions		# 应用的异常处理器
├── helpers.php		# 可选的自定义全局函数类
├── Http			# 控制器、中间件和表单请求
├── Jobs			# 存放队列任务类
└── Providers		# 包含应用的所有服务提供器
```

###### 说明

App目录下默认没有 Models 目录，使用 `make:model` 默认直接在 app/ 下生成文件，在自己参与开发的项目中，有 44 个 Model 文件，app 文件夹显得十分臃肿。

可创建 `app/Models` 目录，并修改模型文件的命名空间： `namespace App\Models` ，即可。

```
app $ ll | grep .php | wc -l
44
```

