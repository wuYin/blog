---
title: Lab3B. Raft 日志压缩及数据快照
date: 2019-06-15 10:40:12
tags: 分布式系统
---

总结 MIT6.824 Lab3B Log Compaction 实验笔记。

<!-- more -->

## Lab3B

### 日志过大的问题

在 3A 部分，`Put`/`Append`/`Get` 等命令的日志只通过 leader 流向 followers，来维护节点间 kvDB 数据的一致性。但随着 Client 请求的增多，各节点的日志将占用更多空间。日志过长会导致：
- 当发生日志冲突时，follower 从头到尾查找冲突任期的首条日志将更耗时。
- 某节点因长时间的网络隔离导致日志过于落后，或新节点加入集群，replay 日志将更耗时。

以上多种耗时操作最终会**降低集群的可用性**，本节目标就是想办法减少日志长度，将大小控制在可控范围内。

### 日志压缩与数据快照

参考论文第 7 节，Raft 最终选择快照机制来压缩日志大小，如下的 1-5 **committed** 状态的日志被压缩为快照，最终使长度为 7 的日志 = 长度为 2 的日志 + 快照数据。快照数据分为两部分：

- Raft 的快照元信息：LastIncludedIndex 与 LastIncludedTerm，用于生成快照后的 AppendEntries 一致性检查。
- kvDB 的状态：生成快照时的数据库状态，用于节点重启后恢复数据。

 <img src="https://images.yinzige.com/2019-06-13-160714.png" width=60% />

如上，压缩日志的本质是用**某条 committed 日志代替之前的所有 committed 日志**。所以生成快照会删除 Raft 的日志：`raft.logs = rf.logs[snapshotIndex:]`，于是要回头修改 Lab2 中基于日志长度、取日志的操作，需要把快照索引的偏移量加回去。所以需要将快照信息放入 Raft 结构中：

```go
type Raft struct {
    // ...
    // snapshot
    lastIncludedIndex int // the snapshot replaces all entries up through and including this index
    lastIncludedTerm  int // term of lastIncludedIndex
}
```

添加几个公用的日志偏移量计算方法，并替换那些直接对 rf.logs 的读操作：

```go
func (rf *Raft) getLog(i int) LogEntry {
    return rf.logs[i-rf.lastIncludedIndex]
}

func (rf *Raft) addIdx(i int) int {
    return rf.lastIncludedIndex + i
}

func (rf *Raft) subIdx(i int) int {
    return i - rf.lastIncludedIndex
}

func (rf *Raft) lastIdx() int {
    return rf.lastIncludedIndex + len(rf.logs) - 1
}

func (rf *Raft) lastTerm() int {
    return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) logLength() int {
    return rf.lastIdx() + 1
}
```



### 交互流程

![image-20190612093216321](https://images.yinzige.com/2019-06-12-013217.png)

1. 任意节点都要实时检测 Raft 模块的日志大小。
2. 若日志大小超过临界值，将 kvDB 数据发送给 Raft 模块用于生成快照。
3. Raft 将快照日志、kvDB 均持久化到 persister（模拟磁盘）
4. leader 在心跳时若发现有新快照则同步，切换 AppendEntries RPC 为 InstallSnaoshot RPC
5. follower 收到快照后，与本地快照及日志对比，决定进行日志覆盖或删除。
6. follower 将新快照数据持久化。
7. follower 将 leader 的快照数据回传给上层的 KVServer2，重置 kvDB 使状态机数据达成一致。

#### 流程划分

 1-2-3：各节点在日志过多时独立生成快照。leader 的存在是为了解决冲突维护日志的一致性，在 follower 生成快照时日志本身就是 **committed** 状态的，这并不违反强 leader 原则。

4-5-6-7：仅 leader 操作。它通过 InstallSnapshot 将自己的快照同步给各 follower，使日志过于落后的 follower 快速达成一致。



### 测试用例

- **TestSnapshotRPC3B**：三个节点集群，若其中一个长时间网络隔离后重新加入集群，要能通过 InstallSnapshot RPC 加速日志回放，其他两个节点要能独立生成快照，使日志大小不超过 1000 字节。
- **TestSnapshotRecover3B**：节点重启后要能从 persister 中恢复快照数据。
- 其他测试：在网络分区、节点崩溃重启的环境下保持 kvDB 数据的线性一致性。

测试通过：

![image-20190615153120071](https://images.yinzige.com/2019-06-15-073120.png)



## KVServer 生成快照

认真阅读并思考：[Lecture: Part B](https://pdos.csail.mit.edu/6.824/labs/lab-kvraft.html) 和 [Guide: An aside on optimizations](https://thesquareplanet.com/blog/students-guide-to-raft/#an-aside-on-optimizations)

### 快照数据

kvServer 在重启后会先从 persister 中读取快照信息重置 kvDB，同时为检测 Client 的重复请求，所以必须将 `kv.cid2seq` 一起持久化：

```go
func (kv *KVServer) encodeSnapshot() []byte {
    w := new(bytes.Buffer)
    e := gob.NewEncoder(w)
    if err := e.Encode(kv.cid2seq); err != nil {
        panic(fmt.Errorf("encode cid2seq fail: %v", err))
    }
    if err := e.Encode(kv.db); err != nil {
        panic(fmt.Errorf("encode db fail: %v", err))
    }
    return w.Bytes()
}
```

### 检测 Raft 日志大小

每次 kvServer 的 Raft 模块通知它有新日志达成一致，就意味着 Raft 的日志又新增了一条，就检测 Raft 的日志是否接近边界值。注意 lab 中的 `maxraftstate` 是日志大小的上限，要在快接近时提前生成快照，因为调用 Raft 模块生成快照抢锁是需要等待的，若不提前就很可能超出上限，无法通过测试。所以取 90% 提前生成：

```go
func (kv *KVServer) waitAgree() {
    for {
        select {
        case <-kv.killCh:
            return
        case msg := <-kv.applyCh:
            op := msg.Command.(Op)
            kv.mu.Lock()
            // ...
            kv.checkSnapshot(msg.CommandIndex) // use committed index as snapshot LastIncludedIndex
            kv.mu.Unlock()
        }
    }
}

func (kv *KVServer) checkSnapshot(appliedId int) {
    if kv.maxraftstate == -1 {
        return
    }
    // take snapshot when raft size come near upper limit
    if kv.persister.RaftStateSize() < kv.maxraftstate*9/10 {
        return
    }
  
    rawSnapshot := kv.encodeSnapshot()
    go kv.rf.TakeSnapshot(appliedId, rawSnapshot) // not take long time with KVServer's lock
}
```

### 通知 Raft 同步快照

kvServer 需告知 Raft 模块以哪条 committed 日志为准来生成快照，由于 applyCh 的 msg 都是 committed 的，所以如上直接使用它的 `msg.CommandIndex` 即可。

Raft 收到生成快照的请求后更新完自己的快照信息就立刻返回，不要阻塞，快照同步交给心跳去处理：

```go
// leader take snapshot should be async like Start(), must return quickly
func (rf *Raft) TakeSnapshot(appliedId int, rawSnapshot []byte) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    // lock competition may delayed snapshot call, check this otherwise rf.logs[0] may out of bounds
    if appliedId <= rf.lastIncludedIndex {
        return
    }

    // discard the entries before that index, preserved it for AppendEntries consistency check
    rf.logs = rf.logs[rf.subIdx(appliedId):]
    rf.lastIncludedIndex = appliedId
    rf.lastIncludedTerm = rf.logs[0].Term
    rf.persistStatesAndSnapshot(rawSnapshot)
}
```

KVServer 多次调用 Raft.TakeSnapshot() 的执行顺序会收到 Raft 抢占锁的影响，可能新快照会先抢到锁，轮到旧日志处理时对应的 appliedId 日志已被删除，会造成 `rf.logs[0]` 溢出，所以要过滤旧快照。



## Raft 同步快照

### Leader 使用快照代替日志加速同步

在 Lab2 中已实现 leader 向各 follower 发送缺失日志的心跳逻辑，引入快照机制后，可直接发送快照。当leader 发现快照比 nextIndex 还新，就将 AppendEntries RPC 换成 InstallSnapshot RPC，大大减少要同步的日志数量。

如下leader 只需发送 6 一条日志，而非 3 4 5 6 四条：

 <img src="https://images.yinzige.com/2019-06-14-152901.png" width=80% />

在 Leader 给 follower 同步日志前，检查快照若更新则走 InstallSnapshot RPC 的逻辑：

```go
// leader sync logs to followers
func (rf *Raft) sync() {
    for i := range rf.peers {
        if i == rf.me {
            continue
        }

        go func(server int) {
            for {
                if !rf.isRunningLeader() {
                    return
                }

                rf.mu.Lock()
                rf.syncConds[server].Wait() // wait for trigger

                // sync new log or missing logs to server
                next := rf.nextIndex[server]

                // if follower far behind from leader, just send snapshot to it for speeding up replay
                if next <= rf.lastIncludedIndex {
                    rf.syncSnapshot(server) // InstallSnapshot RPC logic
                    continue
                }
          
               // AppendEntries RPC logic
    }
}      
```



### InstallSnapshot RPC

参考论文的图 13，RPC 参数如下：

```go
type InstallSnapshotArgs struct {
    Term              int    // leader's term
    LeaderId          int    // so follower can redirect clients
    LastIncludedIndex int    // the snapshot replaces all entries up through and including this index
    LastIncludedTerm  int    // term of lastIncludedIndex
    Data              []byte // raw bytes of the snapshot chunk, starting at offset
    // Offset            int    // byte offset where chunk is positioned in the snapshot file
    // Done              bool   // true if this is the last chunk
}

type InstallSnapshotReply struct {
    Term int // currentTerm, for leader to update itself
}
```

lab 为简化流程，每次直接发送整个快照，而非论文中切割为 chunk 分块有序同步，所以 offset 取 0，done 取 true

#### Leader 端

```go
// sync snap shot to follower server
// rf.mu is locked when call syncSnapshot()
func (rf *Raft) syncSnapshot(server int) {
    if rf.state != Leader || rf.crashed {
        rf.mu.Unlock()
        return
    }

    args := InstallSnapshotArgs{
        Term:              rf.curTerm,
        LeaderId:          rf.me,
        LastIncludedIndex: rf.lastIncludedIndex,
        LastIncludedTerm:  rf.lastIncludedTerm,
        Data:              rf.persister.ReadSnapshot(),
    }
    rf.mu.Unlock()

    var reply InstallSnapshotReply
    respCh := make(chan struct{})
    go func() {
        if ok := rf.sendInstallSnapshot(server, &args, &reply); ok {
            respCh <- struct{}{}
        }
    }()
    select {
    case <-time.After(RPC_CALL_TIMEOUT):
        return
    case <-respCh:
        close(respCh)
    }

    rf.mu.Lock()
    defer rf.mu.Unlock()
    if reply.Term > rf.curTerm {
        rf.back2Follower(reply.Term, VOTE_NIL)
        return
    }
    if rf.state != Leader || reply.Term < rf.curTerm { // curTerm changed already
        return
    }

    rf.matchIndex[server] = args.LastIncludedIndex
    rf.nextIndex[server] = args.LastIncludedIndex + 1
}
```

注意在 syncSnapshot 中调用 RPC 前及时释放锁，否则同一时间只能发起一个调用。

#### Follower 端

```go
// follower receive snapshot from leader and force overwrite local logs
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
    rf.mu.Lock()
    defer rf.mu.Unlock()
    reply.Term = rf.curTerm

    // 1. reply false if term < currentTerm
    if args.Term < rf.curTerm {
        return
    }
    if args.Term > rf.curTerm {
        reply.Term = args.Term
        rf.back2Follower(args.Term, VOTE_NIL)
    }
    rf.resetElectTimer()

    // check snapshot may expired by lock competition, otherwise rf.logs may overflow below
    if args.LastIncludedIndex <= rf.lastIncludedIndex {
        return
    }

    // 2. Create new snapshot file if first chunk (offset is 0)
    // 3. Write data into snapshot file at given offset
    // 4. Reply and wait for more data chunks if done is false
    // 5. Save snapshot file, discard any existing or partial snapshot with a smaller index

    // 6. If existing log entry has same index and term as snapshot's last included entry, retain log entries following it and reply
    if args.LastIncludedIndex < rf.lastIdx() {
        // the args.LastIncludedIndex log has agreed, if there are more logs, just retain them
        rf.logs = rf.logs[args.LastIncludedIndex-rf.lastIncludedIndex:]
    } else {
        // 7. Discard the entire log
        // empty log use for AppendEntries RPC consistency check
        rf.logs = []LogEntry{{Term: args.LastIncludedTerm, Command: nil}}
    }

    // update snapshot state and persist them
    rf.lastIncludedIndex = args.LastIncludedIndex
    rf.lastIncludedTerm = args.LastIncludedTerm
    rf.persistStatesAndSnapshot(args.Data)

    // force the follower's log catch up with leader
    rf.commitIndex = max(rf.commitIndex, args.LastIncludedIndex)
    rf.lastApplied = max(rf.lastApplied, args.LastIncludedIndex)

    // 8. Reset state machine using snapshot contents (and load snapshot's cluster configuration)
    rf.applyCh <- ApplyMsg{
        CommandValid: false, // it's snapshot raw data, not a command
        CommandIndex: -1,
        Command:      args.Data, // use for KVServer restore kvDB
    }
}
```

对应修改 KVServer 监听 applyCh 的逻辑，从中取出 leader 发来的快照数据：

```go
func (kv *KVServer) waitAgree() {
    for {
        select {
        case <-kv.killCh:
            return
        case msg := <-kv.applyCh:
            if !msg.CommandValid { // snapshot data
                buf := msg.Command.([]byte)
                kv.mu.Lock()
                kv.db, kv.cid2seq = kv.decodeSnapshot(buf) // restore kvDB and cid2seq
                kv.mu.Unlock()
                continue
            }
            
      // log agreement loginc
        }
    }
}
```

至此分别完成了三件事：

- 各节点能实时检测 Raft 日志大小，独立地生成快照并持久化，重启时读取。
- leader 生成快照后通过心跳发起 InstallSnapshot RPC 给 followers 加速日志回放和同步。
- followers 将快照数据回传给上层的 KVServer 重置 kvDB 和 cid2seq，最终状态与 leader 一致。



## 总结

在 Lab2 中实现了 Raft 算法的三个子模块，开放了 `Start()` 和 `applyCh` 供状态机使用。

Lab3A 在其 Lab2 基础上实现了容错的 kvDB 集群，依靠底层 Raft 算法在节点崩溃重启甚至不可用、网络延迟丢包甚至分区的环境下，依旧对多个 Client 保证数据的线性一致性。

Lab3B 实现了 Raft 日志大小的实时检测并截断生成快照，由 leader 通过 InstallSnapshot RPC 将日志同步给过于落后的节点来加速回放，同时重置各状态机（kvDB）的状态，使其数据与 leader 最达成一致。

Lab3 比 Lab2 稍简单，关键在于梳理好 KVServer 与底层的 Raft 模块的交互流程，需重读 Raft 论文的第 7、8 节的快照机制，思考 lecture 的提示。调试时活锁、状态更新时机有误等问题居多，其余细节在代码注释中有标注。