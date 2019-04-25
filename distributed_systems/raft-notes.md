---
title: Raft 笔记
date: 2019-04-19 09:02:21
tags: 分布式系统
---

Raft 算法的学习笔记和个人理解。

<!-- more -->

## 前言

Raft 论文对算法本身做了详尽的讲解，同时细节也很多，如果不梳理清楚就去做 6.284 Lab2，越往后做就会越发现很多细节都没考虑清楚，也就没法应对各种非正常网络状况。本文记录对 Raft 的学习笔记和流程梳理。


## Raft 算法概览

### 背景

对于分布式存储系统，必须要解决的主要问题：当系统网络发生故障，或某些节点宕机，集群整体也要能对外可用，即保证容错性。通常通过复制副本来保证容错，即某个节点因宕机或网络隔离不可用后，可让另一个一直在复制它数据的节点来接替它的工作，继续处理请求。

保证多个节点上复制的数据相同就是一致性算法要解决的问题。1990 年至今，[Paxos 一致性算法](https://zh.wikipedia.org/zh-hans/Paxos%E7%AE%97%E6%B3%95) 依旧是学术界最知名的一致性算法，但其晦涩难懂且在工程应用时需进行大幅修改才能应用。于是 2013 年衍生出了更易理解的 [Raft 一致性算法](https://zh.wikipedia.org/zh/Raft)，其功能和性能与 Paxos 相当。

### 可理解性
Raft 的易理解性源于自身的两个创新性设计：
- 分解一致性问题为三个子问题：leader 选举、日志复制、安全性保证
- 通过增强一致性状态来减少要考虑的状态数：如 Raft 通过额外选举机制保证新 leader 一定包含旧 leaders 已经 commited 的日志，让算法更好理解。



## Leader 选举

### 状态变更

在 Raft 中，每个节点只会有三种状态：leader、candidate、follower，转变时机如下：

<img src="https://images.yinzige.com/2019-04-10-095432.jpg" width=60% />

变更时机：
- 集群初始化时节点均为 follower，它们的 election timeout 是从固定区间如 150-300ms 取的随机值
- 第一个选举超时的 follower 发现一直未收到 leader 或 candidate 的 RPC（可能是 leader 宕机了，也可能是 leader 和当前 follower 发生了网络隔离）它就认为当前集群中并没有 leader，于是升级为 candidate，向其他 followers 发起投票请求，开始新一轮选举：
  - 选举期间，若收到本轮的 leader 请求，或发现了更新一轮的节点，则自降为 follower
  - 若本轮选举超时了还未收到大多数票，也没收到请求，则重启新一轮选举
  - 若收到来自大多数 follower 的选票，则升级为 leader
- leader 如果发现有更高任期的节点，则自降为 follower

Raft 限制了只有收到大多数节点的投票，才能升级为 leader，从而保证了每个集群每个任期内只会有一个 leader，这也是节点个数最好设为奇数的原因。
成为 leader 后它会向其他节点发送心跳请求表明自己的领导地位。心跳请求的间隔时间需比选举超时时间小一个量级左右。比如每隔 50ms 并行发送一次心跳请求，对应选举超时时间设为 400ms 左右，才能让 leader 一直保持自己的领导地位。



### 选举流程

#### 从 follower 到 leader 的选举步骤：

1. 选举超时时间到，切换为 candidate 身份
2. currentTerm 自增 1
3. 给自己投一票
4. 并行向其他节点发起 RequestVote RPC，候选人等待响应时可能发生 3 种情况：
- 成功收到大多数节点的选票：升级为 leader
- 收到本轮已选出 leader 的请求，则主动放弃竞选降为 follower。如下图的 B 节点：

 <img src="https://images.yinzige.com/2019-04-19-083231.png" width=70% />

- 本轮选举超时了还没有收到大多数票，也没收到其他请求，就继续保持 candidate 身份，开启下一轮选举。
如下是平票（split vote）情况，有 A,B,C,D 四个节点的集群，若 A 和 C 近乎同时选举超时。B 给最近的 A 投了一票，D 给最近的 C 投了一票。于是 A 和 D 两个候选人都没有达成多数票，二者都会重新开启 Term 2 的新选举。 此时 Raft 让 A 和 C **随机**选择选举超时时间，所以 A 和 C 在 Term2 不会同时超时，能成功选出 leader

<img src="https://images.yinzige.com/2019-04-19-091229.png" width=80% />

#### Raft 对投票者提出了三点要求

- 每轮能投几张：一个任期内，一个节点只能投一张票
- 是否要投：candidate 的日志至少要和自己的一样新（下节详述），才投票
- 投给谁：first-come-first-served，投给第一个符合条件的 candidate

#### 总体流程

![](https://images.yinzige.com/2019-04-23-035725.png)


## 日志复制

成功选出 leader 后，集群即对外可用。Raft 让客户端的请求统一给 leader 处理，若 follower 收到请求会直接转发给 leader 处理。而 leader 的工作是保证各个 follower 上的日志顺序、内容都是一致的。

leader 将每个客户的请求都封装成一条日志 log entry，随后将这些日志 entries replicate 到其他 followers 节点，它们随后以相同的顺序、相同的内容执行命令，从而让 followers 数据一致。

### 请求处理流程

leader 将客户端请求中的命令（如 `SET x 3`）封装成一条 log entry：
1. leader 将该条日志 append 到本地
2. leader 并行地向其他节点发起 AppendEntries RPC 调用
3. leader 收到大多数节点 RPC 调用成功的响应
4. leader 在本地状态机上 apply 该条日志
5. leader 响应客户端请求
6. leader 通知 followers 可以安全地 apply 该条日志

在第 3 步中，只要大多数节点响应说都成功 append 了日志，leader 就认为在自己的状态机上 apply 日志是安全的，对于那些未响应的节点 leader 会无限期地重试 AppendEntries RPC 调用。因此 Raft 并不是类似 2PC 协议的强一致性，而是保证最终一致性。

### Log Entry 存储结构

每个节点的每条日志都会包含：请求 append 该日志的 leader 任期号、要执行的命令。下图展示了 5 个节点的集群可能的状态：leader 看到索引 1-7 的日志至少在其他两个节点上复制成功，就认为该日志是 **commited** 状态，而最后一条 `x <- 4` 的日志并未复制到多数节点，所以索引为 8 的那条日志是 **uncommitted** 状态

 <img src="https://images.yinzige.com/2019-04-10-145221.jpg" width=65% />

注意区分 log 的状态：

- commit / commited：log 成功复制到大多数节点后的状态，还未执行不会影响节点值
- apply / applied：log 成功被状态机执行后的状态，会真正影响节点值
- uncommitted：leader 持有的新日志，但未成功复制到大多数节点上


## 安全性保证

在分布式系统中，网络永远不可靠，而节点一般都用高性价比的普通主机，磁盘异常导致宕机更是常事。Raft 在网络隔离、节点不可用等环境下仍能保证节点数据的一致性，得利于它的一个原则四个特性：

### 1. Leader Append-Only 原则

leader 对自己的日志不能覆盖和删除，只能进行 append 新日志的操作。



### 2. Election Safety 特性

每个任期内最多只能选出一个 leader，试想如果集群同一任期选出了多个 leader，即发生了 brain split（脑裂），会直接导致同一时刻集群中多个 follower 之间数据不一致。在 Raft 中用以下限制来保证 election safety

- 一个节点每个任期内只能投一张票
- 获得多数票（过半）的节点才有资格升级为 leader



### 3. Log Matching 特性

若两个节点的日志中，同一索引的两条日志，任期号也相同。则两个节点在该索引前的日志都是相同的。Raft 通过额外机制保证日志匹配：

- append-only 保证：leader 在每个 term 的每个 index 只会存储一条 log entry，append 成功后不再修改
- 一致性检查：leader 在 AppendEntries RPC 调用时会将上一条日志的索引和任期号 `prevLogIndex`, `prevLogTerm` 一并发送，即告诉 follower 接收新日志之前检查一下上一条日志是否和自己一致。
  - 上一条日志匹配成功：则 follower 将新日志 append 到本地
  - 未匹配成功：follower 告知 leader 日志不一致性

集群初始化时，所有节点都是空日志，自然满足 log matching，之后的一致性检查保证了新增的日志也满足 log matching，一步步地累加日志，才能满足上边一条匹配，则前边所有日志也匹配的特性。

正常 leader 不宕机的情况下，leader 和 followers 的日志会一直遵从 log matching，但 leader 也会出现宕机的情况，可能出现 leader 还没来得及把新日志全部复制给 followers 的情况：

 <img src="https://images.yinzige.com/2019-04-11-021842.jpg" width=60% />

如上盒子是一条日志，编号是任期号。因为 leader 和 follower 都可能会宕机，也就可能出现如上的日志不一致的情况：

- a，b：follower 可能丢失部分日志
- c，d：follower 本地可能 uncommited 的日志
- e，f：follower 可能既缺少本该有的日志，也多出额外的日志

那 leader 如何处理日志不一致的情况呢？

1. leader 强制让日志不一致的 follower 重写自己的日志，和 leader 保持一致
2. leader 维护 `nextIndex[]` 数组，记录要发给每个 follower 的下一条日志索引。用于：

![image-20190422094632883](https://images.yinzige.com/2019-04-22-014633.png)

如上的一致性检查操作能让 follower 的日志和 leader 强制保持一致。



### 4. Leader Completeness 特性

若某条日志在前任 leaders 中已被提交（commited），则这条日志也一定会出现在更大任期的 leader 日志中。此特性由以下限制实现：

- commited 状态：某条日志只有在成功复制给大多数节点后才是 commited 
- leader 选举：只有获得多数票的候选人才能成为 leader，而 voter 给候选人投票的前提是，候选人的日志至少要和 voter 的一样新：

Raft 通过比较两个节点**最后一条日志**的索引、任期号来比较新旧：
- 先比任期：任期不同，则任期大的更新
- 再比索引：任期相同，则索引大（更长）的日志更新

如上两个 commited 大多数和 election 大多数一定会有重叠：
![image-20190422101422311](https://images.yinzige.com/2019-04-22-021422.png)

即 leader 节点至少收到了一个包含最新 commited 日志的节点的投票。足以说明 leader 包含最新的 commited 日志。



### 5. State Mechine Safety 特性

考虑有 5 个节点的集群情况：

 <img src="https://images.yinzige.com/2019-04-11-101454.jpg" width=70% />

- a：**S1** 当选 term2 的 leader，将日志成功复制到 S2，**S1 crash**
- b：**S5** 当选 term3 的 leader，只接收了一条新日志，**S5 crash**
- c：**S1** 重新当选 term4 的 leader，将自己在 term2 的日志复制到了 **S3** 上，该条日志成功复制到了大多数节点，为 commited 状态，S1、S2、S3 的状态机均可安全地 apply

不幸的是：**S1 crash again！**，于是出现 d：**S5** 重新当选 term5 的 leader，将自己在 term3 的日志复制到了全部节点上。导致 c 中可能已被 applied 的日志被回滚。

回滚的根本原因：**S1** 在 term4 中提交了自己在 term2 的旧日志。为避免日志被回滚，<u>Raft 不允许 leader 提交之前任期的日志</u>，而是在提交当前任期的新日志时候，根据 log matching 特性，**顺带** 将旧日志一并提交。此外，Raft 要求 leader 当选后立即尝试提交一条 no-op（无操作）的空日志，在一致性检查成功后及时将已有的日志提交。

如上对 leader 提交时机的约束，集群将不会出现情形 c，而是 e：**S1** 只提交 term4 的新日志，顺带提交 term2 的旧日志。当新日志复制成功后哪怕 **S1** 再次 crash，**S5** 也不会当选（S5 最新日志任期为3，小于 **S2, S3** 的 4）



## 特殊 Case

### 网络分区

如果集群内部发生网络分区，如下图举例：

- A, B 两个节点在上海
- C, D, E 三个节点在北京

B 节点是原 leader，假定两地线路故障，造成集群内部的网络分区。此时北京的三个节点选出了新 leader E。虽然集群中同时存在两个 leader，但二者的 term 却不同。

 <img src="https://images.yinzige.com/2019-04-22-022355.png" width=80% />

现在考虑集群对外的读写

- 写：上海的客户端优先选择节点 B 进行写操作，但 leader B 无法将日志复制到大多数节点，该日志是 uncommitted 状态，不会响应客户端说写入成功，而响应写入超时。

- 读：若网络分区一直未恢复，则可能存在某个客户端在节点  E  上写入新数据，但在节点 B 上读到的却是旧数据。为避免在网络分区阶段读到旧数据，可有如下两种解决方案（原论文 S8）：
  - 每次处理读请求时候，都必须和大多数节点进行通信，检查自己的 leader 地位，因此能保证读取最新数据，但高频通信有效率问题。
  - 使用租约机制实现心跳，若大多数节点的租约都未到期则读到的数据仍旧是最新的，但租约机制依赖时序性。

假设现网络分区恢复，节点 B 会发现有更高 term 的节点存在，就撤销自己 uncommitted 的日志，并和 leader E 进行日志同步，由此保证日志一致性。



### Leader Crash

参考：[Raft 为什么是更易理解的分布式一致性算法](https://www.cnblogs.com/mindwind/p/5231986.html)



## 总结
Raft 将集群中节点的状态分为 3 类：leader（领导）、follower（民众）、candidate（候选人），并为系统增加了许多限制来实现四个特性，从而保证多节点的数据一致性，实现节点容错。
- Leader Append-Only 原则
  日志只能从 leader 流向其他节点，leader 对日志只能 append，不覆盖也不删除。
- Election Safety 特性：
  - 一个节点每个任期只能投一张票，投票标准：候选人的日志至少要和自己一样新
  - 获得多数票（过半）的候选人才能当选 leader
- Log Matching 特性
  AppendEntries RPC 调用时检查日志一致性，leader 维护 `nextIndex[]` 并循环检查后强制同步日志。
- Leader Completeness 特性：两个多数性原则会重叠，保证选出的 leader 包含集群的所有 commited 日志
- State Mechine Safety 特性：leader 不直接提交旧日志，而是 log matching 前提下提交新日志，校验一致后顺带提交。

通读论文会发现细节多且零散，待 lab 做完再补充。

## 参考
[raft.github.io](<https://raft.github.io/>)
[thesecretlivesofdata.com/raft](http://thesecretlivesofdata.com/raft)
[一文搞懂Raft算法](https://www.cnblogs.com/xybaby/p/10124083.html)

[Youtube: Raft lecture (Raft user study)](<https://www.youtube.com/watch?v=YbZ3zDzDnrw>) & 笔记：

![](https://images.yinzige.com/raft-notes.png)