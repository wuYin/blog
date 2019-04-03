---
title: MapReduce 论文和实验笔记
date: 2019-04-02 21:12:50
tags: 分布式系统
---

总结下 6.824 MapReduce lab 的论文笔记和实验过程。本文代码：[MIT6.824/mapreduce](https://github.com/wuYin/MIT6.824/tree/master/mapreduce)

<!-- more -->

## 前言

自己的 nsx PRC 框架 v0.2 需支持分布式环境下服务变更的通知，对 zookeeper 不想只停留在会用的层面，于是学习 [MIT 6.824 Distributed Systems](https://pdos.csail.mit.edu/6.824/schedule.html)，本文是 Lec1: MapReduce 的学习笔记。



## 论文阅读

### 问题来源

在 2004 年以前，Google 团队为处理各种原始数据实现了上百个专用计算程序，比如对原始网页文档生成倒排索引，数据量少时单机处理就行，但数据量过大后单机处理就太耗时了，只能将数据分布在多个主机上并行处理，最后聚合各节点生成的索引数据。

分布式计算降低了耗时，但也必须解决一些问题：如何分发数据？多节点如何保证负载均衡？如何处理节点失效？… 多节点调度工作并不简单。类似的大数据处理场景在谷歌内部还有很多。于是 Jeff 团队就将类似场景的处理流程抽象出来，在 2004 年推出了分布式计算模型 MapReduce，用户只需自定义的 2 个数据的处理函数：

- 如何分割原始数据：Map Func
- 如何聚合中间数据：Reduce Func

之后就能使用 MR 模型通过加节点来提高计算效率，关于节点容错、数据分发、负载均衡的问题 MR 都已处理。



### MR 应用实例

举个例子：对文本文件中的单词计数，论文中 MR 内部处理的伪代码如下：

```go
// MR 处理的数据是 Key-Value 结构的
// key 是文件名，value 是整个文件内容，对空格隔开的每个单词进行计数 "1" 操作
map(String key, String value):
	// key: document name
	// value: document contents
	for each word w in value:
		EmitIntermediate(w, "1");

// 对每个单词 key 都进行词频累加
reduce(String key, Iterator values):
	// key: a word
	// values: a list of counts
	int result = 0;
	for each v in values:
		result += ParseInt(v);
	Emit(AsString(result));
```

MR 内部隐藏了 Map 操作后将计数结果写入中间文件，Reduce 操作从中间文件读取计数信息的细节。只需用户自己实现 Map/Reduce 的逻辑，即可将任务分布式并行化执行来大幅提升效率。



### MR 数据结构

MR 面向的输入输出数据都是 Key-Value 结构，其中 k-v 约定都是 string 类型，值可能是整个原始文件的内容，也可能是数字，取决于用户自定义的 map func 和 reduce func，这 2 个函数的关联类型是固定的：

```
map    (k1,v1)       → list(k2,v2)
reduce (k2,list(v2)) → list(v2)
```

- map 处理 raw data 对每个内容点 k 都生成 k-v pair
- reduce 对每个 k 都聚合其 list 中间数据，最终生成聚合结果

  

### MR 执行流程

首先说明 MapReduce 是一种分布式计算模型，不是某个开源的分布式调度框架，所以在不同场景下对模型的实现代码并不相同。比如对文本文件中的单词进行计数，可使用 MR 模型来实现分布式运行，系统运行流程如下：

![](https://images.yinzige.com/2019-04-03-042855.png)

- Split： MR 将 input files 分割为 M 个子数据片段
- Fork：将用户程序 fork 后运行在多个节点上，整个运行过程会执行 M 个 map task 和 R 个 reduce task，节点由一个 master 和多个 worker 组成，其中 master 负责调度空闲的 worker 来运行 task
- Map：
  - 被分配到 map task 的 worker 先读取子数据片段，再调用 Map func 来处理原始数据生成 k-v pairs 中间数据，并通过分区函数归类到 R 个子文件，随后写入本地磁盘。
  - map worker 将中间文件的存储地址通知 master，随后 master 将 R 个中间文件分配给 reduce worker 处理
- Reduce：
  - 被分配到 reduce task 的 worker 使用 RPC 读取 map worker 上 master 给定的中间文件。虽然同一个 key 会被分区到同一个中间文件，但 key 与 key 之间的写入顺序是无序的，所以读取完毕后需对 keys 统一进行排序，否则输出到 output file 的结果是无序的，会导致 master merge 的结果也是无序的。
  - 排序完毕后对每个 key 都调用 Reduce func 来进行聚合，并将结果输出到对应分区的 output file 中
- Merge：master 等所有的 map task 和 reduce task 都执行完毕后，将 R 个 output files 进行 Merge 操作，整个分布式计算过程执行结束。



### 处理容错

#### worker 失效

master 会定期向各个 worker 发送 ping 心跳包，若在超时时间内收到 pong 包则认为 worker 有效，否则标记为失效不可用。MR 会将原来分配到失效 worker 的 task 回收重新分配到其他可用的 worker 上重新执行。值得区分的是：

- map worker 失效后是必须重新运行 map task，因为 worker 崩溃了无法处理本地中间文件的访问请求
- reduce worker 如果失效但已生成聚合文件，通知给了 master 该文件在 GFS 中的位置，就不必重新运行

相比论文中如上第 2 种 worker 容错机制，实际在 lab 中都是出错超时直接将 task 分配给其他 worker 运行，因为 lab 并没有实现 reduce worker 输出结果到 output file 后通知 master 的机制。

#### master 失效

这种情形论文中只给出了简单的处理方案，即定期将 master 的所有状态作为快照 checkpoint 持久化到磁盘，当 master 崩溃后从最近的 checkpoint 启动新的 master 继续处理。

因为 MR 要求 map func/reduce func 都必须是功能函数，不保留任何状态，即相同的输入能得到相同的输出。所以 master 恢复后继续调度运行是可行的。

#### GFS

论文中的容错机制充分利用了 GFS 分布式文件系统的文件原子特性，可直接看原论文是怎么用的。



### MR 实用技巧

#### 分区函数

在 Map 阶段，使用 `hash(key) mod R` 来保证每个 key 都能汇总到同一中间文件，保证所有 key 尽可能地均匀分布在 R 个中间文件中。

#### 保证顺序

在 Reduce 阶段从中间文件中读取数据时得先排序再聚合，这样聚合到 output files 之间就是分段有序的。





## 实验笔记

### Part1. 处理 MR 的输入输出

注意 map task 的输出要能被 reduce task 读取，所以要约定好 encode/decode 结构。lab 注释建议每行存储一个 JSON Encode 后的  k-v，自己做的时候可以 `[]k-v` 直接 Marshal，在 reduce 读取时对应反序列化。

### Part2. 单机版 word count

对照如上 MR 实例流程图实现。

### Part3. 分布式版 MR

lab 中使用 RPC 在本地模拟分布式多节点的情况，有新 worker 注册后会通知 registerChan，所以在 schedule 调度时候可 select 从 channel 接收新 worker，或者复用旧的空闲 worker 处理 task

### Part4. 实现 worker 容错

lab 没有完全按照 paper 来，map/reduce worker 崩溃了都是直接分配给其他可用的空闲 worker 进行 re-execute，需注意多个 schedule goroutine 之间等待可用 worker 时可能出现竞态条件，自己尝试了几个方案后总结了一些经验：

- 不要通过共享内存来进行通信，而是通过通信来共享内存

  lab 代码已有的 registerChan 是无缓冲 channel，如果复用它来在多个 schedule 间共享空闲 worker，那 map 任务结束后再向它发送 worker 会直接阻塞，此时使用缓冲 channel 合适。反之如果将 worker 的状态变更放到内存中共享使用，多个 schedule goroutine 共享和更新 worker，可能产生很多竞态条件。

- 锁使用粒度要小，要集中，不要写多个 goroutine 可能会产生竞态的代码

  如果跑测试有时候通过，有时候在 lock 周围 panic，那可能代码中还隐藏有竞态条件，而且不好复现调试。总之不要滥用 channel 和 sync.Mutex，梳理好多个 goroutine 之间数据传递方式后再写代码也不迟。

### Part5. 使用 MR 生成倒排索引

注意给每个单词打分，将分数高的单词排在前边即可通过测试。



## 总结

MR 要求用户先把任务拆分成 Map / Reduce 2 种子任务，MR 并发地执行 Map 任务产生中间数据，再并发地执行 Reduce 任务聚合数据，最终 Merge 后输出结果，在处理海量数据时通过直接加 worker 就能提高系统性能，水平扩展能力很高。

MR 是一种开创性的分布式计算模型，能通过拆分逻辑实现任务的分布式运行，比较通用化。现如今，虽然有的分布式场景下 MR 模型不是最佳解决方案，但对于设计和学习分布式系统依然很有价值。

感谢 Jeffrey 团队





















