---
title: B 站直播间数据爬虫
date: 2018-03-31 09:43:17
tags: PHP
---

使用 PHP 实现的 B 站直播间弹幕和礼物爬虫、弹幕分析与精彩时刻自动剪辑。

<!-- more -->

## 前言

### 起因

去年在 B 站发现一个后期超强的 UP 主：[修仙不倒大小眼](https://space.bilibili.com/191388711)，专出 PDD 这样知名主播的吃鸡精彩集锦，涨粉超快。于是想怎么做这样的 UP，遇到的第一个问题便是素材，精彩时刻需要手动从直播录播中剪辑，很低效。

### 用户习惯

我经常看直播，但是很少发弹幕和送礼物，只有在主播玩出很溜的操作或讲很好玩的事情时，才会发弹幕互动、送礼物支持，经常看直播的室友也是如此。

基于这个用户习惯，不难推断出在直播间的弹幕高峰或礼物高峰期，主播应该做了些好玩的事情，比如吃到鸡了，或者全队被歼灭之类的…这些时刻都可以作为精彩时刻的素材。能写程序自动截取这些素材吗？答案是肯定的。

### 实现效果

#### 弹幕抓取  <img width="80%" src="https://contents.yinzige.com/crawler.png" />



#### 数据统计

  <img width="70%" src="https://contents.yinzige.com/visu.png" />



#### 根据弹幕和礼物高峰生成的精彩剪辑 <img width="70%" src="https://contents.yinzige.com/saved.png" />



### 实现思路

通过爬虫抓取 B 站直播间数据，找出弹幕激增的时间点，使用 [FFmpeg](https://www.ffmpeg.org/) 自动剪辑时间点前后的视频即可。

本文代码：[GitHub](https://github.com/wuYinBest/bilibili-live-crawler)

```shell
> bilibili-live-crawler $ tree -L 2
.
├── README.md
├── config.php		# 配置文件：配置 FFmpeg 可执行文件的位置，录像的保存路径
├── const.php		# 常量文件：API 地址，定义数据库用户名和密码、弹幕激增的判定参数等
├── crawler.php		# 连接并抓取弹幕服务器的数据
├── cut_words
│   └── seg.php		# 分词脚本：将弹幕做分词处理，可用于生成本次直播的词图
├── db.sql		# 数据存储
├── edit.php		# 剪辑脚本
├── functions.php	# 公用函数
└── visual_data.php	# 直播数据可视化文件脚本
```



## 准备 API

以 B 站欠王痒局长为例，进入他的 [528](https://live.bilibili.com/528) 直播间，打开 Chrome 的开发者工具，看 Network 容易找出这些 API：

### 直播间原始信息

热门主播会有 2 个房间号：易识记的短房间号、原始长房间号，获取主播原始直播间信息的 API：

```json
Resquest: https://api.live.bilibili.com/room/v1/Room/room_init?id=528

Response:
{
  "code": 0,
  "data": {
    "room_id": 5441,	// 开通直播间时的原始房间号，后边会用到
    "short_id": 528,	// 短房间号
    ...
  }
}
```



### 弹幕服务器信息

直播间在加载时，会请求弹幕服务器的地址，即是我们要去爬取数据的服务器：

```xml
Request: https://api.live.bilibili.com/api/player?id=cid:5441	// 5441 即原始房间号

Response:
...
<dm_port>2243</dm_port>
<dm_server>broadcastlv.chat.bilibili.com</dm_server>
...
```



### 直播推流信息

直播间会有 3~4 个视频推流地址，选用第一个主路线会更稳定：

```Json
Request: https://api.live.bilibili.com/room/v1/Room/playUrl?cid=5441

Response:
{
  "code": 0,
  "data": {
    "durl": [
      {
        "order": 1,
        "length": 0,
        "url": "https://bvc.live-play.acgvideo.com/live-bvc/671471/live_322892_3999292.flv?wsSecret=55083259fbc34c4227691ca0feb9c4b8&wsTime=1522465545"		// flv 视频格式的推流地址
      },
      ...  
}
```





## 协议分析

B 站和斗鱼一样，为传输直播数据自己设计了协议头部。需使用 [Wireshark](https://www.wireshark.org) 抓包分析协议的细节，才能将爬虫的请求伪装成浏览器的请求，连接弹幕服务器去爬取直播间的数据。

找出弹幕服务器的 IP 地址：211.159.194.115 ![](https://contents.yinzige.com/ping-live.png)



查看请求弹幕服务器的数据包：`ip.addr == 211.159.194.115`

![](https://contents.yinzige.com/packinit.png)

前边三个包是我（10.0.1.34）与弹幕服务器（211.159.194.115）三次握手建立 TCP 连接的包。

请求的打包和解码，我参考 2016.3 的博客：[B站直播弹幕协议详解](http://www.lyyyuna.com/2016/03/14/bilibili-danmu01/)，现在抓到的包协议头与博客中的不一样，B站重新修改过了，不过应该是为了兼容，这种旧协议头还能用。

### 请求协议头

下边这个是打开进入直播间时，客户端请求弹幕服务器的请求协议头，响应协议头类似：

```Php
00000000  00 00 00 35    00 10 00 01     00 00 00 07    00 00 00 01 ...5.... ........
	# 数据包长度     # 意义不明     #请求类型：7进入直播间 # 包类型，1是数据包
    					# 2是心跳包		  
00000010  7b 22 72 6f    6f 6d 69 64     22 3a 31 30    31 36 2c 22 {"roomid ":1016,"
00000020  75 69 64 22    3a 31 35 35     39 37 33 36    38 35 37 32 uid":15597368572
00000030  38 31 36 30    7d                                   		8160}
																	# 请求的数据
```

进入直播间，打包生成连接服务器的协议头：

```Php
// $roomID 是直播间的长房间号
// $uid    是当前登录用户的 uid，游客的是随机数
function packMsg($roomID, $uid) {
	$data = json_encode(['roomid' => $roomID, 'uid' => $uid]);
    
	// 大端字节序，使用参数 N (4字节) 和 n(2字节)  打包请求
	// 占4字节的数据包长度：16字节协议头长度 + 请求数据长度
	// 占2字节意义不明：00 10
	// 占2字节意义不明：00 01
	// 占4字节的请求类型：00 00 00 07
	// 占4字节的包类型：00 00 00 01
	return pack('NnnNN', 16 + strlen($data), 16, 1, 7, 1) . $data;
}
```



### 响应数据

服务返回的 JSON 数据包的协议头如下：

```php
00000835  7b 22 69 6e 66 6f 22 3a  5b 5b 30 2c 31 2c 32 35 {"info": [[0,1,25
00000845  2c 31 36 37 37 37 32 31  35 2c 31 34 35 37 39 35 ,1677721 5,145795
...
000008E5  5d 2c 22 63 6d 64 22 3a  22 44 41 4e 4d 55 5f 4d ],"cmd": "DANMU_M
000008F5  53 47 22 7d                                      SG"}
```



解码响应的数据体：

```php
function decodeMessage($socket) {
	while (socket_last_error($socket)) {
		while ($out = socket_read($socket, 16)) {
			$res = @unpack('N', $out);
			if ($res[1] != 16) {
				break;
			}
		}
		$message = @socket_read($socket, $res[1] - 16);
		$resp = json_decode($message, true);
		switch ($resp['cmd']) {
                    case 'DANMU_MSG':	// 弹幕消息
                        // info[1]    弹幕内容
                        // info[2][1] 发送者昵称
                        echo $resp['info'][2][1] . " : " . $resp['info'][1] . PHP_EOL;
                        break;
                    case 'SEND_GIFT':	// 直播间送礼物信息
                        $data = $resp['data'];
                        // uname    发送者的昵称
                        // giftName 赠送的礼物名称
                        // unum     一次赠送的数量
                        // price    礼物的价值
                        echo $data['uname'] . ' 赠送' .  $data['num'] . '份' . $data['giftName'] . PHP_EOL;
                        break;
                    case 'WELCOME': 	// 直播间欢迎信息               
                        break;
                    default: 		// 未知的消息类型              
		}
	}
	socket_close($socket);
}
```





### 心跳包

如果客户端出现突然断网等异常情况，服务端依旧会继续推送数据，维护这种半打开的 TCP 连接将会浪费服务器的资源。客户端可以每隔一小段时间给服务端发送心跳包来保活，如果服务端一定超时时间内没收到某个客户端的心跳包，就主动断开连接。

B 站的弹幕服务器也有类似的机制，随便打开一个未开播的直播间，抓包将看到每隔 30s 左右会给服务端发送一个心跳包，协议头第四部分的值从 7 修改为 2 即可。如果不发送心跳包，弹幕服务器将在 1~2min 内主动断开连接。

```Php
// 发送心跳包
function sendHeartBeatPkg($socket) {
    // 包类型从数据包的 7 修改为心跳包的 2
	$str = pack('NnnNN', 16, 16, 1, 2, 1);
	socket_write($socket, $str, strlen($str));
}
```



## 录播并剪辑精彩时刻

录播：直接使用 FFmpeg 保存推流地址的视频即可

剪辑：根据 **<u>每分钟</u>** 的弹幕数量变化情况，如果出现峰值，取峰值 **<u>前后的一分钟</u>** 作为精彩部分。

峰值的判断标准：

- 对痒局长这样的大主播：直播间人很多，玩出甩狙瞬狙这种骚操作弹幕会激增很多，比如是前一分钟的**<u>三倍</u>**
- 对小主播：人一般比较少，弹幕数量波动不大，出现精彩操作时也涨幅也不大

以上加下划线的判定粒度、判定标准都可根据自己喜欢的主播修改，具体参考 edit.php 的实现。还可以为精彩时刻加上弹幕判定，比如分析是否有大量的 666、233、基本操作、学不来之类的词集中出现等等。



## 弹幕分析

参考 [结巴分词](https://github.com/fxsjy/jieba) 的算法，可用于生成直播的词图、分析粉丝的习惯用语等等。我参考的教程：

- [结巴分词1—结巴分词系统介绍](http://www.cnblogs.com/zhbzz2007/p/6076246.html)
- [结巴分词2--基于前缀词典及动态规划实现分词](http://www.cnblogs.com/zhbzz2007/p/6084196.html)

## 总结

开发遇到了两个难点

- 协议头部：参考的博客里边逆向 B 站官方 C# 版客户端代码，分析协议组成，感谢博主 [lyyyuna](http://www.lyyyuna.com/about/)
- 分词算法：参考的是结巴分词的前缀词典与动态规划算法，算法能力待提升 :(

再看看人家斗鱼，有开放使用的 [《斗鱼弹幕服务器第三方接入协议》](http://dev-bbs.douyutv.com/forum.php?mod=viewthread&tid=109)，协议也不会修改，兴趣使然写了 B 站的爬虫，这种根据弹幕峰值剪辑视频的想法应用在斗鱼上，估计会更有价值 :)



















