---
title: Lab2A. Raft 选主实现
date: 2019-04-23 09:31:52
tags: 分布式系统
---

总结下 MIT6.824 Lab2A Raft 选主的实验笔记。本文代码：[MIT6.824/raft](<https://github.com/wuYin/MIT6.824/tree/master/raft>)

<!-- more -->

## Lab2A
Raft 将一致性问题分解成三个子问题：Leader 选举、日志复制、安全性保证，分别对应 Lab 的 2A, 2B, 2C，均可参考原论文图 2 中对 Raft 实现的简要总结。本小节实验目标：
- 实现 Leader 选举：选出单个 leader 并保持领导地位，直到自己 crash
- 实现心跳通信：实现 leader 与其他节点的无日志 AppendEntries RPC 调用

## Leader 选举
Lab 限制 leader 每秒最多发送 10 次心跳请求，实现时取心跳间隔为 100ms。相应的，选举超时时间应比心跳大一个量级左右，我实现时取 `400 + rand.Intn(4) * 100`，即 400~800ms 内的随机值，尽可能避免选举 split vote 情况。

### 选举流程

参考上一篇文章：[Leader 选举](https://github.com/wuYin/blog/blob/master/distributed_systems/raft-notes.md#leader-%E9%80%89%E4%B8%BE)

![image-20190423115736847](https://images.yinzige.com/2019-04-23-035737.png)

### 发起投票

定义 Raft 节点：

```go
type Raft struct {
	mu        sync.Mutex          // 共享锁
	peers     []*labrpc.ClientEnd // 集群中的全部节点
	persister *Persister          // 持久化工具
	me        int                 // 本节点在 peers 中的索引

	curTerm  int           // 节点目前的任期号
	votedFor int           // 节点目前的投票对象
	entries  []LogEntry    // 本地日志
	state    PeerState     // 节点状态
	timer    *RaftTimer    // 选举超时定时器
	entryCh  chan LogEntry // 日志处理 channel
}
```

每个节点在 `Make` 初始化时都选择时长随机的 RaftTimer，之后启动新的 goroutine 监听 timer 超时和 entryCh 心跳请求，当 RaftTimer 超时后，变身为候选人发起投票。

** 代码实现：**
```go
// 投票参数
type RequestVoteArgs struct {
	Term        int // 候选人的任期号
	CandidateId int // 候选人 id
}

// 响应投票
type RequestVoteReply struct {
	Term        int  // 选民节点的任期号
	VoteGranted bool // 是否赢得该选票
}

// 候选人发起投票
func (rf *Raft) vote() {
	rf.curTerm++
	rf.state = Candidate
	rf.votedFor = rf.me

	args := RequestVoteArgs{
		Term:        rf.curTerm,
		CandidateId: rf.me,
	}
	replyCh := make(chan RequestVoteReply, len(rf.peers))
	var wg sync.WaitGroup
	for i := range rf.peers {
		if i == rf.me {
			continue
		}

		wg.Add(1)
		go func(server int) {
			defer wg.Done()
			var reply RequestVoteReply
			if succ := rf.sendRequestVote(server, &args, &reply); !succ {
				return
			}
			replyCh <- reply
		}(i)
	}
	go func() {
		wg.Wait()
		close(replyCh) // 避免资源泄漏
	}()

	votes := 1
	targetVotes := len(rf.peers)/2 + 1
	for reply := range replyCh {
		// 已有更新 leader，回退到 follower
		if reply.Term > rf.curTerm {
			rf.back2Follower(reply.Term)
			return
		}
		if reply.VoteGranted {
			votes++
		}

		// 如果选票已过半，不再等待已 crash 的节点调用超时
		if votes >= targetVotes {
			break
		}
	}

	// 因 split vote 等原因未达到多数票
	if votes < len(rf.peers)/2+1 {
		rf.resetElectTimer()
		return
	}

	// 成功当选，立刻发送心跳
	rf.state = Leader
	go rf.heartbeat()
}
```

注意减少选举耗时：候选人收集选票过程中，实时计票过半后即可结束选举，而非等待所有请求都返回了才去计票。假设有的节点已 crash，那 RPC 调用将超时返回 false，超时时间为 100ms，若不立即结束选举，候选人将白白浪费 100ms 时间，也就无法及时选出 leader



### 响应投票
Raft 对投票节点提出了三点要求：
- 每轮能投几张：一个任期内，一个节点只能投一张票
- 是否要投：候选人的日志至少要和自己的一样新，才投票
- 投给谁：first-come-first-served，投给第一个符合条件的候选人

**代码实现（干净整洁的代码）：**
```go
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	reply.Voter = rf.me
	reply.Term = rf.curTerm

	switch {
	case args.Term < rf.curTerm: // 拒绝处理
		reply.VoteGranted = false
		return
	case args.Term == rf.curTerm: // 每个任期只能投一票
		if rf.votedFor == VOTE_NIL || rf.votedFor == args.CandidateId {
			reply.VoteGranted = true
			rf.votedFor = args.CandidateId
			rf.back2Follower(args.Term)
		}
	case args.Term > rf.curTerm: // 直接投票
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.back2Follower(args.Term)
	}

	return
}
```

比较候选人与自己的日志将在 2B 中实现。



## 心跳通信

Raft 将客户端的命令封装为 log entry：

```go
type LogEntry struct {
	Index   int         // 日志索引号
	Term    int         // 写入日志时节点的任期号
	Command interface{} // 客户端命令
}
```



### 心跳请求
当候选人成功竞选为 leader 后要 **立刻** 给集群中其他节点发送心跳，避免有的节点也超时发起新一轮选举。

**代码实现：**
```go
// 心跳请求
type AppendEntriesArgs struct {
	Term         int        // leader 任期号
	LeaderId     int        // leader id
	PrevLogIndex int        // 暂时不用
	PrevLogTerm  int        //
	Entries      []LogEntry // 批量日志，心跳时为空
}

// 心跳响应
type AppendEntriesReply struct {
	Term int  // 节点任期号
	Succ bool // 心跳是否成功响应
}

// leader 发送心跳
func (rf *Raft) heartbeat() {
	t := time.NewTicker(HEARTBEAT_INTERVAL) // 100ms
	for {
		if !rf.isLeader() {
			return
		}

		args := AppendEntriesArgs{
			Term:         rf.curTerm,
			LeaderId:     rf.me,
			PrevLogIndex: 0,
			PrevLogTerm:  0,
			Entries:      nil, // 心跳时为空日志
		}
		replyCh := make(chan AppendEntriesReply, len(rf.peers))
		var wg sync.WaitGroup
		for i := range rf.peers {
			if i == rf.me {
				continue
			}
			wg.Add(1)

			go func(server int) {
				defer wg.Done()
				var reply AppendEntriesReply
				if succ := rf.sendAppendEntries(server, &args, &reply); !succ {
					return
				}
				replyCh <- reply
			}(i)
		}
		wg.Wait()
		close(replyCh)

		var lived int
		for reply := range replyCh {
			if reply.Term > rf.curTerm {
				// 发现新 leader，如网络分区恢复
				rf.back2Follower(reply.Term)
				return
			}
			lived++
		}

		// 未收到来自大多数节点的心跳，重新开始选举
		if lived < len(rf.peers)/2+1 {
			rf.vote() // 重新开始投票
			return
		}

		<-t.C
	}
}
```



### 响应心跳

对于心跳请求，节点需对比任期号，并进行日志的一致性检查：

```go
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	if len(args.Entries) > 0 {
		log.Fatal("invalid entry in 2A")
	}

	reply.Term = rf.curTerm
	if rf.curTerm > args.Term {
		reply.Succ = false
		return
	}

	// 检查双方日志的一致性
	if i := len(rf.entries) - 1; i >= 0 {
		switch {
		case i < args.PrevLogIndex: // 本地少日志，让 leader nextIndex[i]-- 后再同步
			reply.Succ = false
			return
		case i == args.PrevLogIndex:
			if rf.entries[i].Term != args.PrevLogTerm { // term 不匹配
				reply.Succ = false
				return
			}
		case i > args.PrevLogIndex: // 强制删除
			rf.entries = rf.entries[args.PrevLogIndex:]
		}
	}
	rf.entries = append(rf.entries, args.Entries...)
	rf.entryCh <- LogEntry{Term: args.Term}

	reply.Succ = true
	return
}
```



## 总结

整个实验可从 test 着手，实验环境是 3 个节点组成的集群，实现时需在 leader crash 后及时选出下一任 leader，且处理好旧 leader re-join 等情况。
个人经验：对于分布式系统，调试时可在请求参数、响应结构中加入 debug 信息，用于追踪某次请求的处理过程和结果，梳理清楚了执行流程，再去针对性的解决问题。
为了让代码更清爽，我把 raft.go 的代码按功能拆分为了 2 部分：vote 处理投票请求，enry 处理心跳请求。之后的两个实验小节将对应修改这两个文件。
 <img src="https://images.yinzige.com/2019-04-23-044741.png" width=40% />








































