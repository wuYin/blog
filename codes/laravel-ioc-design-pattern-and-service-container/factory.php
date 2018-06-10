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
		$factory = new TrafficToolFactory();
		$this->travelTool = $factory->create($toolName);
	}

	public function travel() {
		$this->travelTool->go();
	}
}

$me = new Traveler('train');
$me->travel();