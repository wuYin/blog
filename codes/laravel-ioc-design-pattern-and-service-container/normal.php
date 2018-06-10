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