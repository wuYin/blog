<?php

interface TrafficTool
{
	public function go();
}

class Traveller
{
	protected $trafficTool;

	public function __construct(TrafficTool $tool) {
		$this->trafficTool = $tool;
	}

	public function travel() {
		$this->trafficTool->go();
	}
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