title: PHP 源码编程规范
date: 2018-01-02 20:06:28
tags: 

- Coding style

---

在学习 TIPI 项目第二节介绍根目录文件时，第一个文件便是 [CODING_STANDARDS](https://github.com/php/php-src/blob/master/CODING_STANDARDS) ，遂翻译学习下官方给出的 PHP 源码代码规范。

<!-- more -->



#### 一、代码实现建议

-   在源码中逻辑复杂的地方应加以注释
-   若同一个模块中函数之间相互依赖，则那些小的功能函数应该声明为 `static` 只在类内部使用，且应尽量避免这种紧耦合
-   尽可能的使用 define 宏定义，让值作为有意义命名的常量使用。但有一个例外，当 1 和 0 分别作为 `true` 和 `false` 使用时，使用数值常量 1、0 来区分不同的行为应该通过 `#define` 完成操作
-   在各种处理 string 的函数实现中，应保证 PHP 对 string 依旧有 length 长度属性，且这个长度不该使用 `strlen()` 来计算，应该自己实现字符串的长度计算，并保证操作高效、安全
-   尽量避免使用 `char * strncat(char *dest, const char *src, size_t n)` 函数来操作字符串
-   使用 `#if` 语句注释外部源码时，条件应该有意义命名，例如 `#if wuYin_0` 
-   不要定义用不到的函数，找不到 function 并不应报出运行时错误： `function doesn't exists`，应该在函数使用前使用 `function_exists()` 先检测
-   应该使用 `emalloc()`、 `efree()`、 `estrdup()` 等原生 C 函数来操作内存，这些函数在内部实现了 "safety-net" 机制，能保证在请求结束前释放掉不可用的内存，在调试时这些函数也能提供有用的内存分配、溢出信息



#### 二、函数命名规范

##### 	`	user-level` 函数

-   命名应该全小写、下划线 `_` 分隔、且尽量简短清晰

-   当缩写会大大降低函数名称可读性时，不应该使用缩写

    ```php
    // 优秀命名
    str_word_count()
    array_key_exists()
      
    // 一般命名
    // 在不影响可读性的情况下尽量缩写
    // 如 string 写为 str、interval 写为 intval   
    date_interval_create_from_date_string()		// date_intvl_create_from_date_str() 更好
    get_html_translation_table()			// html_get_trans_table() 更好
      
    // 糟糕命名
    hw_GetObjectByQueryCollObj()	// 大小写、驼峰下划线混用，可读性很差
    pg_setclientencoding()
    jf_n_s_i()
    ```

-   当一系列函数限定属于一个共同的父集，函数命名应加上 `parent_*` 前缀

    ```php
    // 优秀命名
    mysql_connect()
    mysql_select_db()
    mysql_affected_rows()
      
    // 糟糕命名
    mysqlconnect()
    mysql_selectdb()
    affected_mysql_rows()  
    ```

    ​

-   变量名必须有意义、全小写、单词间下划线分隔，应尽量避免无实际意义的命名，比如

    `for (i=0; i<100; i++) {...}` 中的变量 `i`

-   方法的命名有别于函数，应该使用驼峰式命名，如： `getData()`

-   类的命名不该使用缩写，每个单词的首字母均应大写，如： `FooBar` 



##### internal 函数

-   当函数作为外部 API 使用时，应该命名为 `php_modulename_function()` 来避免引用多个模块造成函数的命名冲突
-   开放的 API 需在 `php_modulename.h` 头文件中定义，不开放的 API 不在该文件中且应声明为 `static`
-   主模块源文件必须命名问 `modulename.c`
-   被其他源文件使用的头文件必须命名为 `php_modulename.h`



#### 三、语法和缩进

-   使用 C 风格的注释，而非 C++ 的单行注释：

    因为 PHP 是 C 实现，目标是在任何兼容 ANSI-C 规范的编译器均可编译，即时很多编译器兼容 `//` 的注释语法，为了保证其他编译器也应该使用 `/** ... **/` 注释。但在 Win32 平台是个特例，它的编译器均支持在 C 中使用单行注释

-   遵循 [K&R-style](http://www.catb.org/~esr/jargon/html/I/indent-style.html)  缩进风格：

    ```php
    if（<cond>） { 
        <body> 
    }
    ...
    ```

-   应大量使用空白来使代码结构清晰：

    -   保证变量声明、逻辑代码块等有空行分隔

    -   函数定义之间用 1~2 个空行分隔

        ```php
        // 优秀清晰的代码
        if (foo) {
        	bar;
        }

        // 简短但不够清晰的代码
        if(foo) bar;
        ```

-   缩进应使用 `Tab` 符，来保证缩进统一，也可统一使用 4 个 space 表示一个 tab

-   预处理语句如 `#if` 必须在行首，即 `#` 为行第一个字符

#### 四、文档格式

-   `user-level` 函数都应该有相应的函数注释，如用一句话简要的概括函数的功能

    ```php
    /* {{{ proto int abs(int number)
       返回数的绝对值 */
    PHP_FUNCTION(abs)
    {
       ...
    }
    /* }}} */

    // 注明：{{{ 和 }}} 是 vim、Emacs 的折叠符号（相当于 PHPStorm 的 Command + .）
    ```

    ​


大致有参考意义的建议就是这些，然而有一部分并不能很好的理解，在后边学习中再回头看下。