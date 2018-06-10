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

	public function __construct(TrafficTool $tool) {
		$this->trafficTool = $tool;
	}

	public function travel() {
		$this->trafficTool->go();
	}
}


$train = new Train();
$me    = new Traveler($train);
$me->travel();