---
title: Lab2B. Raft 日志复制实现
date: 2019-05-09 19:12:41
tags: 分布式系统
---

总结 MIT6.824 Lab2B Log Replication 实验笔记。

<!-- more -->

## Lab2B

### 测试用例

2A 部分完成了基础的 Leader Election 和 Heartbeat 机制，2B 部分要完成 Log Replication，同时实现论文中 5.4.1 节选举限制机制来保证选举的安全性。 本节实验目标是通过 `test_test.go` 中的 *2B 测试用例：
- **TestBasicAgree2B：实现最简单的日志复制**
  对 leader 请求执行命令，节点均正常的情况下日志要能达成一致。
- **TestFailAgree2B：处理少部分节点失效**
  三个节点组成的集群中，某个普通节点发生了网络分区后，剩余两个节点要能继续 commit 和 apply 命令，当该隔离节点网络恢复后，要能正确处理它的 higher term
- **TestFailNoAgree2B：处理大部分节点失效**
  在五个节点组成的集群中，若有三个节点失效，则 leader 处理的新命令都是 uncommit 的状态，也就不会 apply，但当三个节点的网络恢复后，要能根据日志新旧正确处理选举。
- **TestConcurrentStarts2B：处理并发的命令请求**
  在多个命令并发请求时，leader 要保证每次只能完整处理一条命令，不能因为并发导致有命令漏处理。
- **TestRejoin2B：处理过期 leader 提交的命令**
  过期 leader 本地有 uncommit 的旧日志，在 AppendEntries RPC 做日志一致性检查时进行日志的强制同步。这是最棘手的测试，其流程如下：
   <img src="https://images.yinzige.com/2019-05-09-122249.png" width=80% />
- **TestBackup2B：性能测试**
  在少部分节点失效、多部分节点失效环境下，尽快完成两百个命令的正确处理。
- **TestCount2B：检查无效通信的次数**
  正常情况下，超时无效的 RPC 调用不能过多。

测试均通过（但 backup 待优化）：
![image-20190509203922666](https://images.yinzige.com/2019-05-10-085732.png)

### 实现思路

![image-20190510083949269](https://images.yinzige.com/2019-05-10-003949.png)

#### 整体流程
- client 将命令发送给 leader 后，leader 先本地 append 日志后立刻响应（lab 与 paper 此处有差异），随后广播给所有其他节点的 sync trigger，主动触发日志复制。
- follower 收到日志后进行一致性检查，强制覆写冲突日志并 append 新日志，通知 leader 复制成功。
- leader 在后台统计当前任期的日志复制成功的节点数量，若达到 majority 则将日志标记为 commit 状态并通知 apply
- 在之后的心跳请求中，leader 将自己的 commitIndex 一并同步，follower 发现自己的 commitIndex 落后，随即更新，通知 apply

#### 关键点
- 正如 lecture 的提示，实现时需要大量的同步触发机制，可选择 Go 的阻塞 channel 或 sync 包的条件变量。其中，阻塞 channel 使用不当可能会造成死锁或资源泄漏，而且触发点很多，会造成一个 channel 满天飞的情况，遂选择条件变量做同步。
- leader 要实现上图三个 daemon 机制，而 follower 只需要实现 apply checker
  - leader 的 sync trigger：新日志 append 或心跳通信都会触发
  - leader 的 commit checker：一直通过 matchIndex 检测日志的 commit 状态
  - leader、follower 的 apply checker：当确定某条日志未 commit 状态时触发 apply 执行
- 充分理解论文的图 2：lastApplied 和 commitIndex，nextIndex[] 和 matchIndex[] 都将用于复制机制。



## 日志复制

### 日志结构

Raft 的目标是在大多数节点都可用且能相互通信的前提下，保证多个节点上日志的一致性。日志的存储结构：

```go
type LogEntry struct {
	Term    int          
	Command interface{}
}
```

Term 是 Raft 协议的逻辑时钟，用于检查日志的一致性。它有三种状态：
- commit / committed：当日志被成功 replicated 到大多数节点后的状态
- apply / applied：日志已处于 commit 状态后，即可直接 apply 执行
- uncommit：日志因网络分区等原因未成功复制到大多数节点，停留在 leader 内的状态


### AppendEntries RPC

leader 通过 AppendEntries RPC 与各节点进行日志的同步。请求参数和响应参数如下：

```go
type AppendEntriesArgs struct {
	Term         int        // leader term
	LeaderID     int        // so follower can redirect clients
	PrevLogIndex int        // index of log entry immediately preceding new ones
	PrevLogTerm  int        // term of prevLogIndex entry
	Entries      []LogEntry // log entries to store (empty for heartbeat;may send more than one for efficiency)
	LeaderCommit int        // leader’s commitIndex
}

type AppendEntriesReply struct {
	Term          int  // currentTerm, for leader to update itself
	Succ          bool // true if follower contained entry matching prevLogIndex and prevLogTerm
}
```



### 被调用方（Servers）

参考论文图 2，当节点收到此调用后，依次进行五个判断：
- Reply false if term < currentTerm
- Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
- If an existing entry conflicts with a new one (same index but different terms), delete the existing entry and all that follow it
- Append any new entries not already in the log
- If leaderCommit > commitIndex, set commitIndex =min(leaderCommit, index of last new entry)

```go
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	reply.Term = rf.curTerm
	reply.Succ = false

	if args.Term < rf.curTerm {
		return // leader expired
	}

	if args.Term > rf.curTerm {
		rf.curTerm = args.Term
		rf.back2Follower(args.Term, VOTE_NIL)
	}
	// now terms are same
	rf.resetElectTimer()

	// consistency check
	last := len(rf.logs) - 1
	if last < args.PrevLogIndex { // missing logs
		return
	}
	// now peer and leader have same prevIndex and same prevTerm

	// check conflict and append new logs
	committed := prevIdx
	for i, e := range args.Entries {
		cur := prevIdx + 1 + i
		if cur <= last && rf.logs[cur].Term != e.Term { // term conflict, overwrite it
			rf.logs[cur] = e
			committed = cur
		}
		if cur > last {
			rf.logs = append(rf.logs, e) // new log, just append
			committed = len(rf.logs) - 1
		}
	}

	// if leaderCommit > commitIndex, set commitIndex =min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(committed, args.LeaderCommit) // need to commit
		rf.applyCond.Broadcast() // trigger apply
	}

	rf.back2Follower(args.Term, args.LeaderID)
	reply.Succ = true
}
```



### 调用方（Leader）

```go
// leader replicate logs or send heartneat to other nodes
func (rf *Raft) sync() {
	for i := range rf.peers {
		if i == rf.me {
			rf.resetElectTimer()
			continue
		}

		go func(server int) {
			for {
				if !rf.isLeader() {
					return
				}

				rf.mu.Lock()
				rf.syncConds[server].Wait() // wait for heartbeat or Start to trigger

				// sync new log or missing logs to server
				next := rf.nextIndex[server]
				args := AppendEntriesArgs{
					Term:         rf.curTerm,
					LeaderID:     rf.me,
					Entries:      nil,
					LeaderCommit: rf.commitIndex,
				}
				if next < len(rf.logs) { // logs need sync
					args.PrevLogIndex = next - 1
					args.PrevLogTerm = rf.logs[next-1].Term
					args.Entries = append(args.Entries, rf.logs[next:]...)
				}
				rf.mu.Unlock()

				// do not depend on labrpc to call timeout(it may more bigger than heartbeat), so should be check manually
				var reply AppendEntriesReply
				respCh := make(chan struct{})
				go func() {
					rf.sendAppendEntries(server, &args, &reply)
					respCh <- struct{}{}
				}()
				select {
				case <-time.After(RPC_CALL_TIMEOUT): // After() with currency may be inefficient
					continue
				case <-respCh:
				}

				if !reply.Succ {
					if reply.Term > rf.curTerm { // higher term
						rf.back2Follower(reply.Term, VOTE_NIL)
						return
					}
					continue
				}

				// append succeed
				rf.nextIndex[server] += len(args.Entries)
				rf.matchIndex[server] = rf.nextIndex[server] - 1 // replicate succeed
			}
		}(i)
	}
}
```





## Daemon goroutines

### Apply Checker

每个节点在 Make 初始化时会启动两个后台 goroutine：
- vote goroutine：监听选举超时，开启选举
- apply goroutine：不管是 leader 还是 follower，都要循环检查自己的 `lastApplied < commitIndex`，若有差异则 执行这部分 committed 窗口日志：

```go
// apply (lastApplied, commitIndex]
func (rf *Raft) waitApply() {
	for {
		rf.mu.Lock()
		rf.applyCond.Wait() // wait for new commit log trigger

		var logs []LogEntry // un apply logs
		applied := rf.lastApplied
		committed := rf.commitIndex
		if applied < committed {
			for i := applied + 1; i <= committed; i++ {
				logs = append(logs, rf.logs[i])
			}
			rf.lastApplied = committed // update applied
		}
		rf.mu.Unlock()

		for i, l := range logs {
			msg := ApplyMsg{
				Command:      l.Command,
				CommandIndex: applied + 1 + i, // apply to state machine
				CommandValid: true,
			}
			rf.applyCh <- msg
		}
	}
}
```



### Commit Checker

在设计实现时，leader 将日志的 replicate 和 commit 解耦，所以需要 leader 在后台循环检测本轮中哪些日志已被提交：

```go
// leader daemon detect and commit log which has been replicated on majority successfully
func (rf *Raft) leaderCommit() {
	for {
		if !rf.isLeader() {
			return
		}

		rf.mu.Lock()
		majority := len(rf.peers)/2 + 1
		n := len(rf.logs)
		for i := n - 1; i > rf.commitIndex; i-- { // looking for newest commit index from tail to head
			// in current term, if replicated on majority, commit it
			replicated := 0
			if rf.logs[i].Term == rf.curTerm {
				for server := range rf.peers {
					if rf.matchIndex[server] >= i {
						replicated += 1
					}
				}
			}

			if replicated >= majority {
				// all (commitIndex, newest commitIndex] logs are committed
				// leader now apply them
				rf.applyCond.Broadcast()
				rf.commitIndex = i
				break
			}
		}
		rf.mu.Unlock()
	}
}
```





## 选举限制

参考论文 5.4.1 节，为保证选举安全，在投票环节限制：若 candidate 没有前任 leaders 已提交所有日志，就不能赢得选举。限制是通过比较 candidate 和 follower 的日志新旧实现的，Raft 对日志新旧的定义是，让两个节点比较各自的最后一条日志：

- 若任期号不同，任期号大的节点日志最新
- 若任期号相同，日志更长的节点日志最新

```go
// election restrictions
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	reply.Term = rf.curTerm
	reply.VoteGranted = false

	if args.Term < rf.curTerm {
		return // candidate expired
	}
	if args.Term > rf.curTerm {
		rf.back2Follower(args.Term, VOTE_NIL)
	}
	// now the term are same

	// check up-to-date, from Paper:
	// if the logs have last entries with different terms, then the log with the later term is more up-to-date.
	// if the logs end with the same term, then whichever log is longer is more up-to-date.
	i := len(rf.logs) - 1
	lastTerm := rf.logs[i].Term
	if lastTerm > args.LastLogTerm {
		return
	}
	if lastTerm == args.LastLogTerm && i > args.LastLogIndex {
		return
	}
	// now last index and term both matched

	if rf.votedFor == VOTE_NIL || rf.votedFor == args.CandidateID {
		reply.VoteGranted = true
		rf.back2Follower(args.Term, args.CandidateID)
	}

	return
}
```

至此，梳理了 Lab2B 日志复制的设计流程、实现了选举限制 up-to-date。



## 总结

Lab2B 应该是三个部分最难的了，我前后折腾了两三个星期，从尝试到处飞的 channel 同步换到了 sync.Cond 才更易调试和实现。值得一提的是，文件结构上的解耦也是十分有必要的，比如我的：

```
➜  raft git:(master) tree
.
├── config.go
├── persister.go
├── raft.go          # 节点初始化，超时选举机制
├── raft_entry.go    # AppendEntries RPC 逻辑
├── raft_leader.go   # sync 日志，心跳通信等
├── raft_peer.go     # 定义超时时间
├── raft_vote.go     # RequestVote RPC 逻辑
├── test_test.go
└── util.go          # 自定义的调试函数等
```

讲义参考：[lab-raft.html](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html)，为尊重课程的 Collaboration Policy，我把 GitHub repo 设为了 Private，由于经验有限，上述代码可能还有 bug，如您发现还望留言告知，感谢您的阅读。