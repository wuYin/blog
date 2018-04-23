---
title: Unix 环境高级编程：进程控制
date: 2018-04-23 20:33:37
tags: Unix
---





## 本文结构

  ![image-20180423211757350](http://p7f8yck57.bkt.clouddn.com/2018-04-23-image-20180423211757350.png)



## 进程标识

### PID 标识

#### 范围

进程 ID（Process ID）是操作系统中唯一标识进程的非负整数，取值范围：0 ~ 32767（signed short int最大值）

 ![image-20180423205959393](http://p7f8yck57.bkt.clouddn.com/2018-04-23-image-20180423205959393.png)

其中0 ~299（macOS 是 100）为系统预留给守护进程使用，递增向上分配，进程终止后其 PID 会被回收利用。

#### 特殊进程

| PID  |   名字    |                             描述                             |
| :--: | :-------: | :----------------------------------------------------------: |
|  0   | idle 进程 |        调度进程，是系统创建的第一个进程，运行在内核态        |
|  1   | init 进程 | 由 idle 进程创建，完成 Unix 系统的初始化，是一个有 root 权限的普通用户进程，是后续系统中所有孤儿进程的父进程 |

### 其他标识

```c
#include<unistd.h>	 // 以下标识函数没有出错返回
pid_t getpid(void);  // 返回进程的 PID
pid_t getppid(void); // 返回当前进程的父进程 PID
uid_t getuid(void);  // 返回进程的实际用户 UID
uid_t geteuid(void); // 返回进程的有效用户 UID
gid_t getgid(void);  // 返回进程的实际组   GID
gid_t getegid(void); // 返回进程的有效组   GID
```



## fork 函数

### 定义

```c
// 在当前线程中创建新进程，对子进程返回 0，对父进程返回子进程的 PID，若出错则返回 -1
pid_t fork (void);
```

### 返回值

特点：子进程只有一个父进程，能轻松通过 `getppid()` 获取父进程 PID，但父进程有多个子进程，没有函数能获取它所有子进程的 PID

fork 出的子进程会共享父进程的代码正文段，在同一份代码中区分二者：

```c
// 都会执行的代码
if ((pid = fork()) < 0) {
	// fork 出错
} else if (pid == 0) {     
	// 仅子进程内执行的代码
} else {
    // 仅父进程内执行的代码
}
// 都会执行的代码
```

### 写时复制 COW（copy on write） 

 COW 是常见的内存优化手段，只在真正需要使用时才分配资源，来看 PHP 应用 COW 的例子：

```php
<?php  
$arr = array_fill(0, 100000, 233);
var_dump(memory_get_usage());
 
$arrCopy = $arr;
var_dump(memory_get_usage());	// 仅赋值：二者指向同一块内存，此时脚本占用的内存并没有翻倍

$counts = 1;
foreach($arrCopy as $a){
    $counts += count($a); 
}
var_dump(memory_get_usage());	// 新变量只读不修改，就不会占用新内存

$arrCopy[0] = 100;
var_dump(memory_get_usage());	// 修改 $arrCopy 数组的值，才申请新的内存来存放 [100, 233...]
```

内存占用：

 ![image-20180423220648467](http://p7f8yck57.bkt.clouddn.com/2018-04-23-image-20180423220648467.png)



























