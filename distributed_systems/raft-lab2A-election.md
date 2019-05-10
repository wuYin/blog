---
title: Lab2A. Raft 选主实现
date: 2019-04-23 09:31:52
tags: 分布式系统
---

总结下 MIT6.824 Lab2A Raft 选主的实验笔记。本文代码：[MIT6.824/raft](<https://github.com/wuYin/MIT6.824/tree/master/raft>)

<!-- more -->

## Lab2A
Raft 将一致性问题分解成三个子问题：Leader 选举、日志复制、安全性保证，分别对应 Lab 的 2A, 2B, 2C，均可参考原论文图 2 中对 Raft 实现的简要总结。本小节实验目标：
- 实现 Leader 选举：选出单个 leader 并保持领导地位，直到自己 crash 或发生网络分区
- 实现心跳通信：实现 leader 与其他节点的无日志 AppendEntries RPC 调用

## 一些坑

- RPC 调用超时

  在分布式系统中，每次调用会有三种结果：成功、失败、超时。Lab 将 net rpc 库封装成 labrpc，通过隔离节点网络来模拟节点不可用。不可用节点的 RPC 调用超时会返回 `false`，但这里不能死等 labrpc 库不确定的超时时长（100ms，2s 等都有可能），应该在调用时使用 timer 有预期地控制超时（如固定两倍心跳，200ms）
  调用超时后，不必像论文中描述的无限次重试，应简化处理，直接认为超时。

- 充分使用 sync 包来实现同步
  AppendEntries RPC 用于心跳通信和日志同步，调用时机有两个：
  - 定时心跳：leader 需在后台定期向其他节点发送 heartbeat，保持领导地位。同时在心跳还充当着日志同步的作用，当某个节点日志一致性检查失败后，会将冲突信息返回，leader 需将本地日志同步到该节点。
  - 新日志同步：当客户端发来新命令时，leader 将日志 append 到本地后即响应（lab 与论文不同），随后立刻开始新日志的同步。
  
  要调用 AppendEntries 的地方很多，因而使用 sync.Cond 条件变量而非散落各处的 channel 来进行同步触发。

- 一些时机

  - 每个节点在收到有效 RPC 调用后要重置 Election Timer，即使 Leader 无需对自己进行 rpc 调用，但重置 Timer 也是必要的。
  - 当节点收到更高 term 的 RPC 调用或响应时，要立刻回退到 follower 并重置 Timer，由于不能确信对方身份就是 Leader，所以 `voteFor` 要重置为 nil

- 关于调试
  改造 util.go 中 DPrintf() 来输出毫秒及调试信息，方便追溯系统的时序性等问题。如：

  ![image-20190509090156921](https://images.yinzige.com/2019-05-09-010157.png)

## Leader 选举
Lab 限制 leader 每秒最多发送 10 次心跳请求，实现时取心跳间隔为 100ms。相应的，选举超时时间应比心跳大一个量级左右，我实现时取 `400 + rand.Intn(4) * 100`，即 400~800ms 内的随机值，尽可能避免选举 split vote 情况。

### 选举流程

参考上一篇文章：[Leader 选举](https://github.com/wuYin/blog/blob/master/distributed_systems/raft-notes.md#leader-%E9%80%89%E4%B8%BE)

![image-20190423115736847](https://images.yinzige.com/2019-04-23-035737.png)

### 发起投票

定义 Raft 节点：

```go
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// persistent states
	curTerm  int        // latest term server has seen(initialized to 0 on first boot, increases monotonically)
	votedFor int        // candidateId that received vote in current term(or null if none)
	logs     []LogEntry // log entries; each entry contains command for state machine, and term when entry was received by leader(first index is 1)

	// implementation
	state     PeerState
	timer     *RaftTimer
	syncConds []*sync.Cond  // every Raft peer has a condition, use for trigger AppendEntries RPC
}
```



每个节点在 `Make` 初始化时都选择时长随机的 RaftTimer，之后启动新的 goroutine 监听 election timer 超时：

```go
go func() {
	for {
		select {
		case <-rf.timer.t.C: // election timeout
			rf.resetElectTimer() // this reset is necessary, reset it when timeout
			rf.vote()
		}
	}
}()
```



timer 超时后，发起投票：

```go
// start vote
// leader can start vote repeatedly, such as 2 nodes are crashed in 3 nodes cluster
// leader should reset election timeout when heartbeat to prevent this
func (rf *Raft) vote() {
	pr("Vote|Timeout|%v", rf)
	rf.curTerm++
	rf.state = Candidate
	rf.votedFor = rf.me

	args := RequestVoteArgs{
		Term:        rf.curTerm,
		CandidateID: rf.me,
	}
	replyCh := make(chan RequestVoteReply, len(rf.peers))
	var wg sync.WaitGroup
	for i := range rf.peers {
		if i == rf.me {
			rf.resetElectTimer() // other followers will reset when receive valid RPC, leader same
			continue
		}

		wg.Add(1)
		go func(server int) {
			defer wg.Done()
			var reply RequestVoteReply
			respCh := make(chan struct{})
			go func() {
				rf.sendRequestVote(server, &args, &reply)
				respCh <- struct{}{}
			}()
			select {
			case <-time.After(RPC_CALL_TIMEOUT): // 1s
				return
			case <-respCh:
				replyCh <- reply
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(replyCh) // avoid goroutine leak
	}()

	votes := 1
	majority := len(rf.peers)/2 + 1
	for reply := range replyCh {
		if reply.Term > rf.curTerm { // higher term leader
			pr("Vote|Higher Term:%d|%v", reply.Term, rf)
			rf.back2Follower(reply.Term, VOTE_NIL)
			return
		}
		if reply.VoteGranted {
			votes++
		}

		if votes >= majority { // if reach majority earlier, shouldn't wait crashed peer for timeout
			rf.state = Leader
			go rf.heartbeat()
			go rf.sync()

			pr("Vote|Win|%v", rf)
			return
		}
	}

	// split vote
	pr("Vote|Split|%v", rf)
	rf.back2Follower(rf.curTerm, VOTE_NIL)
}
```


### 响应投票
Raft 对投票节点提出了三点要求：
- 每轮能投几张：一个任期内，一个节点只能投一张票
- 是否要投：候选人的日志至少要和自己的一样新，才投票（Lab2B 实现日志的 up-to-date 比较）
- 投给谁：first-come-first-served，投给第一个符合条件的候选人

**实现**

```go
type RequestVoteArgs struct {
	Term        int
	CandidateID int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

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

	if rf.votedFor == VOTE_NIL || rf.votedFor == args.CandidateID {
		reply.VoteGranted = true
		rf.back2Follower(args.Term, args.CandidateID)
	}
}
```



## 心跳通信

Raft 将客户端的命令封装为 log entry：

```go
type LogEntry struct {
	Term    int
	Command interface{}
}
```



### 心跳请求
当候选人成功竞选为 leader 后要 **立刻** 给集群中其他节点发送心跳，避免其他节点也超时发起新一轮选举。实现方案：获得多数票后，在后台为其他的所有 peer 启动同步日志的 goroutine，等待下一轮心跳 tick，这种广播方式最好使用 sync.Cond 条件变量来实现。

获得多数票后为所有节点准备 sync

```go
// leader sync logs to followers
func (rf *Raft) sync() {
	for i := range rf.peers {
		if i == rf.me {
			rf.resetElectTimer()
			continue
		}

		go func(server int) {
			for {
				rf.mu.Lock()
				rf.syncConds[server].Wait() // wait for trigger

				args := AppendEntriesArgs{
					Term:         rf.curTerm,
					LeaderID:     rf.me,
					PrevLogIndex: 0,
					PrevLogTerm:  0,
					Entries:      nil, // heartbeat entries are empty
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
				case <-respCh: // response succ
				}

				if reply.Term > rf.curTerm { // higher term
					rf.back2Follower(reply.Term, VOTE_NIL)
					return
				}
			}
		}(i)
	}
}
```

同时开启心跳 tick，准备广播通知 sync

```go
// send heartbeat
func (rf *Raft) heartbeat() {
	ch := time.Tick(HEARTBEAT_INTERVAL)
	for {
		if !rf.isLeader() {
			return
		}

		for i := range rf.peers {
			if i == rf.me {
				rf.resetElectTimer() // leader reset timer voluntary, so it won't elect again
				continue
			}

			rf.syncConds[i].Broadcast()
		}
		<-ch
	}
}
```



### 响应心跳

对于心跳请求，节点暂时只需对比任期号，若 term 未过期则调用成功。2B 部分将实现日志的一致性检查：

```go
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	reply.Term = rf.curTerm
	reply.Succ = false

	if rf.curTerm > args.Term {
		return // leader expired
	}

	rf.back2Follower(args.Term, VOTE_NIL)
	reply.Succ = true
}
```



**2A 测试通过：**

 <img src="https://images.yinzige.com/2019-05-10-035030.png" width=60% />

## 总结

整个实验可从 test 着手，实验环境是 3 个节点组成的集群，实现时需在 leader crash 后及时选出下一任 leader，且处理好旧 leader re-join 等情况。
个人经验：对于分布式系统，调试时可在请求参数、响应结构中加入 debug 信息，用于追踪某次请求的处理过程和结果，梳理清楚了执行流程，再去针对性的解决问题。
为了让代码更清爽，我把 raft.go 的代码按功能拆分为了 2 部分：vote 处理投票请求，enry 处理心跳请求。之后的两个实验小节将对应修改这两个文件。
 <img src="https://images.yinzige.com/2019-04-23-044741.png" width=40% />