---
title: Unix 环境高级编程：进程环境
date: 2018-04-19 12:44:15
tags: Unix
---



[《Unix 环境高级编程》](https://book.douban.com/subject/25900403/) 第七章读书笔记，转载请注明来源。

<!-- more -->

本文代码：[Github](https://github.com/wuYin/apue)



## 本文结构

```
进程的环境 
    ├── 执行程序：main 函数
    ├── 终止进程
    ├── 命令行参数
    ├── 进程的环境表
    ├── 进程的内存分布
    ├── 进程间的共享库
    ├── 内存分配
    ├── 环境变量
    ├── setjmp 与 longjmp 函数
    └── getrlimit 与 setrlimit 函数
```



## 执行程序：main 函数

### 定义

`main()` 是 C 程序的主函数，是程序执行的入口，Golang 与之类似，C99 标准对 main 的 2 种正确的定义：

```c
// 无参数
int main(void) {
    return 0;
}

// 有参数
int main(int argc, char *argv[]) {
	return 0;
}

// main 还可有第三个参数 char *ecvp[]，用于存储运行环境的环境变量
```

为提高程序的可移植性，避免使用下边对 main 定义的形式：

```c
// 没有纳入 C 的标准，大部分编译器是不允许这么写的
void main() {
    // ...
}
```



### 参数值

```c
// argc 即 argument counts，命令行中参数的个数，程序会在运行时自动统计
// argv 即 argument values，使用空格分隔，包含指向命令行参数值的指针。其中 argv[0] 为程序名
int main(int argc, char *argv[]) {
    printf("argc: %d\n", argc);

    int i;
    for (i = 0; i < argc; i++) {         	// 循环输出全部的命令行参数
        printf("argv[%d]: %s\n", i, argv[i]);	// 命令行参数的类型全是字符串，只能使用 %s 输出
    }
}    
```

在程序中就能检查并获取运行参数了（a.out 相当于 Windows 上的 a.exe）：

 ![image-20180419154529101](http://p7f8yck57.bkt.clouddn.com/2018-04-19-image-20180419154529101.png)



### 返回值

`return 0;`  返回给操作系统程序的执行状态为 0，表示正常退出，返回其他值则认为程序发生了错误。如：

```c
/* demo.c */
int main(void) {
    return 233.7;	// 返回值会被强制转换，截断为整型 233
}
```

在 Unix 上使用 `$?` 来验证程序的退出状态：

 ![image-20180419153037368](http://p7f8yck57.bkt.clouddn.com/2018-04-19-image-20180419153037368.png)

说明：在 shell 中执行 `cc demo.c && ./a.out` 后 `a.out` 作为 shell 的子进程运行，在退出时将 0 返回给了 shell，故能使用 `echo $?` 查看其退出状态。



## 终止进程

### 终止的 8 种方式

#### 5 种正常终止

```
main() 中 return
exit()
_exit() 或 _Exit()
多线程程序中，最后一个线程从其 main() 中 return
多线程程序中，从最后一个线程调用 pthread_exit()
```

```C
// ISOC 标准
#include<stdlib.h>
void exit(int status);	// 正常结束当前正在执行的进程，将 status 返回给父进程，关闭进程打开的文件
			// 关闭文件（I/O流）之前，会调用 fclose 将缓冲区数据写回该文件（I/O流 ）
void _Exit(int status);	// 立刻结束当前进程，返回 status 并关闭打开的文件，但是不处理缓冲区


// POSIX 标准
#include<unistd.h>
void _exit(int status); // 同 _Exit() // 遵循的标准不一样，在不同的头文件中
```

#### 3 种异常终止

```
abort()
接到一个信号
多线程的程序中，最后一个线程对取消请求作出响应		// 多线程部分在第 12 章
```



### 退出状态

#### 状态范围

```go
// 退出状态码为 0 ~ 255，超出后则返回 n % 256
int main(void) {
    exit(257);		// 返回 1
}
```

#### 状态不确定的三种情况

```c
// 调用上边三个函数时无 status 参数
int main(void) {
    exit();		// c99 标准的编译器会直接报错：too few arguments to function call
}


// main 没有声明返回值是整型
float main(void) {
    exit(233);		// 第一次返回 233，之后均返回 0
}

// main 执行了无返回值的 return 语句
void main(void) {
    return ;		// 第一次返回 176，之后均返回 0
}
```





### `atexit()` 函数

```c
#include<stdlib.h>
int atexit(void (*func) (void));	// 注册的函数参数和返回值均为 void
```

 `atexit()` 注册的函数称为 exit handler，在进程退出时会被 `exit()` 自动调用后再清理缓冲区。有 2 个特点：

- 先注册的后调用：类似于 Golang 的 defer 语句，handlers 的调用也是栈顺序的
- 多次注册同一函数，依旧会被执行多次


#### 特性：

```c
int main(void) {

    if (atexit(myExit2) != 0) {
        err_sys("can't register myExit2");      // 终止函数先注册后调用
    }
    if (atexit(myExit1) != 0) {
        err_sys("can't register myExit1");
    }
    if (atexit(myExit1) != 0) {                 // 终止程序登记一次就会被调用一次
        err_sys("can't register myExit1");
    }

    printf("main is done\n");
    return 0;
}


static void myExit1(void) {
    printf("first exit handler\n");
}

static void myExit2(void) {
    printf("second exit handler\n");
}
```

运行：

![image-20180420101734467](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420101734467.png)



#### 限制：

书上说一个进程使用 `atexit()` 最多注册 32 个清理函数，但后来的操作系统有所差异，使用 `sysconf` 查看这个限制值：

```c
#include <stdio.h>
#include <unistd.h>

int main(void) {
    printf("%ld", sysconf(_SC_ATEXIT_MAX));	// 能注册 2147483647 个
}
```

差距太大了，于是我在 macOS 上使用  for 循环验证了一下：

```c
int main(void) {
    for (int i = 0; i < 214748367; i++) {
        if (atexit(myExit) != 0) {		// 终止程序登记一次就会被调用一次
            err_sys("can't register myExit1");
        }
    }
    return 0;
}


static void myExit(void) {
    printf("exit handler\n");
}
```

程序能正确调用，只是内存占用会飙升233：

 ![image-20180420100952293](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420100952293.png)



### C 程序的启动与终止流程：

- 内核执行程序的唯一办法是调用 `exec` 
- 进程主动退出的唯一办法是调用 `exit()`、`_Exit()`、`_exit()`

 ![image-20180420095014936](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420095014936.png)









## 命令行参数

在 main() 函数的参数值已讨论，不过遍历命令行参数的结束条件还有另一种方式：

```c
// for (i = 0; i < argc; i++) {         // 循环输出全部的命令行参数
for (i = 0; argv[i] != NULL; i++) {     //  POSIX 和 ISO C 标准都要求 argv[argc] 是空指针
    printf("argv[%d]: %d\n", i, argv[i]);
}
```







## 进程的环境表

### `ecvp` 参数

环境参数是name=value 格式的字符串，能通过第三个参数 `char *ecvp[]` 接收到环境表：

```c
int main(int argc, char *argv[], char *ecvp[]) {
    for (int i = 0; ecvp[i] != NULL; i++) {     //  POSIX 要求 argv[argc] 是空指针
        printf("ecvp[%d]: %s\n", i, argv[i]);
    }
}
```

输出的环境参数：

```shell
ecvp[8]: LANG=zh_CN.UTF-8
ecvp[9]: PWD=/Users/wuyin/C/apue
ecvp[10]: SHELL=/bin/zsh
...
ecvp[16]: HOME=/Users/wuyin
ecvp[18]: USER=wuyin
```



### `environ` 全局变量

环境表与命令行参数表一样，也是字符指针数组，指针指向各环境参数。其地址存储在全局变量 `environ` 中：

```c
#include <stdio.h>
#include <unistd.h>		

extern char **environ;	// 使用来自 unistd.h 的外部变量 environ，类型是指向指针的指针

int main(int argc, char *argv[], char *ecvp[]) {
    char **env = environ;

    while (*env != '\0') {	// 环境参数字符串以 NULL 结尾
        printf("%s\n", *env);
        env++;
    }
    return 0;
}
```







## 进程的内存分布

![image-20180420111106632](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420111106632.png)

常驻内存：从进程开始到退出一直存在，使用常量地址访问。

### 静态区域

#### 正文段

- 图中的 .text
- 内容：程序的机器指令
- 共享的：同时运行多个 shell，开始运行时都执行同一代码段；只读的：避免被篡改

#### 只读数据段

- 图中 .rodata
- 内容：程序中不会修改的数据（常量），比如字符串

#### 已初始化数据段

- 图中的 .data
- 内容：程序中初始化的数据，比如被赋初值的全局变量、静态变量

#### 未初始化数据段

- 图中的 .bss
- 内容：程序中没有初始化的数据：比如仅声明，使用默认零值的变量



### 动态区域

#### 堆

- 用于动态内存分配
- 一般由程序员手动分配 malloc 和 释放 free

#### 栈

- 存放：临时数据：临时变量、调用函数时需要保存的数据等
- 函数在递归调用时，会新开一个栈来存储自身的变量集，所以变量互不影响






## 进程间的共享库

 ![image-20180420152353111](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420152353111.png)

参考 [阮一峰：编译器的工作过程](http://www.ruanyifeng.com/blog/2014/11/compiler.html)

在 link 链接阶段，C 程序引用的库分为 2 种：

静态库 *.a、\*.lib：外部函数库添加到可执行文件，体积大，但适用性更高

动态库（共享库）*.so、\*.dll：外部函数库只在运行时动态引用，体积更小，但适用性更低







## 内存分配

### 动态内存分配函数

```c
#include<stdlib.h>	// 定义

// 分配 size 字节大小的内存区域
// 成功则返回内存地址，失败返回 NULL
void *malloc(size_t size);	


// 分配 nobj 个长度为 size 字节的内存区域，并将每一个字节都初始化为 0 
// 成功则返回内存地址，失败返回 NULL
void *calloc(size_t nobj, size_t size);


// 为 ptr 指向的空间分配新的 newsize 字节的内存
// ptr 必须指向动态分配的内存，即三个 *alloc() 函数的返回值
// ptr 为 NULL：realloc() 与 malloc() 相同
// newsize 为 0： ptr 指向的空间会被释放，返回 NULL，类似 free()
//         更大： 存储区域有足够的空间可扩充，则扩充后直接返回 ptr
// 		 存储区域没有足够的空间可扩充，复制 ptr 指向区域的数据到更大的内存空间
// 类似 Golang 中 slice 在 len 接近 cap 之后进行内存重新分配的操作
void *realloc(void *ptr, size_t newsize);
```

### 注意

#### 返回值类型

返回值都是 `void *` ，不是没有返回值或返回空指针，而是返回通用指针，指向的类型未知，类似于 Go 中的 interface{} 类型。在使用时，需要将返回的 `void *` 进行强制类型转换，方便存储数据

#### 内存释放

必须使用 `void free (void* ptr);` 来手动释放分配的内存，如果忘记释放的内存累计过多，进程将可能出现内存泄漏

对一块内存只能释放一次，调用多次 `free()` 将出错



### 使用示例

```c
#include <stdio.h>
#include <stdlib.h>

int main() {
    int n = 2;
    int *buf1 = (int *) malloc(n);	// 强制将 void* 转换为 int*
    if (buf1 == NULL) {   		// 检查是否分配成功
        exit(1);
    }
    for (int i = 0; i < n; i++)
        printf("buf1[%d]: %d\n", i, buf1[i]);	// malloc 分配的空间值的值是未知的

    int *buf2 = (int *) calloc(n, sizeof(int));	// calloc 分配的空间有默认值 0 
    for (int i = 0; i < n; i++)
        printf("buf2[%d]: %d\n", i, buf2[i]);

    free(buf1);
    // free(buf);  
    // malloc: *** error for object 0x7fd9b4d000e0: pointer being freed was not allocated
    // 没有 free(buf2) 则可能发生内存泄漏
    return 0;
}
```

运行：

 ![image-20180420163216349](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420163216349.png)







## 环境变量

### 查询环境变量的值

```c
#include<stdlib.h>
char *getenv(const char* name);		// 环境变量 name 存在则返回值的指针，不存在则返回 NULL
```



### 设置环境变量的值

```c
#include<stdlib.h>

// str 是 name=value 格式的参数，用于新增或覆盖 name
// 执行成功返回 0，失败返回 -1
int putenv(char *str);	


// 设置 name 环境变量的值为 value
// 若 rewrite == 0 则新增或不覆盖
//            != 0 则新增或覆盖
int setenv(const char *name, const char *value, int rewrite);


// 删除 name 环境变量，不存在也不会报错
int unsetenv(const char *name);
```

参考前边环境表的 environ 全局变量，操作环境参数时更推荐使用上边的函数。







## setjmp 与 longjmp 函数

### 函数内跳转

在 C 中使用 goto 在函数内部（栈中）跳转，可往前也可往后跳。不过为了提高代码的可维护性，应尽量少使用。除非你明确要使用它来跳出深层次的循环，那也不错。

### 函数间跳转

在 C 中使用 `setjmp()` 与 `longjmp()` 在函数之间（栈之间）跳转：

```c
#include<setjmp.h>	// 定义

// env 参数的类型是 jmp_buf，用于标识当前进程状态的锚点
int setjmp(jmp_buf env);			


// 使用与 setjmp 对应的 env 参数，调用时直接跳转到 env 处
// 一个 setjmp 可对应多个 longjmp，使用 val 标识是从哪里回退的
// val 即是 setjmp 的返回值
void longjmp(jmp_buf env, int val);
```



#### 示例

```c
#include <stdio.h>
#include <setjmp.h>
#include <stdlib.h>

jmp_buf saved_state;

void myLongjmp();

int main(void) {
    printf("设置 setjmp 的锚点\n");
    int ret_code = setjmp(saved_state);
    if (ret_code == 1) {
        printf("结束序号为 1 的跳转\n");
        return 0;
    }
    int *flag = (int *) calloc(1, sizeof(int));
    myLongjmp();    // 直接跳转到第 11 行
}

void myLongjmp() {
    printf("准备开始序号为 1 的跳转\n");
    longjmp(saved_state, 1);
}
```

效果：

 ![image-20180420190004608](http://p7f8yck57.bkt.clouddn.com/2018-04-20-image-20180420190004608.png)



#### 内存泄漏

使用 [Valgrind](http://valgrind.org/) 做内存泄漏检测，结果显示有 4 字节的内存 lost，即是 16 行分配的内存没有释放：![image-20180420193100439](http://p7f8yck57.bkt.clouddn.com/2018-04-20-2018-04-20-image-20180420193100439.png)

在 `setjmp()` 和 `longjmp()` 之间分配的内存，在跳转后直接就废弃了，可能会因此发生内存溢出。

和 goto 一样，除非你知道自己在做什么，否则应尽量避免使用。





## getrlimit 与 setrlimit 函数

系统对进程能调用的资源有限制，可使用 `getrlimit()` 来查看、`setrlimit()` 来修改

### 函数原型

```c
#include<sys/resource.h>	// 定义

// resource 是标识资源类型的常量
// rlimit 的定义
struct rlimit {
	rlim_t	rlim_cur; // current (soft) limit	// 当前软链接的限制值
	rlim_t	rlim_max; // maximum value for rlim_cur	// 硬链接的限制值
};

// 修改成功返回 0，失败返回非 0
int getrlimit(int resource, struct rlimit *rlptr);
int setrlimit(int resource, struct rlimit *rlptr);
```



### 可修改的资源值

|    参数值     |        参数说明        |
| :-----------: | :--------------------: |
|   RLIMIT_AS   |  进程可使用的最大存储  |
| RLIMIT_NOFILE | 进程可打开的最大文件数 |
| RLIMIT_STACK  |   进程的栈的最大长度   |
|       …       |           …            |

更多请参考：[手册](http://man7.org/linux/man-pages/man2/getrlimit.2.html)





## 总结

开始学习 APUE 就卡在了第三章，于是从第七章熟悉的 `main()` 开始学起，发现 C 和 Golang 真的有千丝万缕的联系，比如 `atexit()` 与 `defer func(){}`，另外还有进程相关的内存分布、内存分配都值得深入学习，后边依旧在本篇笔记中补充。

计划下周 4.27 前更新第八章笔记 :)















