---
title: Lab3A. 基于 Raft 实现容错的 kvDB
date: 2019-06-01 12:58:02
tags: 分布式系统
---

总结 6.824 Lab3A kvDB 的实验笔记。

<!-- more -->

## Lab3A

Lab3 的目标是基于 Raft 实现容错的 key-value DB 集群：3A 处理节点容错，3B 实现日志快照。

### 交互流程

阅读 [lecture](https://pdos.csail.mit.edu/6.824/labs/lab-kvraft.html) 可知，Clerk 是客户端，KVServers 即 kvDBs（状态机），每台 KVServer 即一个 Raft 节点，依靠 Raft 协议保证底层的日志一致性，流程交互图：

 <img src="https://images.yinzige.com/2019-06-03-002335.png" width=80% />

- Client 将 `Put`/`Append`/`Get` 命令发送给集群 Leader 处理，并等待调用返回。
- KVServer1 底层的 Raft 模块会向 follower 发起命令日志的复制。
- 当复制副本达到大多数后，KVServer1 执行该命令，并将结果响应给 Client

Raft 模块在 Lab2 已实现，本节将用到以下开放的接口：

```go
// 发起命令的复制
// idx 是命令复制成功后，其在各节点日志中的索引
// isLeader 表明当前节点是否为 leader
func (rf *Raft) Start(command interface{}) (idx int, term int, isLeader bool)
```

### 线性一致性

linearizability 可理解为 CAP 理论中的 C(Consistency)，意为：

> A call must observe the effects of all calls that have completed before the call starts

如下 4 个 Client 分别在不同时间向 KV 集群发起 4 个命令，蓝线是集群处理命令的时间点。如下 Get 命令的执行结果严格按时间受 Put 命令的影响，即系统满足线性一致性：

 <img src="https://images.yinzige.com/2019-06-03-010109.png" width=70% />

参考：[anishathalye.com](https://www.anishathalye.com/2017/06/04/testing-distributed-systems-for-linearizability/)



### 测试用例

- **TestBasic3A**：正常情况下，保证单个 Client 命令能执行成功，保证 5 台 KVServer 日志一致。
- **TestUnreliable3A**：处理 RPC 调用超时，重试请求。
- **TestOnePartition3A**：处理多台 Client 和多台 Server 都发生网络分区的情况。
- **TestPersistPartitionUnreliableLinearizable3A**：在节点失效、网络不可靠的环境中保证线性一致性。

测试均通过：

 <img src="https://images.yinzige.com/2019-06-03-004945.png" width=60% />



## Client

Client 需记录已知的 leader 位置，下次直接向该节点发起请求。Client 结构如下：

```go
type Clerk struct {
    servers []*labrpc.ClientEnd // kv servers / raft peers
    leader  int                 // latest known leader
    cid     int64               // client id
    seq     int32               // latest request seq num
}
```

clientid 在初始化时调用 `nrand()` 随机生成即可，生产系统中可用 `ip:port` 来唯一标识。

### 检测重复请求

Client 向 KVServer 发起 RPC 调用，当调用超时或被告知节点不是 Leader 后，需换个及诶点重试请求。因此，KVServer 要避免二次执行命令，或因网络延迟使执行过期命令。
参考论文第八节：为检测重复请求，可在每次请求中加入唯一 id，并随请求自增，再重试时使用同一 id，Server 只需**对每个 Client 记录最大的请求 id**，即可排除过期或重复请求。Request 结构如下：

```go
type PutAppendArgs struct {
    Cid   int64 // client id
    Seq   int32 // request sequential number
    Key   string
    Value string
    Op    string // Put/Append
}
```



### 发起请求

对于 Get 请求本身是幂等的，无需加 id 标识。对于 Put/Append 操作则需唯一标识：

```go
func (ck *Clerk) PutAppend(key string, value string, op string) {
    curSeq := atomic.AddInt32(&ck.seq, 1)
    for {
        args := PutAppendArgs{
            Cid:   ck.cid,
            Seq:   curSeq,
            Key:   key,
            Value: value,
            Op:    op,
        }
        var reply PutAppendReply
        ok := ck.servers[ck.leader].Call("KVServer.PutAppend", &args, &reply)

        if !ok || reply.WrongLeader { // RPC call timeout, or ck.leader isn't current leader
            ck.changeLeader()
            continue // retry and re-use current sequential number
        }
        return
    }
}
```



## KVServer

数据库的 key, value 都是 `string` 类型，可直接使用 `map[string]string` 存储，为避免并发读写还需加锁保护。KVServer 结构如下：

```go
type KVServer struct {
    mu      sync.Mutex
    me      int
    rf      *raft.Raft
    applyCh chan raft.ApplyMsg

    maxraftstate int               // snapshot if log grows this big
    db           map[string]string // kvDB
    cid2seq      map[int64]int32   // client id to max request sequential number
    agreeChs     map[int]chan Op   // command index to op channel
    killCh       chan struct{}     // kill KVServer
}
```



Raft 复制的日志需记录具体的某次请求：

```go
type Op struct {
    Cid   int64  // client id
    Seq   int32  // request sequence number
    Cmd   string // command type, Put/Append/Get
    Key   string
    Value string
}
```

由于日志可能被新 leader 覆盖，所以当 KVServer 发现统一索引上，自己发出的 Op 和 Raft 返回的 Op 不一致，就说明同步过程中，我已不再是 leader 且日志已被覆盖：

```go
func isSameOp(a, b Op) bool {
    return a.Cid == b.Cid && a.Seq == b.Seq && a.Cmd == b.Cmd && a.Key == b.Key && a.Value == b.Value
}
```



### 异步复制

根据 [Guide](https://thesquareplanet.com/blog/students-guide-to-raft/#applications-on-top-of-raft) 提示，KVServer 调用 `Start(command)` 发起同步后，需**异步等待** Raft 模块从 `applyCh` 通知已复制成功的日志 `index`，再响应 index 对应的请求。在初始化时在后台开启 goroutine 监听：

```go
// wait agreement from Raft
func (kv *KVServer) waitAgree() {
    for {
        select {
        case <-kv.killCh:
            return
        case msg := <-kv.applyCh:
            op := msg.Command.(Op)
            kv.mu.Lock()
            maxSeq, ok := kv.cid2seq[op.Cid]
            if !ok || op.Seq > maxSeq { // only handle new request from specific client
                kv.cid2seq[op.Cid] = op.Seq
                switch op.Cmd {
                case "Put":
                    kv.db[op.Key] = op.Value
                case "Append":
                    kv.db[op.Key] += op.Value
                }
            }
            kv.mu.Unlock()

            kv.getAgreeCh(msg.CommandIndex) <- op
        }
    }
}
```

注意：由于请求处理与 waitAgree 监听的执行顺序是不确定的，需有一个共用 agreeCh 的逻辑：

```go
func (kv *KVServer) getAgreeCh(idx int) chan Op {
    kv.mu.Lock()
    defer kv.mu.Unlock()

    ch, ok := kv.agreeChs[idx]
    if !ok {
        ch = make(chan Op, 1) // never block this
        kv.agreeChs[idx] = ch
    }
    return ch
}
```



### 处理 Get 请求

为避免过期 leader 返回旧数据，在处理 Get 请求前，leader 必须与集群中大多数节点完成通信，确保自己的数据是最新的。论文第八节建议让 leader 主动发起一次心跳并统计正常节点数量，不过根据 lecture 提示：

> A kvserver should not complete a `Get()` RPC if it is not part of a majority (so that it does not serve stale data). A simple solution is to enter every `Get()` (as well as each `Put()` and `Append()`) in the Raft log

可让 Get 请求像 Put/Append 请求一样走日志同步流程，就不必再修改 Lab2 的 Raft 实现。请求处理流程：

```go
func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
    cmd := Op{Cmd: "Get", Key: args.Key,}
    idx, _, isLeader := kv.rf.Start(cmd)
    if !isLeader {
        reply.WrongLeader = true
        return
    }

    ch := kv.getAgreeCh(idx)
    var op Op
    select {
    case op = <-ch: // current leader can communicate with majority
        close(ch)
    case <-time.After(500 * time.Millisecond): // agreement may be failed, treat as timeout and client will retry
        reply.WrongLeader = true
        return
    }

  // if old leader has in net partition, may it's log may be overwrited, then reply value will be different
    if !isSameOp(cmd, op) {
        reply.WrongLeader = true
        return
    }
    
    kv.mu.Lock()
    reply.Value = kv.db[args.Key] // if key not exist, just return "" or return ErrNoKey
    kv.mu.Unlock()
}
```



### 处理 Put/Append 请求

由于 Put/Append 请求会更新 `kv.db` 数据，要避免重复请求被二次执行，即 waitAgree 中的 Seq 对比逻辑。

```go
func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
    cmd := Op{
        Cid:   args.Cid,
        Seq:   args.Seq,
        Cmd:   args.Op,
        Key:   args.Key,
        Value: args.Value,
    }
    idx, _, isLeader := kv.rf.Start(cmd)
    if !isLeader {
        reply.WrongLeader = true
        return
    }

    ch := kv.getAgreeCh(idx) // sequence of PutAppend() and <-applyCh are uncertain
    var op Op
    select {
    case op = <-ch:
        close(ch)
    case <-time.After(500 * time.Millisecond):
        reply.WrongLeader = true
        return
    }

    if !isSameOp(cmd, op) {
        reply.WrongLeader = true
        return
    }
}
```

至此完成了基于 Raft 实现容错 kvDB 的搭建。



## 总结

在 Lab2 中实现的 Raft 库开放了 `applyCh` 和 `Start(command)` 接口，本节在此基础上实现异步监听、超时重试、请求去重等机制，使上层的 kvDB 能在主机崩溃重启，请求发生延迟、失序、丢失甚至隔离的网络环境下，依旧能对客户端保证数据的线性一致性。

