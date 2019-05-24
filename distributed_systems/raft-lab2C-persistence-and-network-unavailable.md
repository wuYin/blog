---
title: Lab2C. Raft 日志备份与不可靠网络处理
date: 2019-05-23 19:34:18
tags: 分布式系统
---

总结 MIT6.824 Lab2C Log Persistence 实验笔记。

<!-- more -->

## Lab2C

参考 [lecture](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html) 和注释，可从测试用例着手实现日志备份。测试用例：

- **TestPersistN2C**：持久化日志、任期等节点状态数据，需实现 `Kill()` 来模拟节点 crash 和重启。
- **TestFigure82C**：模拟原论文图 8，即 leader 不能通过计数副本来 commit 旧任期的日志，而只能 commit 在当前任期日志，再根据 log matching 原则顺带提交旧日志。
- **TestUnreliableAgree2C**：模拟网络不可靠，请求会延迟甚至丢弃，但最终要达成一致。
- **TestFigure8Unreliable2C**：模拟图 8 的请求乱序，节点频繁崩溃与重启，但最终要达成一致。

测试通过：
 <img src="https://images.yinzige.com/2019-05-23-113554.png" width=70% />

## 实现思路

2C 分为两个部分：日志状态的持久化、在不可靠网络下达成最终一致性。

### 日志状态的持久化

lab 通过封装 gob 对象 persister 来模拟数据持久化到磁盘的过程。根据论文，需持久化的状态有 3 个：

 <img src="https://images.yinzige.com/2019-05-23-121246.png" width=50% />

由于节点随时可能 crash，更新这三个状态后要重新持久化，触发时机有三个：
- 发起 vote 时
- 节点在 AppendEntries RPC 中 append 新日志后
- 节点在 RPC 中发现更大 term 需回退到 follower 时

根据注释提示很好实现。同时要修改与 `log[]` 相关的 `nextIndex[]` 的初始化：

```go
rf.readPersist(persister.ReadRaftState()) // initialize from state persisted before a crash
rf.nextIndex = make([]int, len(rf.peers))
for i := range rf.peers {
  rf.nextIndex[i] = len(rf.logs) // initialized to leader last log index + 1
}
```

### 网络不可靠
**节点发生网络隔离：**
Raft 通过选举时 up-to-date 检查、同步日志时一致性检查等限制来保证 Safety，即只要集群中大多数节点正常且能相互通信，就能保证集群对外可用，保持数据一致性，两种检查要严格按照论文图 2 进行实现。

**RPC 请求和响应包发生延迟、乱序、重复和丢失：**
对原论文中未提及太多，要认真参考[Students' Guide to Raft](https://thesquareplanet.com/blog/students-guide-to-raft/) 并将建议落实到代码中。



## Student Guide

原文是 6.824 的课程助教写的，指出了很多学生经常犯的错误。这些细节如果不注意，在不可靠网络中很可能无法达成一致。如下详述 Guide 中我忽视掉的细节。

### 重置选举超时 timer

我在 Lab2B 中觉得应重置 timer 的地方都调用 `rf.restElectTimer()`，然而根据 guide 提示仔细看了论文：

> If election timeout elapses without receiving `AppendEntries` RPC *from current leader* or *granting* vote to candidate: convert to candidate.

如上，节点只会在 3 个时机重置选举 timer
- get an `AppendEntries` RPC from the *current* leader（`args.Term >= rf.curTerm`）
- you are starting an election
- you *grant* a vote to another peer

 <img src="https://images.yinzige.com/2019-05-24-064715.png" width=75% />

Raft 的实现一定要严格遵从论文图 2 所述规则，否则会踩很多坑，还不好复现调试。

### LiveLocks

在 Lab2B 中，leader 有一个 daemon commit checker，用于在后台检测已经复制成功的新日志：

```go
// leader daemon detect and commit log which has been replicated on majority successfully
func (rf *Raft) leaderCommit() {
    for {
        if !rf.isRunningLeader() {
            return
        }

        rf.mu.Lock()
        majority := len(rf.peers)/2 + 1
        for i := len(rf.logs) - 1; i > rf.commitIndex; i-- { // looking for newest commit index from tail to head
            replicated := 0

            // in current term, if replicated on majority, commit it
            if rf.logs[i].Term == rf.curTerm {
                for server := range rf.peers {
                    if rf.matchIndex[server] >= i {
                        replicated += 1
                    }
                }
            }

            if replicated >= majority { // now preceding logs has been committed
                rf.commitIndex = i       
                rf.applyCond.Broadcast() // trigger to apply [lastApplied+1, commitIndex] logs
                break
            }
        }
        rf.mu.Unlock()
    }
}
```

如上是 2B 中 leader 的 daemon commit checker 机制，让 leader 在后台检测并更新 commitIndex。然而，如上的机制是*错误的*，checker 会极其频繁地抢占锁。回归独占锁的本质：`Lock()` 和 `Unlock()` 间的临界区代码在执行时类似于原子操作，其他未抢到锁的 goroutine 只能等待。而在代码中为避免竞态，在对节点数据进行更新和读取时，都要先加锁再操作，为避免这些操作被频繁拖延和中断，就要避免有线程会频繁地占用锁。

> Guide：Make sure that you check for `commitIndex > lastApplied` either periodically, or after `commitIndex` is updated (i.e., after `matchIndex` is updated)
> Paper：If there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N, and log[N].term == currentTerm: set commitIndex = N (§5.3, §5.4).

periodically check 不好估算间隔，所以在 matchIndex 更新时手动触发 check：

```go
// once leader replicate logs successfully, check commitIndex > lastApplied
func (rf *Raft) updateCommitIndex() {
    n := len(rf.peers)
    matched := make([]int, n)
    copy(matched, rf.matchIndex)
    revSort(matched)
    // newest committed index
    if N := matched[n/2]; N > rf.commitIndex && rf.logs[N].Term == rf.curTerm {
        rf.commitIndex = N
        rf.checkApply()
    }
}

// apply logs in (lastApplied, commitIndex]
func (rf *Raft) checkApply() {
    for rf.lastApplied < rf.commitIndex {
        rf.lastApplied++
        rf.applyCh <- ApplyMsg{
            CommandValid: true,
            Command:      rf.logs[rf.lastApplied].Command,
            CommandIndex: rf.lastApplied,
        }
    }
}
```


### 处理日志冲突

当 follower 收到 AppendEntries RPC 调用后先进行一致性检查，若发现本地缺少日志 / 日志不匹配，则应将冲突信息告知 leader，下次同步时再将冲突的日志发送过来。

#### follower：
> If a follower does not have `prevLogIndex` in its log, it return with `conflictIndex = len(log)` and `conflictTerm = None`
> If a follower does have `prevLogIndex` in its log, but the term does not match, it should return `conflictTerm = log[prevLogIndex].Term`, and then search its log for the first index whose entry has term equal to `conflictTerm`

follower 查找冲突任期内的首条日志：
```go
// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term matches prevLogTerm (§5.3)
last := len(rf.logs) - 1
if last < args.PrevLogIndex { // missing logs
    reply.ConflictTerm = NIL_TERM
    reply.ConflictIndex = last + 1
    return
}

// prevLogIndex log consistency check
prevIdx := args.PrevLogIndex
prevTerm := rf.logs[prevIdx].Term
if prevTerm != args.PrevLogTerm { // term not match
    reply.ConflictTerm = prevTerm
    for i, e := range rf.logs {
        if e.Term == prevTerm {
            reply.ConflictIndex = i // first index of conflict term
            break
        }
    }
    return
}
```

#### leader：
Upon receiving a conflict response, the leader should first search its log for `conflictTerm`
> If it finds an entry in its log with that term, it should set `nextIndex` to be **the one beyond the index of the *last* entry in that term in its log**
> If it does not find an entry with that term, it should set `nextIndex = conflictIndex`

leader 查找冲突任期最后一条日志的下一条日志：
```go
// consistency check failed, use conflict info to back down server's nextIndex
if reply.ConflictIndex > 0 {
    rf.mu.Lock()
    firstConflict := reply.ConflictIndex
    if reply.ConflictTerm != NIL_TERM { // not missing logs
        lastIdx, _ := rf.getLastLog()
        for i := 0; i <= lastIdx; i++ {
            if rf.logs[i].Term != reply.ConflictTerm {
                continue
            }
            for i <= lastIdx && rf.logs[i].Term == reply.ConflictTerm {
                i++ // the last conflict log's next index
            }
            
            firstConflict = i
            break
        }
    }
    rf.nextIndex[server] = firstConflict // next sync, send conflicted logs to the follower
    rf.mu.Unlock()
}
```

#### ConflictIndex 优化

leader 本可以直接 `rf.nextIndex[server] = reply.ConflictIndex`，但是可以减少不必要的日志同步：

> leader will sometimes end up sending more log entries to the follower than is strictly necessary to bring them up to date.

因为 leader 冲突任期内的日志和 follower 一致。如下图，下次同步不必再将 3、4 两条日志发送给 follower：

 <img src="https://images.yinzige.com/2019-05-24-074121.png" width=80% />



### 响应延迟

> From experience, we have found that by far the simplest thing to do is to first record the term in the reply (it may be higher than your current term), and then to compare the current term with the term you sent in your original RPC. If the two are different, drop the reply and return

测试用例中通过 sleep 随机时长模拟了 RPC 响应延迟，很可能出现：
candidate 等待 RequestVote RPC 响应过程中，再次过期开启下一轮选举，待响应返回时 `curTerm` 已自增，此时上一轮选举已无意义，要手动终止上一轮选举。

```go
for reply := range replyCh {
    if reply.Term > rf.curTerm { // higher term leader
        rf.back2Follower(reply.Term, VOTE_NIL)
        return
    }
    if rf.state != Candidate || reply.Term < rf.curTerm { // current term changed already
        return // end last election manually
    }

    // ...
}
```

同理，leader 在等到 AppendEntries RPC 的响应后，也要检查并手动结束该次日志同步：

```go
if !reply.Succ {
    if reply.Term > rf.curTerm {
        rf.back2Follower(reply.Term, VOTE_NIL) // higher term
        return
    }
    if rf.state != Leader || reply.Term < rf.curTerm { // curTerm changed already
        return // end last agreement manually
    }

    // ...
}
```



### 更新 leader 的 nextIndex[] 与 matchIndex[]

> Setting `matchIndex = nextIndex - 1`, or `matchIndex = len(log)` when you receive a response to an RPC. This is *not* safe, because both of those values could have been updated since when you sent the RPC. Instead, the correct thing to do is update `matchIndex` to be `prevLogIndex + len(entries[])`from the arguments you sent in the RPC originally.

nextIndex[] 是 leader 对各节点日志复制情况的大致估计，在冲突时可能会回滚。而 matchIndex 是对日志复制的精确记录，必须要准确地更新。当 AppendEntries RPC 调用成功后：

```go
if len(args.Entries) > 0 {
    rf.mu.Lock()
    rf.nextIndex[server] += len(args.Entries) // this assuming state no change between sent RPC and get reply, not safe obviously
    rf.matchIndex[server] += rf.nextIndex[server]-1
    rf.updateCommitIndex()
    rf.mu.Unlock()
}
```

如上的更新前提是于 RPC 请求与响应期间 leader 状态不变，显然不正确的。应根据此次请求参数进行更新：

```go
if len(args.Entries) > 0 {
    rf.matchIndex[server] = args.PrevLogIndex + len(args.Entries) // conservative measurement of log agreement
    rf.nextIndex[server] = rf.matchIndex[server] + 1              // just guess for agreement, maybe move backwards
    rf.updateCommitIndex()
    rf.mu.Unlock()
}
```

Lab2C 的网络不可用测试是整个 lab2 最棘手的部分，实验前要认真读 lecture，参考 [raft-structure](https://pdos.csail.mit.edu/6.824/labs/raft-structure.txt) 和 [student guide](https://thesquareplanet.com/blog/students-guide-to-raft/)，提到的有些细节和很重要，但也很容易被忽视。



## 总结

至此整个 Lab2 撒花完结，工作之余调试了近一个月，前后重构了三次。为了来年二刷少踩点坑，总结几条经验：
- lab 的实现**必须严格遵从**原论文的图 2：保证 State 在正确的时机更新，保证两种 RPC 的处理逻辑正确，保证节点准守 Rules for Servers
- 区分好 `nextIndex[]` 和 `matchIndex[]`，前者是对复制日志的乐观估计，后者是复制日志的精确记录，一定要保证后者的正确更新。
- 由于网络不可靠，在收到 RPC 响应后，检查 term 若已过期要放弃本次处理。RPC 调用可能超时，要么把每个 RPC 放入单独的 goroutine 处理，要么自己控制超时时间。
- 避免 livelocks 扰乱 Raft 的时序性。
- 确保用 `go test -race -count` 测试多次看是否都能通过。

做完 Lab2 最大的收获：更加深刻地理解了 Raft 算法的 leader election, log replication, safety，对 Go 的 channel, sync.Mutex, sync.Cond 实现同步异步也更加顺手。

感谢 MIT