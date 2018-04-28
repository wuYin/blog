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
// 出错场景：系统中进程过多、当前用户创建的进程总数超过了系统的 CHILD_MAX 参数限制
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

### 使用场景

- 父进程复制自己，使得与子进程在同一时间执行不同的代码段，这在早些网络服务中使用：父进程用于 listen 端口，每次 accept 请求都 fork 出一个子进程去处理，而父进程则继续 listen，实现一对多的服务
- 在 fork 后配合 `exec` 函数去执行其他的程序

### 写时复制 COW（copy on write） 

 COW 是常见的内存优化手段，只在真正需要使用时才分配资源，来看 PHP 应用 COW 的例子：

```php
<?php  
$arr = array_fill(0, 100000, 233);
var_dump(memory_get_usage());
 
$arrCopy = $arr;				// 浅拷贝，只复制引用
var_dump(memory_get_usage());	// 二者指向同一个地址，此时脚本占用的内存并没有翻倍

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

在 `fork()` 后，子进程与父进程共享用户空间、代码正文段、堆和栈，内核将四块区域的权限设置为只读，只有在二者中任一进程想修改内存中的数据时，才将要修改的内存区域复制为两份，添加可写权限后处理修改请求。这种延迟复制的实现，减少了进程创建的时间开销，也节省了内存。

### 示例

```c
#include "apue.h"

int globalVar = 6;
char buf[] = "a write to stdout\n";

int main(void) {
    int var;
    pid_t pid;

    var = 88;
    if (write(STDOUT_FILENO, buf, sizeof(buf) - 1) != sizeof(buf) - 1) {    
        err_sys("write error");
    }

    printf("before fork\n");

    if ((pid = fork()) < 0) {
        err_sys("fork error");
    } else if (pid == 0) {      // 在子进程内修改变量的值，
        globalVar++;
        var++;
    } else {
        sleep(2);       		// 强制使父进程休眠 2s，让子进程先执行，否则二者的执行顺序是不定的
    }

    printf("pid = %ld, glob = %d, var = %d\n", (long) getpid(), globalVar, var);    
    exit(0);
}
```

可以看到，在子进程内将全局变量 `globalVar` 的值修改为 7，将局部变量 `var` 的值修改为 89，父进程的两个变量值均未改变：

 ![image-20180424092821178](http://p7f8yck57.bkt.clouddn.com/2018-04-24-image-20180424092821178.png)

输出流重定向多了一行  "before fork" 的原因：

- `./a.out` 的标准输出指向终端，是行缓冲的，在第 15 行遇到 `\n` 后会调用 `write()` 输出并冲洗缓冲区
- `./a.out > temp.out` 的标准输出指向文件，是全缓冲的，所以在 `fork()` 子进程时会将缓冲区一同复制







## vfork 函数

### 定义

```c
#include <unistd.h>

// 创建一个调用其他程序的新进程
pid_t vfork(void);
```

返回 PID 指向的子进程直接在父进程的地址空间中运行，没有内存复制的概念，会比 COW 更快。

### 与 fork 的区别

|  区别项  |                             fork                             |                            vfork                             |
| :------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
|  隔离性  | 子进程在写时复制父进程的数据作为副本，数据（变量）的修改互不影响 | 子进程运行在父进程的地址空间中，数据的修改对二者是同步修改的 |
| 执行顺序 |                     父子进程调用顺序不定                     | 保证子进程先运行，它调用 exec() 或 exit() 后，父进程才继续运行 |

### 示例

```c
int globalVar = 6;
char buf[] = "a write to stdout\n";

int main(void) {
    int var;
    pid_t pid;

    var = 88;
    if (write(STDOUT_FILENO, buf, sizeof(buf) - 1) != sizeof(buf) - 1) {
        err_sys("write error");
    }

    printf("before fork\n");    

    if ((pid = vfork()) < 0) {
        err_sys("fork error");
    } else if (pid == 0) {      // 在子进程内修改变量的值
        globalVar++;
        var++;
        _exit(0);       		// 不会像 exit 一样冲洗标准 IO
    }

    printf("pid = %ld, glob = %d, var = %d\n", (long) getpid(), globalVar, var); 
    exit(0);
}
```

可看到，子进程内对 2 个变量的自增操作同步到了父进程：

![image-20180424122800708](http://p7f8yck57.bkt.clouddn.com/2018-04-24-image-20180424122800708.png)





## 异常进程 与 wait 函数

因为子进程与父进程的运行是异步的，异常终止时，会产生 2 种异常的子进程：

- 父进程比子进程提前终止，子进程变为孤儿进程
- 子进程终止后，父进程不予处理，子进程变为僵死进程

### 僵死进程

#### 产生原因

一般情况下，子进程在退出时，内核为它保存了如 PID、终止状态和运行时间等信息，其父进程可使用 `wait` 或 `waitpid` 来获取这些信息，进而释放子进程的资源。

#### 例子

特殊情况下，子进程运行完毕终止后，父进程对其不做处理，则其将变成僵死进程，如：

```c
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int main(void) {
    pid_t pid;
    if ((pid = fork()) >= 0) {
        if (pid == 0) {
            exit(0);
        } else {
            sleep(233);	// 强制使父进程休眠，不处理执行结束的子进程
        }
    }
    return 0;
}
```

STAT 一栏 Z 标识 PID 为 7154 的子进程为僵死进程： 

![image-20180424151351528](http://p7f8yck57.bkt.clouddn.com/2018-04-24-image-20180424151351528.png)

#### 危害

内核为僵死进程保留的信息得不到处理，PID 等资源会被一直占用，如果系统中大量的僵死进程耗尽了可用的 PID，将无法再创建进程。



### 孤儿进程

#### 产生原因

父进程比子进程先终止，则这些子进程将会被 PID 为 1 的 init 进程收养。作为 ”父进程“ 的 init 进程会时刻监视着它们，任一个孤儿进程终止，init 都会主动调用 wait 处理并释放它占用的资源。

#### 危害

因为 init 会帮孤儿进程善后，相比僵死进程，它就没什么危害。



### wait 函数

```c
#include <sys/wait.h>

// 阻塞当前进程的执行，直到有子进程终止
// 成功返回该子进程的 PID，出错则返回 -1，错误原因保存在 errno
// 若参数不是 NULL，则子进程的整型退出状态会保存到 status
pid_t wait (int *status);
```

Unix 中有专门检查 status 退出状态的函数宏，如：

- `WIFEXITED(status)` ：检查子进程是否是正常退出的，是则返回真

- `WEXITSTATUS(status)`：若子进程正常退出，则取得子进程 exit() 的状态码，如果是非正常退出则返回 0 

#### 示例

```c
#include <stdlib.h>
#include <unistd.h>
#include <sys/wait.h>
#include <stdio.h>

int main(void) {
    pid_t pid;
    int status, i;
    if (fork() == 0) {
        printf("子进程 PID = %d\n", getpid());
        exit(233);
    } else {
        sleep(1);
        printf("父进程中等待子进程运行终止...\n");
        pid = wait(&status);
        if (WIFEXITED(status)) {
            printf("接收到子进程 PID = %d \n退出状态 = %d\n", pid, WEXITSTATUS(status));
        }
    }
}
```

  ![image-20180424164220450](http://p7f8yck57.bkt.clouddn.com/2018-04-24-image-20180424164220450.png)





## 进程竞争与死锁

