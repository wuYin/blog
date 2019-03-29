---
title: Laravel 框架中常用的 PHP 语法
date: 2018-06-09 17:04:55
tags: [Laravel, PHP]
---

Laravel 框架大量运用了 PHP 的特性与新语法，本文章简述一下。

<!-- more -->

## 前言

Laravel 框架因为其组件化的设计并恰当使用设计模式，使得框架本身简洁易扩展。区别于 ThinkPHP 那种整合式功能的框架（功能要么全用要么全不用），Laravel 使用 [composer](https://getcomposer.org/) 工具进行 package 的管理，想加功能直接添加组件即可。比如你写爬虫使用页面采集组件： `composer require jaeger/querylist`

本文简要介绍 Laravel 中频繁用到的 PHP 特性与新语法，具体可参考。



## 组件化开发

Laravel 进行组件化开发，得益于遵循 PSR-4 规范的 composer 工具，其利用命名空间和自动加载来组织项目文件。更多参考：[composer 自动加载机制](http://laravelacademy.org/post/7074.html)

### 命名空间

#### 命名冲突

在团队协作、引入第三方依赖代码时，往往可能会出现类、函数和接口重名的情况。比如：

```php
<?php	
# google.php
class User 
{
	private $name;
}
```

```php
<?php	
# mine.php
// 引入第三方依赖
include 'google.php';

class User
{
	private $name;
}

$user = new User();	// 命名冲突
```

因为同时定义了类 `User` 导致命名冲突：

![image-20180609193721860](http://p7f8yck57.bkt.clouddn.com/2018-06-09-113721.png)



#### 解决办法

从 PHP 5.3 开始引入，参考 [PHP 手册](https://secure.php.net/manual/zh/language.namespaces.rationale.php) 能知道命名空间有 2 个作用：避免**命名冲突**、保持**命名简短**。比如使用命名空间后：

```php
<?php
# google.php
namespace Google;

// 模拟第三方依赖
class User {
	private $name = 'google';

	public function getName() {
		echo $this->name . PHP_EOL;
	}
}
```

```php
<?php
# mine.php
namespace Mine;

// 导入并命名别名
use Google as G;

// 导入文件使得 google.php 命名空间变为 mine.php 的子命名空间
include 'google.php';

/* 避免了命名冲突 */
class User
{
	private $name = 'mine';

	public function getName() {
		echo $this->name . PHP_EOL;
	}
}

/* 保持了命名简短 */
// 如果没有命名空间，为了类名也不冲突，可能会出现这种函数名
// $user = new Google_User();
// Zend 风格并不提倡
$user = new G\User();

// 为了函数名也不冲突，可能会出现这种函数名
// $user->google_get_name()
$user->getName();

$user = new User();
$user->getName();
```

运行：

```shell
$ php demo.php
google
mine
```



#### PSR 规范

其实 namespace 与文件名无关，但按 PSR 标准要求：命名空间与文件路径一致 & 文件名与类名一致。比如 Laravel 默认生成的 `laravel-demo/app/Http/Controllers/Auth/LoginController.php`，其命名空间为 `App\Http\Controllers\Auth` & 类名为 `LoginController`

遵循规范，上边的 `mine.php` 和 `google.php` 都应叫 `User.php`



#### namespace 操作符与`__NAMESPACE__ ` 魔术常量

```php
...
// $user = new User();
$user = new namespace\User();	// 值为当前命名空间
$user->getName();

echo __NAMESPACE__ . PHP_EOL;	// 直接获取当前命名空间字符串	// 输出 Mine
```



#### 三种命名空间的导入

```php
<?php
namespace CurrentNameSpace;

// 不包含前缀
$user = new User();		# CurrentNameSpace\User();

// 指定前缀
$user = new Google\User();	# CurrentNameSpace\Google\User();

// 根前缀
$user = new \Google\User();	# \Google\User();
```



#### 全局命名空间

如果引用的类、函数没有指定命名空间，则会默认在当在` __NAMESPACE__`下寻找。若要引用全局类：

```php
<?php
namespace Demo;

// 均不会被使用到
function strlen() {}
const INI_ALL = 3;
class Exception {}

$a = \strlen('hi'); 		// 调用全局函数 strlen
$b = \CREDITS_GROUP; 	 	// 访问全局常量 CREDITS_GROUP
$c = new \Exception('error');   // 实例化全局类 Exception
```



#### 多重导入与多个命名空间

```php
// use 可一次导入多个命名空间
use Google,
	Microsoft;

// 良好实践：每行一个 use
use Google;
use Microsoft;
```

```php
<?php
// 一个文件可定义多个命名空间
namespace Google {
	class User {}
}    
    
namespace Microsoft {
	class User {}
}   

// 良好实践：“一个文件一个类”
```



#### 导入常量、函数

从 PHP 5.6 开始，可使用 `use function` 和 `use const` 分别导入函数和常量使用：

```php
# google.php
const CEO = 'Sundar Pichai';
function getMarketValue() {
	echo '770 billion dollars' . PHP_EOL;
}
```

```php
# mine.php
use function Google\getMarketValue as thirdMarketValue;
use const Google\CEO as third_CEO;

thirdMarketValue();
echo third_CEO;
```

运行：

```shell
$ php mine.php
google
770 billion dollars
Sundar Pichaimine
Mine
```



### 文件包含

#### 手动加载

使用 `include` 或 `require` 引入指定的文件，（字面理解）需注意 require 出错会报编译错误中断脚本运行，而 include 出错只会报 warning 脚本继续运行。

include 文件时，会先去 php.ini 中配置项 `include_path` 指定的目录找，找不到才在当前目录下找：

![image-20180609203210194](http://p7f8yck57.bkt.clouddn.com/2018-06-09-123210.png)

```php
<?php
    
// 引入的是 /usr/share/php/System.php
include 'System.php';
```



#### 自动加载

`void __autoload(string $class )`  能进行类的[自动加载](http://php.net/manual/zh/language.oop5.autoload.php)，但一般都使用 [spl_autoload_register](https://secure.php.net/manual/zh/function.spl-autoload-register.php) 手动进行注册：

```php
<?php

// 自动加载子目录 classes 下 *.class.php 的类定义
function __autoload($class) {
	include 'classes/' . $class . '.class.php';
}

// PHP 5.3 后直接使用匿名函数注册
$throw = true;		// 注册出错时是否抛出异常
$prepend = false;	// 是否将当前注册函数添加到队列头

spl_autoload_register(function ($class) {
    include 'classes/' . $class . '.class.php';
}, $throw, $prepend);
```

在 composer 生成的自动加载文件 `laravel-demo/vendor/composer/autoload_real.php`  中可看到：

```php
class ComposerAutoloaderInit8b41a
{
    private static $loader;

    public static function loadClassLoader($class)
    {
        if ('Composer\Autoload\ClassLoader' === $class) {
            // 加载当前目录下文件
            require __DIR__ . '/ClassLoader.php';
        }
    }
    
     public static function getLoader()
    {
        if (null !== self::$loader) {
            return self::$loader;
        }
	
        // 注册自己的加载器
        spl_autoload_register(array('ComposerAutoloaderInit8b41a6', 'loadClassLoader'), true, true);
        self::$loader = $loader = new \Composer\Autoload\ClassLoader();
        spl_autoload_unregister(array('ComposerAutoloaderInit8b41a6a', 'loadClassLoader'));

        ...
     }
 
    ...
}    
```

这里只提一下，具体 Laravel 整体是怎么做自动加载的，后边的文章会细说。



## 反射

参考 [PHP 手册](https://secure.php.net/manual/zh/book.reflection.php)，可简单的理解为在运行时获取对象的完整信息。反射有 5 个类：

```php
ReflectionClass 	// 解析类名
ReflectionProperty 	// 获取和设置类属性的信息(属性名和值、注释、访问权限)
ReflectionMethod 	// 获取和设置类函数的信息(函数名、注释、访问权限)、执行函数等
ReflectionParameter	// 获取函数的参数信息
ReflectionFunction	// 获取函数信息
```

比如 `ReflectionClass` 的使用：

```php
<?php

class User
{
	public $name;
	public $age;

	public function __construct($name = 'Laruence', $age = 35) {
		$this->name = $name;
		$this->age  = $age;
	}

	public function intro() {
		echo '[name]: ' . $this->name . PHP_EOL;
		echo '[age]: '  . $this->age  . PHP_EOL;
	}
}

reflect('User');

// ReflectionClass 反射类使用示例
function reflect($class) {
	try {
		$ref = new ReflectionClass($class);
		// 检查是否可实例化
		// interface、abstract class、 __construct() 为 private 的类均不可实例化
		if (!$ref->isInstantiable()) {
			echo "[can't instantiable]: ${class}\n";
		}

		// 输出属性列表
		// 还能获取方法列表、静态常量等信息，具体参考手册
		foreach ($ref->getProperties() as $attr) {
			echo $attr->getName() . PHP_EOL;
		}

		// 直接调用类中的方法，个人认为这是反射最好用的地方
		$obj = $ref->newInstanceArgs();
		$obj->intro();
	} catch (ReflectionException $e) {
        	// try catch 机制真的不优雅
        	// 相比之下 Golang 的错误处理虽然繁琐，但很简洁
		echo '[reflection exception: ]' . $e->getMessage();
	}
}
```

运行：

```shell
$ php reflect.php
name
age
[name]: Laruence
[age]: 35
```

其余 4 个反射类参考手册 demo 即可。





## 后期静态绑定

参考 [PHP 手册](https://secure.php.net/manual/zh/language.oop5.late-static-bindings.php)，先看一个例子：

```php
<?php

class Base
{
        // 后期绑定不局限于 static 方法
	public static function call() {
		echo '[called]: ' . __CLASS__ . PHP_EOL;
	}

	public static function test() {
		self::call();		// self   取值为 Base  直接调用本类中的函数
		static::call();		// static 取值为 Child 调用者
	}
}

class Child extends Base
{
	public static function call() {
		echo '[called]: ' . __CLASS__ . PHP_EOL;
	}
}


Child::test();
```

输出：

```shell
$ php late_static_bind.php
[called]: Base
[called]: Child
```

在对象实例化时，`self::` 会实例化根据定义所在的类，`static::` 会实例化调用它的类。



## trait

### 基本使用

参考 [PHP 手册](https://secure.php.net/manual/zh/language.oop5.traits.php)，PHP 虽然是单继承的，但从 5.4 后可通过 trait 水平组合“类”，来实现“类”的多重继承，其实就是把重复的函数拆分成 triat 放到不同的文件中，通过 use 关键字按需引入、组合。可类比 Golang 的 struct 填鸭式组合来实现继承。比如：

```php
<?php

class DemoLogger
{
	public function log($message, $level) {
		echo "[message]: $message", PHP_EOL;
		echo "[level]: $level", PHP_EOL;
	}
}

trait Loggable
{
	protected $logger;

	public function setLogger($logger) {
		$this->logger = $logger;
	}

	public function log($message, $level) {
		$this->logger->log($message, $level);
	}
}

class Foo
{
        // 直接引入 Loggable 的代码片段
	use Loggable;
}

$foo = new Foo;
$foo->setLogger(new DemoLogger);
$foo->log('trait works', 1);
```

运行：

```shell
$ php trait.php
[message]: trait works
[level]: 1
```

更多参考：[我所理解的 PHP Trait](https://overtrue.me/articles/2016/04/about-php-trait.html)



### 重要性质

#### 优先级

当前类的函数会覆盖 trait 的同名函数，trait 会覆盖父类的同名函数（ `use trait` 相当于当前类直接覆写了父类的同名函数）

#### trait 函数冲突

同时引入多个 trait 可用 `,` 隔开，即多重继承。

多个 trait 有同名函数时，引入将发生命名冲突，使用 `insteadof` 来指明使用哪个 trait 的函数。

#### 重命名与访问控制

使用 `as` 关键字可以重命名的 trait 中引入的函数，还可以修改其访问权限。

#### 其他

trait 类似于类，可以定义属性、方法、抽象方法、静态方法和静态属性。

下边的苹果、微软和 Linux 的小栗子来说明：

```php
<?php

trait Apple
{
	public function getCEO() {
		echo '[Apple CEO]: Tim Cook', PHP_EOL;
	}

	public function getMarketValue() {
		echo '[Apple Market Value]: 953 billion', PHP_EOL;
	}
}


trait MicroSoft
{
	public function getCEO() {
		echo '[MicroSoft CEO]: Satya Nadella', PHP_EOL;
	}

	public function getMarketValue() {
		echo '[MicroSoft Market Value]: 780 billion', PHP_EOL;
	}

	abstract public function MadeGreatOS();

	static public function staticFunc() {
		echo '[MicroSoft Static Function]', PHP_EOL;
	}

	public function staticValue() {
		static $v;
		$v++;
		echo '[MicroSoft Static Value]: ' . $v, PHP_EOL;
	}
}


// Apple 最终登顶，成为第一家市值超万亿美元的企业
trait Top
{
	// 处理引入的 trait 之间的冲突
	use Apple, MicroSoft {
		Apple::getCEO insteadof MicroSoft;
		Apple::getMarketValue insteadof MicroSoft;
	}
}


class Linux
{
	use Top {
        	// as 关键字可以重命名函数、修改权限控制
		getCEO as private noCEO;
	}

	// 引入后必须实现抽象方法
	public function MadeGreatOS() {
		echo '[Linux Already Made]', PHP_EOL;
	}

	public function getMarketValue() {
		echo '[Linux Market Value]: Infinity', PHP_EOL;
	}
}

$linux = new Linux();
// 和 extends 继承一样
// 当前类中的同名函数也会覆盖 trait 中的函数
$linux->getMarketValue();

// trait 中可以定义静态方法
$linux::staticFunc();

// 在 trait Top 中已解决过冲突，输出库克
$linux->getCEO();
// $linux->noCEO();		// Uncaught Error: Call to private method Linux::noCEO() 

// trait 中可以定义静态变量
$linux->staticValue();
$linux->staticValue();
```

运行：

```shell
$ php trait.php
[Linux Market Value]: Infinity
[MicroSoft Static Function]
[Apple CEO]: Tim Cook
[MicroSoft Static Value]: 1
[MicroSoft Static Value]: 2
```



## 总结

本节简要提及了命名空间、文件自动加载、反射机制与 trait 等，Laravel 正是恰如其分的利用了这些新特性，才实现了组件化开发、服务加载等优雅的特性。