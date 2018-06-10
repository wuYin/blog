---
title: IOC 模式解耦与 Laravel 服务容器
date: 2018-06-10 14:58:04
tags: [PHP, Laravel]
---

简要介绍 IOC 模式进行代码解耦及其在 Laravel 服务容器中的应用。

本文代码：[GitHub](https://github.com/wuYin/blog/tree/master/codes/laravel-ioc-design-pattern-and-service-container/)

<!-- more -->

## 前言

服务容器是 Laravel 框架实现模块化解耦的核心。模块化即是将系统拆成多个子模块，子模块间的耦合程度尽可能的低，代码中尽可能的避免直接调用。这样才能提高系统的代码重用性、可维护性、扩展性。

下边出行例子有火车、飞机两种出行方式，对应给出了 3 种耦合度越来越低的实现：高度耦合实现、工厂模式解耦、IOC 模式解耦。

## 高度耦合实现

### 代码实现

定义 TrafficTool 接口并用 Train、Plane 实现，最后在 Traveler 中实例化出行工具后说走就走。代码十分简洁：

```php
<?php

// 定义交通工具接口
interface TrafficTool
{
	public function go();
}

class Train implements TrafficTool
{
	public function go() {
		echo '[Travel By]: train', PHP_EOL;
	}
}

class Plane implements TrafficTool
{
	public function go() {
		echo '[Travel By]: plane', PHP_EOL;
	}
}


// 旅游者类，使用火车出行
class Traveler
{
	protected $trafficTool;

	public function __construct() {
		// 直接 new 对象，在 Traveler 与 Train 两个类之间产生了依赖
		// 如果程序内部要修改出行方式，必须修改 Traveler 的 __construct()
		// 代码高度耦合，可维护性低
		$this->travelTool = new Train();
	}

	public function travel() {
		$this->travelTool->go();
	}
}


$me = new Traveler();
$me->travel();
```

运行：

```shell
$ php normal.php
[Travel By]: train
```



### 优点

代码十分简洁：一个接口两个类最后直接调用。

### 缺点

在第 32 行，`Traveler` 与 `Train` 两个组件发生了耦合。以后想坐飞机出行，必须修改 `__construct()` 的内部实现：`$this->travelTool = new Plane();`

重用性和可维护性都很差：在实际的软件开发中，代码会根据业务需求的变化而不断修改。如果组件之间直接相互调用，那组件的代码就不能轻易修改，以免调用它的地方出现错误。





## 工厂模式解耦

### 工厂模式

分离代码中不变和变的部分，使得在不同条件下创建不同的对象。

### 代码实现

```php
...

class  TrafficToolFactory
{
	public function create($name) {
		switch ($name) {
			case 'train':
				return new Train();
			case 'plane':
				return new Plane();
			default:
				exit('[No Traffic Tool] :' . $name);
		}
	}
}


// 旅游者类，使用火车出行
class Traveler
{
	protected $trafficTool;

	public function __construct($toolName) {
		// 使用工厂类实例化需要的交通工具
		$factory = new TrafficToolFactory();
		$this->travelTool = $factory->create($toolName);
	}

	public function travel() {
		$this->travelTool->go();
	}
}

// 传入指定的方式
$me = new Traveler('train');
$me->travel();
```

运行：

```shell
$ php factory.php
[Travel By]: train
```



### 优点

提取了代码中变化的部分：更换交通工具，坐飞机出行直接修改 `$me = new Traveler('plane')` 即可。适用于需求简单的情况。

### 缺点

依旧没有彻底解决依赖：现在 `Traveler` 与 `TrafficToolFactory` 发生了依赖。当需求增多后，工厂的 `switch...case` 等代码也不易维护。





## IOC 模式解耦

IOC 是 Inversion Of Controll 的缩写，即控制反转。这里的“反转”可理解为将组件间依赖关系提到外部管理。

### 简单的依赖注入

依赖注入是 IOC 的一种实现方式，是指组件间的依赖通过外部参数（interface）形式直接注入。比如对上边的工厂模式进一步解耦：

```php
<?php

interface TrafficTool
{
	public function go();
}

class Train implements TrafficTool
{
	public function go() {
		echo '[Travel By]: train', PHP_EOL;
	}
}

class Plane implements TrafficTool
{
	public function go() {
		echo '[Travel By]: plane', PHP_EOL;
	}
}


class Traveler
{
	protected $trafficTool;

	// 参数 $tool 就是控制反转要反转部分，将依赖的对象直接传入即可
	// 以后再有 Car, GetWay ... 等新增工具也是实例化后传参直接调用
	public function __construct(TrafficTool $tool) {
		$this->trafficTool = $tool;
	}

	public function travel() {
		$this->trafficTool->go();
	}
}

$train = new Train();
$me    = new Traveler($train);	// 将依赖直接以参数的形式注入
$me->travel();
```

运行：

```shell
$ php simple_ioc.php
[Travel By]: train
```



### 高级依赖注入

#### 简单注入的问题

如果三个人分别自驾游、坐飞机、高铁出去玩，那你的代码可能是这样的：

```php
$train = new Train();
$plane = new Plane();
$car   = new Car();

$a = new Traveler($car);
$b = new Traveler($plane);
$c = new Traveler($train);

$a->travel();
$b->travel();
$c->travel();
```

看起来就两个字：蓝瘦。上边简单的依赖注入相比工厂模式已经解耦挺多了，参考 Laravel 中服务容器的概念，还能继续解耦。将会使用到 PHP 反射和匿名函数，参考：[Laravel 框架中常用的 PHP 语法](https://wuyin.io/2018/06/09/php-common-features-used-in-laravel/)

#### IOC 容器

高级依赖注入 = 简单依赖注入 + IOC 容器

```php
<?php
# advanced_ioc.php    
...
    
class Container
{
	protected $binds = [];
	protected $instances = [];

	/**
	 * 绑定：将回调函数绑定到字符指令上
	 *
	 * @param $abstract 字符指令，如 'train'
	 * @param $concrete 用于实例化组件的回调函数，如 function() { return new Train(); }
	 */
	public function bind($abstract, $concrete) {
		if ($concrete instanceof Closure) {
			// 向容器中添加可以执行的回调函数
			$this->binds[$abstract] = $concrete;
		} else {
			$this->instances[$abstract] = $concrete;
		}
	}

	/**
	 * 生产：执行回调函数
	 *
	 * @param $abstract     字符指令
	 * @param array $params 回调函数所需参数
	 * @return mixed        回调函数的返回值
	 */
	public function make($abstract, $params = []) {
		if (isset($this->instances[$abstract])) {
			return $this->instances[$abstract];
		}

		// 此时 $this 是有 2 个元素的数组
		// Array (
		//     [0] => Container Object (
		//                [binds] => Array ( ... )
		//                [instances] => Array()
		//            )
		//     [1] => "train"
		// )
		array_unshift($params, $this);

		// 将参数传递给回调函数
		return call_user_func_array($this->binds[$abstract], $params);
	}
}

$container = new Container();
$container->bind('traveler', function ($container, $trafficTool) {
	return new Traveler($container->make($trafficTool));
});

$container->bind('train', function ($container) {
	return new Train();
});

$container->bind('plane', function ($container) {
	return new Plane();
});

$me = $container->make('traveler', ['train']);
$me->travel();
```

运行：

```shell
$ php advanced_ioc.php
[Travel By]: train
```



#### 简化并解耦后的代码

那三个人再出去玩，代码将简化为：

```php
$a = $container->make('traveler', ['car']);
$b = $container->make('traveler', ['train']);
$c = $container->make('traveler', ['plane']);

$a->travel();
$b->travel();
$c->travel();
```

更多参考：[神奇的服务容器](https://laravel-china.org/topics/789/laravel-learning-notes-the-magic-of-the-service-container)



## Laravel 的服务容器

Laravel 自己的服务容器是一个更加高级的 IOC 容器，它的简化代码如下：

```php
<?php
# laravel_ioc.php    
...
    

class Container
{
	// 绑定回调函数
	public $binds = [];

	// 绑定接口 $abstract 与回调函数
	public function bind($abstract, $concrete = null, $shared = false) {
		if (!$concrete instanceof Closure) {
			$concrete = $this->getClosure($abstract, $concrete);
		}
		$this->binds[$abstract] = compact('concrete', 'shared');
	}

	// 获取回调函数
	public function getClosure($abstract, $concrete) {
		return function ($container) use ($abstract, $concrete) {
			$method = ($abstract == $concrete) ? 'build' : 'make';
			return $container->$method($concrete);
		};
	}

	protected function getConcrete($abstract) {
		if (!isset($this->binds[$abstract])) {
			return $abstract;
		}
		return $this->binds[$abstract]['concrete'];
	}


	// 生成实例对象
	public function make($abstract) {
		$concrete = $this->getConcrete($abstract);
		if ($this->isBuildable($abstract, $concrete)) {
			$obj = $this->build($concrete);
		} else {
			$obj = $this->make($concrete);
		}
		return $obj;
	}


	// 判断是否要用反射来实例化
	protected function isBuildable($abstract, $concrete) {
		return $concrete == $abstract || $concrete instanceof Closure;
	}

	// 通过反射来实例化 $concrete 的对象
	public function build($concrete) {
		if ($concrete instanceof Closure) {
			return $concrete($this);
		}
		$reflector = new ReflectionClass($concrete);
		if (!$reflector->isInstantiable()) {
			echo "[can't instantiable]: " . $concrete;
		}

		$constructor = $reflector->getConstructor();
		// 使用默认的构造函数
		if (is_null($constructor)) {
			return new $concrete;
		}

		$refParams = $constructor->getParameters();
		$instances = $this->getDependencies($refParams);
		return $reflector->newInstanceArgs($instances);
	}


	// 获取实例化对象时所需的参数
	public function getDependencies($refParams) {
		$deps = [];
		foreach ($refParams as $refParam) {
			$dep = $refParam->getClass();
			if (is_null($dep)) {
				$deps[] = null;
			} else {
				$deps[] = $this->resolveClass($refParam);
			}
		}
		return (array)$deps;
	}

	// 获取参数的类型类名字
	public function resolveClass(ReflectionParameter $refParam) {
		return $this->make($refParam->getClass()->name);
	}
}


$container = new Container();

// 将 traveller 对接到 Train 
$container->bind('TrafficTool', 'Train');
$container->bind('traveller', 'Traveller');

// 创建 traveller 实例
$me = $container->make('traveller');
$me->travel();
```

运行：

```shell
$ php laravel_ioc.php     
[Travel By]: train
```

Train 类要能被实例化，需要先注册到容器，这就涉及到 Laravel 中服务提供者（Service Provider）的概念了。至于服务提供者是怎么注册类、注册之后如何实例化、实例化后如何调用的... 下节详细分析。



## 总结

本文用一个旅游出行的 demo，引出了高度耦合的直接实现、工厂模式解耦和 IOC 模式解耦共计三种实现方式，越往后代码量越多还有些绕，但类（模块）之间的耦合度越来越低，最后实现了简化版的 Laravel 服务容器。

Laravel 的优美得益于开发的组件式解耦，这与服务容器和服务提供者的理念是离不开的，下篇将用 Laravel 框架 `laravel/framework/src/Illuminate/Container.php` 中 `Container` 类来梳理 Laravel 服务容器的工作流程。