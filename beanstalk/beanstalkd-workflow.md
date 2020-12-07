---
title: beanstalk 实现分析
date: 2020-12-04 12:00:00
tags: pulsar
---

简要梳理 [beanstalkd](https://github.com/beanstalkd/beanstalkd) MQ 的实现。

<!-- more -->

## 1. 基本概念

### 1.1  消息模型

 <img src="https://images.yinzige.com/20201204114918.png" style="zoom:50%;" />

由上图，beanstalk 可简单认为是传统 PubSub 模型加 Stateful Message 的实现。与传统 MQ 的概念对应：

- **job**：即 message，但有状态，如消息可带延迟时间，让 server 延迟下发给 worker，还能带优先级让紧急 job 优先下发等等。

- **tube**：即 topic，一个 producer 只能向一个 tube 投递 job；一个 worker 能订阅多个 tube 来同时处理多个 job，多个 woker 也能订阅同一个 tube 轮询消费。

- **producer** 与 **worker**：即 producer 与 consumer

### 1.2  Stateful Job

Job 有五种状态：

- **Ready**：准备完毕。可被立即下发给 worker
- **Delayed**：被指定为延迟投递。倒计时结束后转为 Ready 才能下发。
- **Reserved**：已被下发给 worker，等待 worker 回复确认，或等到 TTR 超时自动回退为 Ready 重新下发。
- **Buried**：被指定为仅保留。等待 worker 主动将其改为 Ready 或删除。
- **Deleted**：被彻底删除。

其中，producer 只参与前 2 种，job 发布后与其再无关联。但 worker 可按需让 job 在多个状态间灵活切换。命令交互图：

 <img src="https://images.yinzige.com/20201204125951.png" style="zoom:50%;" />



## 2. 数据结构

### 2.1  优先级队列

**2.1.1  场景**

- tube 的 ready 队列：以 job 的优先级为权重，最紧急 job，最先下发给 worker

- tube 的 delay 队列：以 job 的到期时间戳为权重，最快到期的 job，最先由 Delayed 升级为 Ready

- server 的 conns 队列，权重有 2 种情况：

  - 以 reserve-timeout 等待超时的时间戳为权重，先到期的 conn 先响应 TIMED_OUT
  - 以 reserved job TTR 超时的时间戳为权重，先到期的 job 先从 Reserved 回退到 Ready

  说明：因为上述  2 种超时事件只可能依次发生，时间上不重叠，所以 beanstalk 只用一个 `conn.tickat` 字段，动态地在这 2 种超时时间戳上切换，避免了维护 2 个 conns 堆的开销及同步问题。

**2.1.2  实现**

```c
struct Heap {
    size_t  cap;
    size_t  len; 
    void    **items; // job, conn

    less_fn   less;   // 2 个 item 间比较大小的函数，比如比较 job.priority 和 conn.tcickat 的大小
    setpos_fn setpos; // item 在 heap 中移动调整时的回调函数
};
typedef int(*less_fn)(void* item1, void* item2); // 比较函数
typedef void(*setpos_fn)(void* item, size_t k);  // 调整堆时通知 item 新位置为 k 的回调函数

int   heapinsert(Heap *h, void *x);
void* heapremove(Heap *h, size_t k); // 直接从 heap 中移除位置 k 上的 item
```

说明：`setpos_fn()` 回调对于优化删除有必要的，item 必须知道自己在堆中的正确位置，才能主动地**直接**将自己从堆中删除。

原因：当堆插入新 item 时，item 会向上冒泡寻找合适的位置，一路上被交换的旧 item 在堆中的位置也会被动态调整，需要 `setpos_fn` 回调来将最新的索引同步给新旧 item

场景：job 加入 tube 时会按优先级放入 ready 堆，此时堆上的某些旧  job 位置会被调整，当 job 被下发后需从 ready 删除时，不必低效地遍历整个堆来查找被下发的 job 再删除，而是直接指定删除位置 k 上的 job



### 2.2  双向循环链表

**2.2.1  场景**

- conn 的 reserved_job 链表：对某个 conn，server 要为它维护从多个 tube 下发的 job 列表，用于遍历计算 conn 的最短 TTR
- tube 的 buried_job 链表：维护保留的 job

**2.2.2  实现**

只有 job 才会用链表，beanstalk 直接把 `prev, next` 节点指针嵌入到了 job 结构中。针对 job 的链表操作函数：

```c
void job_list_reset(Job *head);
int job_list_is_empty(Job *head);
Job *job_list_remove(Job *j);
void job_list_insert(Job *head, Job *j); // 直接在链表头节点之前插入新 job
```

**2.2.3  状态**

- 初始状态，或被从链表中移除后

   <img src="https://images.yinzige.com/20201204151340.png" style="zoom:50%;" />

- 正常状态

   <img src="https://images.yinzige.com/20201204151359.png" style="zoom:50%;" />



### 2.3  集合

用 C 实现了翻倍扩容的动态数组。

**2.3.1  场景**

- server 维护全局 tubes：实现 tube 的查找。
- tube 维护 waiting  conns 集合：tube 需要维护 watching 自己的多个 conn，用于轮询下发 job
- conn 维护 watching tubes 集合：conn 需要维护自己在 watching 的多个 tube（类似 Kafka rebalance）
  - 要 reserve 消息就加入 `tube.waiting` 等待轮询。
  - reserve 成功就离开 `tube.waithing`，以实现一次 reserve、一次下发的效果。

**2.3.2  实现**

```c
struct Ms {
    size_t len; // 类比 go slice
    size_t cap;
    size_t last; // 从集合 take 元素的自增游标
    void **items;

    ms_event_fn oninsert; // 追加 item 的回调
    ms_event_fn onremove; // 删除 item 的回调
};

typedef void(*ms_event_fn)(Ms *a, void *item, size_t i); // 从集合 a 的位置 i 上增删 item 时的回调
```

说明：`oninsert/onremove` 仅用在了 `conn.watching` 集合上，用于 tube 的 GC

回调场景：每个 conn watch 或 ignore 某个 tube 时，回调会实时增减该 tube 的引用计数，减为 0 时删除该 tube 及时释放内存。



### 2.4  哈希表

为执行 `release <jobId>` 等用 jobId 查找 job 再处理的命令，server 端要像维护全局 tubes 集合那样维护全局 job 列表，频繁查询场景哈希表比集合更适合，另外 job 有额外的 `ht_index` 字段实现拉链法解决哈希冲突。



## 3. 协议

beanstalk 客户端与服务端使用 ASCII 纯文本协议交互，客户端发起请求后阻塞等待响应，服务端再有序应答。类比 HTTP 1.0

协议由 2 部分组成，由 CRLF `\r\n` 分隔：

- cmd header：各种命令及对应的参数列表。
- body raw bytes：可选，只在投递和下发 job 时才有值。

```C
CMD_TEXT_LINE\r\n
[DATA_CHUNKS\r\n]
```

值得注意的是 `fill_extra_data` 函数：由于 CMD_TEXT_LINE 的长度由各命令而定，长短不一，beanstalk 在读 header 时直接按最长命令来读：

```c
// "pause-tube a{200} 4294967295\r\n"
#define MAX_TUBE_NAME_LEN 201
#define LINE_BUF_SIZE (11 + MAX_TUBE_NAME_LEN + 12)
```

当出现读 header 预读了部分 body 时，就需要 `fill_extra_data` 来将预读部分提前拷贝到 body 中。



## 4. 主流程

### 4.1  epoll

beanstalk server 使用 epoll 监听、注册并处理读写事件，并封装了 `Socket` 接口进行平台透明的 epoll 操作：

```c
struct Socket {
    int    fd; // server 或 client socket
    Handle f;  // event handler: srvaccept 或 prothandle
    void   *x; // handler f 的参数 x
};

int sockinit(void); // 创建 epoll fd
int sockwant(Socket *s, int rw); // 为 s->fd 注册读写事件，rw 为 0 时取消注册
int socknext(Socket **s, int64 timeout); // 等待 epoll_fd 上最近发生事件的 socket s 并返回事件类型。若超时则返回 0
```

说明：`sockwant` 与 `socknext` 通过 `event.data.ptr`  来注册、传递有读写事件的 `Socket` 对象。

### 4.2 流程分析

从 main.c 入手，主要分析服务端的 `srvserve` 实现：

```c
void srvserve(Server *s)
{
    // 1. 创建 epoll fd 和 server Socket
    if (sockinit() == -1) exit(1);

    // 2. 为 server Socket 注册读事件，准备 accept 连接
    s->sock.x = s;
    s->sock.f = (Handle)srvaccept; // server Socket 的 handler
    s->conns.less = conn_less;
    s->conns.setpos = conn_setpos;
    if (sockwant(&s->sock, 'r') == -1)  exit(2);

    // 3. 阻塞执行 event loop
    Socket *sock; // sock 指向 Socket，有事件时才能在 server Socket 和 client Socket 切换
    for (;;) {
        // 3.1 计算本次 epoll wait 的超时时间
        // 同时负责执行核心的定时任务逻辑，比如将 delay 已过期的 job 放入 ready 队列，处理超时连接等等
        int64 period = prottick(s);

        // 4.2 等待 sock 上发生的事件
        int rw = socknext(&sock, period);
        if (rw == -1) exit(1);

        // 4.3 调用对应的 handler 处理读写事件
        if (rw) sock->f(sock->x, rw);
    }
}
```

其中，`srvaccept` handler 负责 accept 新连接，并制定其 handler `prothandle`：

```c
// fd: client unix socket fd
// which: 'r' 'w' 'h' 等事件
// c: client Conn
static void h_conn(const int fd, const short which, Conn *c)
{
    if (fd != c->sock.fd) {
        twarnx("Argh! event fd doesn't match conn fd.");
        close(fd);
        connclose(c);
        epollq_apply();
        return;
    }
    if (which == 'h') {
        c->halfclosed = 1;
    }

    // 1. 根据 conn 的 state 指导下一步 IO 操作
    conn_process_io(c);

    // 2. 读取到命令后分发给各 cmd handler
    while (cmd_data_ready(c) && (c->cmd_len = scan_line_end(c->cmd, c->cmd_read))) {
        dispatch_cmd(c);
        fill_extra_data(c);
    }
    if (c->state == STATE_CLOSE) {
        epollq_rmconn(c);
        connclose(c);
    }
    epollq_apply();
}
```

各 cmd handler 又与各数据结构交互，最终实现了各种 feature，细节不再赘述。



## 5. 总结

尽管 beanstalk 是单线程，但使用了 epoll 进行事件处理，仍能高效应对高并发场景。此外高效利用合适的数据结构实现 feature，IO 读写时的缓存 trick，都很值得学习。