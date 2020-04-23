---
title: TGIP-CN 演讲笔记
date: 2020-04-16 14:37:11
tags: pulsar
---

「持续更新」[TGIP-CN](https://github.com/streamnative/tgip-cn) 官方关于 Pulsar 演讲的笔记。

<!-- more -->

## 1. Pulsar Basics

### 1.1 Event streaming

- Connect：Pub/Sub、Connector（第三方数据源接入如 ES），protocol handler（broker 自定义协议如 KOP）

- Store：bookeeper 层级数据存储，而非 broker

- Process：编写 functions 进行轻量计算、SQL 查询

### 1.2 Pub/Sub

Producer

注：发送的消息需满足 schema 定义

Consumer

四种订阅模式（抽象为插件式配置）

- Exclusive Sub：topic 仅由一个 consumer 独享消费
- Failover Sub：多个 consumer 订阅多个 partition，一对一或一对多消费，即 kafka consumer group
- Shared Sub：多个 consumer 订阅多个 partition，多对多 broker 轮询分发消费，即传统 MQ，横向扩展 consumer 来提高吞吐
- Key_Shared Sub：按 Key hash 分发消费 && Shared Sub，即有序性 && 扩展性

### 1.3 Storage

分层：`InfiniteEventStream -> Segments -> Entries -> Batch Messages -> Message`

Segment 存储：BookKeeper（分布式的 append-only 日志系统），分片生成策略可配置，存储系统接口：

- 可 append
- 可根据 offset 读取 message

Message

- Event Time: 生产端可自定义的消息时间

- SchemaVersion：消息对应的 schema 版本号

- MessageID（bytes）：即 offset = 四元组（ledger-id, entry-id, batch-index, partition-index)

  - ledger-id 为全局唯一 `id -> message id` 全局唯一
  - batch-index 为 -1，单独发送的消息
  - partition-index 为 -1，topic 未被分区的消息

  ​	![image-20200414121858169](https://images.yinzige.com/2020-04-14-041858.png)

- Broker：负责每个 subscription 的 ACK move 并存储 cursor（subscription，messageID） 中
  - cursor 的变更会记录到 log，log 过多会进行 compaction，存储在 ZK
  - DESIGN：subscription 的消费状态 ==> cursor ==> log，log 记录消费状态，故能 Reset
  
- Reader：自己管理消费状态，消费状态不被持久化，即无 cursor 的消费者，如 exactly-once 消费
  - 读取消息需自己存储并提供 messageID，如绑定 flink 的 checkpoint
  - broker 也在内存记录 cursor
  - 根据 cursor 可直接取差值得 LAG，Reader 的概念在 kafka 中 LAG 需在 consumer 端自己计算，如 flink
  
- 多租户：三层层级化
  - `domain(persistent) ://  tenant / namespace / topic`
  - domain 分为 non-persistent 不保存消息、默认 persistent（简写 p）
  - 如 topicA => 对应的 Fully Qualified Name 为 `p://public/default/topicA`



---



## 2. Data Lifecycle

### 2.1  Data Flow

#### 2.1.1  计算与存储分离

- broker：处理生产消费请求，stateless 无状态
- bookie：作为 bookkeeper 节点持久化消息

元数据存储：zookeeper（插件接口可替换为 etcd）

 ![image-20200414135154659](https://images.yinzige.com/2020-04-14-055155.png)



#### 2.1.2  生产流程

- Producer Route Partition：选定要写的分区
  - producer 默认 round robin 选择，batch interval 内复用同一 partition
  - 若提供 KEY 则 hash
  - 可自定义 router 规则
- **Producer Topic Discovery**：首次向任意 broker LookUP 目标分区所在的 broker（LB 层），发起长连接（复杂设计如连接池）

- Broker Write Bookie：并发调用 BK 客户端写数据，大多数 ACK 则认为写成功

 ![image-20200414140907444](https://images.yinzige.com/2020-04-14-060908.png)





#### 2.1.3  消费流程

- Topic Discovery：按消费模式找出分区对应的 broker 并进行长连接
- Broker Cache Consume：consumer 发起 ConsumeRequest，broker 等待 bookie 成功持久化后，dispatch 到订阅的 consumer 的 ReceiveQueue 中，后者可选择同步阻塞、异步通知来 receive 消息
- Broker NonCache Consume：broker 选择任意 bookie 副本读数据。注：
  - bookie 分层存储无 leader / follower 概念
  - 若某个 broker 过慢，consumer 可配置 speculation timeout 重发读取请求给副本 broker，同时等待，以达到预期超时内的读操作

 ![image-20200414141918249](https://images.yinzige.com/2020-04-14-061918.png)



#### 2.1.4  Failure Handling

DESIGN：各环节 failure 由各层次组件独自处理，快速 failover

- producer：对已发消息 pending queue 进行 send timeout 内重发（v2.5 前创建 producer 有异常抛出不会重试）
- broker：crash 对 partition 的 ownership 会被快速转移到其他 broker，生产和 消费客户端都会断开重连到新 broker
- bookie：crash 后会被 auditor 监测到，会通知其他副本会开新的 segment 继续写，对生产消费端透明。随后 auditor 会和其他 bookie 执行 auto recover 进行副本数复制恢复
- consumer：重连（注：broker 的缓存数据已被大多数 bookie ACK，不会丢消息）

四个组件解耦：

 ![image-20200414145511881](https://images.yinzige.com/2020-04-14-065512.png)



### 2.2  Data Retention

#### Cursor

- Subscription Init Position：Earliest, Latest（默认）
- Reader 带 messageID 或时间偏移量可消费指定位置开始的消息，Seek 同理

#### Message Retention

- 积极保留：最慢的 subscription 之后的数据都不能被删除

- Retention 过期：时间或大小限制，只对积极保留之前的消息生效

  注：subscription 下线后 cursor 依旧会保留在 streaming 中，会导致 Retention 无效

- TTL：强制移动旧慢 cursor 到 TTL 时间点

  注：若 TTL == Retention，则与 kafka 一样强制过期

- Msg Backlog（积压）：最慢的 subscription 的 cursor 到最新一条消息之间的消息

- Storage Size：所有未删除的 segments 所占用的字节数
  - 按 segment 粒度删除，以 last motify time 是否早于 retention 为标准过期，与 kafka 一致
  - 注：bookie 并非同步过期，空间释放是后台进程定期 清理或 compact，与 kafka 一致



---



## 3. Topic Discovery, ServiceURL and Cluster

问题：

- pulsar assign topics to broker
- client find broker for target topic

### 3.1  多租户与 topic 分配

多租户：`tenant -> namespace -> topic` 相互对应  develop_department -> app -> topic

分配流程：

- namespace 维护一个 bundle ring，该环会被切分为可配置数量的 bundle 区间（最大 split 为 128，建议为 broker 数量 2~3 倍，topic 分布更均匀）
  - v2.6 新 feature：可选按 topic 数量绑定 bundle，避免少量 topic 时落在单个 broker
- hash(topic.Name) 取 mod 后落到某个 bundle 区间中
- broker 的 LoadManager 组件，负责：
  - 向 zk 汇报负载指标
  - 若为 LoadManager Leader
    - 收集其他 brokers 的负载
    - 根据负载将含有多个 topic 的 bundle 分配给其他 brokers
- 被分配 bundle 的 broker 执行 acquire bundle 加载 topic，完成后处于可服务状态

例：4 个 bundle，6 个 topic，据负载按 2,2,1,1 分配 bundle

 ![image-20200414161951632](https://images.yinzige.com/2020-04-14-081952.png)



按 bundle 分配的优点：

zk 元数据数量极少，支持百万量级 topic，不必记录 topic 到 broker 的映射关系，而是取 hash 值映射到 bundle，而 bundle 又绑定到 broker。



### 3.2  Topic Lookup

lookup 是客户端查找 topic 对应的 broker 的过程，一个 topic 有多个 partitions，一个 partition 由一个 broker 处理（ownership）

- ownership 会以非持久化 ZNode 存在 zk 中，若 broker crash，LoadManager leader 会将 bundle 的 topic 转移到另一个 broker，后者成为新 topic owner

- 结果：任一 broker 可从 zk 获取所有 bundle 到对应 broker 的 ownership

查找流程

- client 向 service URL 中的任一 broker 发起 Lookup 请求
  - Binary 6650：TCP 请求
  - HTTP 8080：GET 请求
- broker 根据 target topic namespace 下的 bundles 数量计算该 topic 对应的 bundle
- broker 从 zk 查询 bundle ownership
- 返回 owner broker 

 ![image-20200414164827683](https://images.yinzige.com/2020-04-14-084828.png)





### 3.2 ServiceURL

#### Lookup 高可用

配置最好不要硬编码

- DNS：需避免过期缓存指向 crashed 的 broker
- LB：探活机制
- HTTP：nginx 反向代理探活
- Multi-Hosts：依次重试



#### Proxies

问题：K8S 中  client 和 owner broker 因网络隔离无法建立连接

Proxies 存储 topic => owner broker 的路由表，提供 broker 查询的 TCP 反向代理

优点：

- 安全：不暴露 broker 地址

- TCP 连接 LB：`clients --> LB --> Proxies <--> owner brokers`（集群 LB：LoadManager leader 分配 bundle）



#### Topic Lookup with Proxies

 ![image-20200414180436781](https://images.yinzige.com/2020-04-14-100437.png)





### 3.3 Cluster

配置 cluster-name 仅用于跨集群复制，创建租户、命名空间需使用 cluster-name

Config URL：在 initialize-cluster-metadata 时指定，供跨机房其他集群连接使用

- web service URL：`http[s]://` admin 资源 CURD 接口
- broker service URL：`pulsar[+tls]://`  二进制 TCP 接口

Operations

- Create：配置远端集群地址而无需 global zk
- Get, List, Update



---

## 4. Geo / Multi-Clusters Replication

### 4.1 双向复制

full-mesh 多集群间双向全量异步复制

#### 4.1.1  Global Config Store

global zk 只存储多个集群的 service URL 配置信息、namespace 配置信息，不存储其他元数据。

DESIGN：指定级别存储多集群元数据，集群间相互感知且保证复制可控

- tenants 粒度：`bin/pulsar-admin tenants create v --alow-clusters <clusters-list>`
- namespace 粒度：`bin/pulsar-admin namespaces create v --alow-clusters <clusters-list>`

复制流程

- cluster 1 创建 replication cursor 记录复制进度
- cluster 1 创建 replication producer 依次发送
- cluster 2 同理

 ![image-20200415113858878](https://images.yinzige.com/2020-04-15-033859.png)

#### 特点

- 不会出现循环复制：复制消息中带 replicate-from 元数据，不会再发送回源。每条消息只会从 source 集群发给所有其他同步集群 ，后者内部不会再复制，即全员复制 1 发到 N-1
- 保证 exactly-once：broker 端有可选的去重机制，replication producer 会将本地 cluster-name 和 messageId 绑定，唯一标识某条消息
- 异步复制：latency 取决于集群间网络。单机房内消息有序，复制到目标机房同样也有序，但因为 latency 存在，不保证目标机房所有消息有序。若全局有序需同步复制。

#### DEMO

- configuration store：全局共享的独立 zk，多集群需 chroot
- 2 个 clusters
  - `admin/clusters/*` ：保存各集群暴露的 service URL
  - `admin/policies/*`：指定 namespace 的 replication_clusters 流控等配置
- 在多集群创建 shared tenant，再创建 shared namespace，即可往 shared topic 双向读写
- tools：pulsarcli 与 pulsar-admin 一样有 context cluster 概念，topic  stats 常用



### 4.2 单向复制

场景（global 模式不支持单向，无需 global zk）

- aggregate：多集群数据聚合到单一集群
- failover：跨机房容灾备份

- center 作为独立集群，edges 集群与 center 保持一致的 tenants/namespaces

其他：

- v2.4 支持 replica subscription 模式，消费状态跨机房会一并复制，消费者切机房后保证不丢消息，但可能重复



---

## 5. Cluster Deployment

- zookeeper: 元数据存储，brokers 和 bookkeeper 的注册中心。
- bookeeper & bookies: message store
  - Role 平等，均可读写
  - 磁盘用量超过配额如 95% 才变为 read-only 状态，可通过 shell listbookies rw/ro 检查状态，simpletest 检测读写
- broker: message serve

  配置 zk 拉取 bookie 地址、cluster-name、minimum bookie 启动数
- Proxies，Producer, Consumer

![img](https://images.yinzige.com/2020-04-15-055607.png)



---

## 6. Bookeeper

概念：

append-base 的分布式日志系统。

特性：

- 高效写操作
- 高容错性：通过 bookies ensemble 的冗余复制来保证。
- 高吞吐：可直接水平扩展 bookie 提高读写吞吐。



![image-20200415144215159](https://images.yinzige.com/2020-04-15-064215.png)



### 6.1  Ledger

类似文件的 append-olny 日志存储系统：create, open, addEntry, close, delete

```shell
bk1-data
└── current
    ├── VERSION
    └── lastMark
bk1-txn
└── current
    ├── 171a0b8f0fd.txn
    └── VERSION
    
# VERSION
4
bookieHost: "10.13.48.57:3181"
journalDir: "/tmp/bk1-txn"
ledgerDirs: "1\t/tmp/bk1-data"
instanceId: "598a4cc0-1754-43ac-90a7-d3e111fe7256"

# WAL journal: 171a0b8f0fd.txn # 二进制 entry
MESSAGE_1=-yp�X�LM���S�ٺ%MESSAGE_4=
```





#### 6.1.2  元数据

Ledger 的 create, open, delete 操作都直接与 zk 交互，操作指定  ledger_id 的元数据：

- State：当前 ledger 写满则 close
- Last Entry ID：当前 ledger 最后一条
- Ensemble, QA, WA 等配置

```shell
zk> ls /ledgers/00/0000
[L0000, L0001, L0002, L0003, L0004, L0005, L0006, L0007, L0008, L0009, L0010, L0011, L0012, L0013, L0014]

zk> get /ledgers/00/0000/L0000
BookieMetadataFormatVersion	2
quorumSize: 2
ensembleSize: 3
length: 0
lastEntryId: -1 # 未写满
state: OPEN
segment {
  ensembleMember: "10.13.48.57:3183"
  ensembleMember: "10.13.48.57:3181"
  ensembleMember: "10.13.48.57:3182"
  firstEntryId: 0
}
digestType: HMAC
password: ""
ackQuorumSize: 2
```



#### 6.1.2   Ledger Entry

结构：

- MetaData
  - `(Lid, Eid)` 能（唯一标识 entry）
  - LAC（Last Add Confirmed）：写入当前 entry 时客户端的 LAC，用作一致性检查
  - Digest：类 CRC 的校验和字节数组
- Data：消息的字节数组

特点：entry 根据 id 有序、唯一地 append-only 写到 ledger 中。

![1](https://images.yinzige.com/2020-04-15-121011.png)





### 6.2  BookKeeper 架构

#### 三个组件

- 强一致性元数据存储：zk / etcd

  - 存储：ledger 元数据

  - 服务发现：bookies 注册中心，供 client 使用

    ```
    zk> ls /ledgers/available
    [10.13.48.57:3181, 10.13.48.57:3182, 10.13.48.57:3183, readonly]
    ```

- 存储节点 bookie：client 选择一组 bookies 作为 ledger ensemble 来存储，接口简单故极易横向扩展。

- 高复杂性 client：实现一致性复制策略的逻辑（bk 最重要组件）

 ![image-20200416122214606](https://images.yinzige.com/2020-04-16-042215.png)



### 6.3  Bookie Server

- 概念：专为 bookeeper 设计的轻量级 KV DB，存储键值对 `(leger_id, entry_id) -> entry_raw_data`
- 实现：bookie 只负责 KV 的 ADD / READ 操作，存储层不保证一致性和可用性。

#### 三种文件

- Journals：保留 bookie 操作 ledger 的事务日志即 WAL，会被持久化存储。

- Entry Logs

  - 有序保存不同 ledger 的 entries，并维护 write cache 实现快速查找。
  - 旧 entry 关联的 ledger 若从 zk 中删除，则后台 GC 线程随之删除落盘了的 entry 日志。

- Index Files：一个 ledger 有自己独立的 index 文件，用于记录 ledger 的 entries 在日志文件中的偏移量。

  

#### ADD 写

- Journal：只负责缓存 entry，将来自不同 ledger 的 entry 直接追加写文件
  - 磁盘顺序写速度快，fsync 低时延同步落盘，写满后开新 journal 文件继续写。
  - 可配置 flush 条件：`journalMaxGroupWaitMSec`、`journalBufferedWritesThreshold=`
- **问题**：随机读 entry，必须维护 `(lid, eid) -> entry `的索引结构。不在 Journal 上构建索引
  - 让 Journal 只写而不随机读，吞吐更高
  - 原地无法做排序，整个 ledger 扫描很低效

#### READ 读

- bookie 在其 JVM 进程内存中维护 Write Cache 缓冲池，缓存已成功追加到 journal 的 Entry
- 在缓存变满迁，对 cache 中对 entry 按 ledger_id 重新排序，保证刷盘时顺序地按 ledger_id 写
- 缓存变满或定期操作，都会 flush 缓存中的两部分数据，还是顺序写：
  - LedgerIndex：各个 index 内部各 entry_id 到 entry log 的映射关系。两种实现 ：
    - DB ledger storage：用 rocksdb 存储 entry index
    - Sorted ledger storage：用文件
  - EntryLog：顺序存储 ledger
- Flush 完成后，journal 文件会被清理



![1](https://images.yinzige.com/2020-04-16-043438.png)



#### QA

- entry id 如何产生：

  一个 ledger 只会有一个 client writer，从而保证 client 可以生成并从 0 顺序递增 entry_id ，与 bookie server 无关。

- write cache：skip-list 实现，并非在 flush 前才统一排序。



### 6.4  Bookie Client Write

负责 entry 读写，副本复制，bookkeeper 数据一致性保证



#### 6.4.1  Quorums

- Ensemble Size = 5：client 会选 5 台 bookie 来存 ledger entry
- Write Quorum Size = 3：每条 entry 按 round robin 策略顺延分布存在 3 台 bookie 上（黑点）
- ACK Quorum Size = 2：每条 entry 有 2 台 bookie 响应 ACK 即认为写成功



#### 6.4.2  LAP 与 LAC

- LastAddPushed (LAP)：client 发出的最后一条的 entry_id，从 0 递增
- LastAddConfirmed (LAC)：client 最后一条 ACK 成功的 entry_id，是一致性的边界

  - LAC 必须根据 LAP ***依次顺序确认***，故能保证 LAC 前的 entry 都已成功存储在 bookie 上
  - `(LAC, LAP]` 区间的 entries 正在被 bookie 存储，还未收到响应
- 注：发送 entry 时会将 LAC 附加在 entry 元数据后发往 bookie，整个写数据过程中，仅在打开和关闭 ledger 时才与 zookeeper 交互，更新 ledger 元数据。





#### 个人理解

- LAC 与 LAP 的存在，使 entry 能以内嵌顺序元数据的方式，均匀分散存储到各台 bookie 中。

- LastAdd* 机制与 Raft 不同之处在于：

  > 各 bookie 节点的数据不是从单个节点异步复制而来，而是由 Client 直接轮询分发。

  - 为保证 bookie 能快速 append 日志，设计了 Journal Append-only 顺序写日志机制。
  - 为保证 bookie 能快速根据 `(lid, eid)` 读取消息`(entry)`，设计了 Ledger Store

  因此，各 bookie  存储节点的身份是平等的，没有传统一致性算法的 Leader 和 Follower 的概念，完美避开了读写只能走 Leader 导致 Leader 容易成为单点瓶颈的问题。

  同时，能直接添加新 bookie 存储节点，提升 Client 的读写吞吐，降低其他旧 bookie 的负载。







#### 6.4.3  Ensemble Change

bookie failover 流程

- 写请求转移：加入新 bookie 后直接继续写，写不会中断，后续旧 entry 再异步从其他 bookie 复制恢复，保证写请求高可用。

- 记录 LAC 分界线：修改 ledger metadata 中的 ensembles，在 zookeeper 上记录每次 failover 导致 LAC 重新分布哪些 bookie 上。

  如 3183 crash 后 3182 上线：

  ```yaml
  zk> get /ledgers/00/0000/L0016
  BookieMetadataFormatVersion	2
  quorumSize: 2
  ensembleSize: 3
  length: 0
  lastEntryId: -1
  state: OPEN
  segment {
    ensembleMember: "10.13.48.57:3185"
    ensembleMember: "10.13.48.57:3184"
    ensembleMember: "10.13.48.57:3183"
    firstEntryId: 0
  }
  segment {
    ensembleMember: "10.13.48.57:3185"
    ensembleMember: "10.13.48.57:3184"
    ensembleMember: "10.13.48.57:3182"
    firstEntryId: 47
  }
  digestType: HMAC
  password: ""
  ackQuorumSize: 2
  ```

  

![bookkeeper-protocol](https://images.yinzige.com/2020-04-16-050710.png)





### 6.5  Bookie Client Read

#### 6.5.1  LAC 与 failover entry_id 区间

- LAC 保证其之前的 entry 都可读，其后消息不可读。

- Client 可从 zookeeper 中读出指定 ledger_id 的 failover entry_id  区间，不同区间对应从不同的 ensembles 读取。

#### 6.5.1  均摊读

如同轮询写，Cleint 也会轮询读 ensembles 以均摊读取器，如 3 个 bookie 存放了 entry_id [0, 9K) 的 entry，则可均摊：

- 从 bookie1 读 `[0, 3K)`
- 从 bookie2 读 `[3K, 6K)`
- 从 bookie3 读 `[6K, 9K)` 

优点：同样不存在 leader 读瓶颈。

#### 6.5.2  预期读

若某个 broker 读响应确实很慢，client 会向其他副本 bookie 发起读请求，同时等待。类似机制与 consumer 配置 speculation timeout 同理，保证读操作可在预期时间内返回，从而实现**低延时响应**。

#### 6.5.3  无序读

因为 Client 往 bookie 写是轮询无序地写，故从 bookie ensemble 中读到是消息是无序的，需在 Client 端自行按 entry_id 重新排序，以保证有序。



### 6.6  Distributed Log 与 Raft

broker 即 Writer，每个 partition 会选出  leader broker，它会不断 create ledger 并保持 append 写。若该 broker crash，则会根据负载重新选出新 leader broker，新 broker 会**直接 create 新 ledger** 写数据。

| 概念        | Raft                       | DL                         |
| ----------- | -------------------------- | -------------------------- |
| role        | Leader => Followers        | Writer (broker) => Bookies |
| failover    | term                       | ledger_id                  |
| replication | Majority AppendEntries RPC | QW AddEntry RPC            |
| consistency | Last Committed Index       | Last Add Confirmed（LAC）  |
| brain split | Majority Vote              | Bookie Fence               |



写流程对比：

 <img src="https://images.yinzige.com/2020-04-22-122805.png" alt="image-2020042220280482" style="zoom:45%;" />



### 6.7  QA

- 是否存在多个 Clients 竞争写 Journal ?

  不会。bookeeper 中 client 和 server 在网络层面没有 ledger 概念，在 server 端仅仅只是从多个 client sockets 中收到多个 AddEntry 请求，在 server 内部会依次进行请求处理，顺序写日志。

  另，bookie 的配置项 `journalDirectories` 可对应多个目录，对应增加 `numJournalCallbackThreads` 可并发写 Journal 日志，提高写吞吐。

- 为 journal 配置多个 目录 ，如何保证目录负载均衡？

  目前是尽可能均摊。实现上是单个 ledger 所有 entry 由单个 journal thread 负责缓存，后者写单个目录。journal 并不在乎 entry 具体写哪个缓存目录，但能避免跨目录写的复杂性，同时保证每个 journal 平摊 ledgers

- zk 是否会成为瓶颈？

  一般情况不会。zk 混用 watcher 和数据持久化机制时会有问题，bookkeeper 仅 open, close ledger 时更新 zk

- 先写 journal 还是 write cache？

  和一般 kvDB WAL 不同，bookkeeper 是先写 write cache，才写 journal，因为 LAC 之后的 entry 对 reader 不可见。

- 如何规划目录？

  - journal 目录：按写吞吐需求，如 800M/s 的写每块盘上限 200MB/s 需至少 4 块。最好 SSD，HDD 关闭同步刷盘配置。
  - ledger 目录：写读都有，需同时考虑吞吐和容量。





## 7. BookKeeper Continue

### 7.1  Broker Recovery: Fence

#### 场景

broker crash 或 partition leader broker 发生脑裂，导致 broker 与 zk 出现网络分区，需进行 partition ownership 的转移。

#### 流程

- broker2 向 bookie ensemble 发起 Fence ledger X 请求。
- bookies 将 ledger x 置为 Fence 不可写状态。
- broker1 收到 FenceException，知道有其他 broker 已经接管该 partition，主动放弃 ownership
- client 发现异常，与 broker1 断开连接，进行 topic discovery 找到 broker2
- 同时，broker2 对 ledger x LAC 之后的 entry 依次逐一进行 forwarding recovery（若 entry 已达到 WQ 则认为该 entry 写成功，LAC 自增）
- broker2 将 ledger x 置为 CLOSE 状态，再创建新 ledger，继续处理 client 的写请求。

![image-20200423113838368](https://images.yinzige.com/2020-04-23-033838.png)



#### 特点

- 数据一致性与正确性由 bookie 保证，而非无状态的 broker

  bookie 不继续 append 旧 ledger，直接开新 ledger 继续写的原因：若复用旧 ledger，必须保证所有 bookie ensemble 的 LAC 一致，还要涉及尾部 entry 的强一致复制，直接 CLOSE 能保证旧 ledger 不会再被写入，降低复杂度。

- zookeeper 仅管理 ownership，各 bookie 仅会复制 ledger 末端堆积的 entry，流程简单，秒级恢复。



### 7.2  Bookie Recovery

#### 场景

bookie  crash，需进行副本恢复。

#### 流程

- 通过 zk 选出 leader bookie10 作为 auditor，不断向其他 worker bookies 发送心跳保活
- auditor 发现 bookie1 x crash，对其上的 ledger_x，会找出其他 ensemble bookies2~3 和新的能接手的 bookie4
- 从 bookie2~3 按轮询均摊复制压力的方式，将 entry 逐一复制到 bookie4 上
- 复制完毕后修改 zk 元数据，将 4 加入到 ledger_x 的 ensemble，接收读写请求

![image-20200423130215577](https://tva1.sinaimg.cn/large/007S8ZIlly1ge3m7eud8nj30gd0dnmxn.jpg)



#### 存在问题

bookie 目前不会记录复制的中间状态，只保证最终复制成功的结果。

复制 ledger_x 的过程中若 bookie4 crash，auditor 能感知到并会重复以上流程，找出 bookie5 重新开始复制。在 bookie4 上会残留部分无效的 ledger_x 复制数据会被 lazy GC 定期从 zk 检查出删除后再删除，broker 删除 topic 数据也是一样定期清理的逻辑。



### 7.3  Bookkeeper with k8s

#### Cookie & Bookie Identifier

zk 中各 ledger 的元数据直接指定了 bookie 的 IP，容器部署时若 bookie 重启后 IP 更新，会导致旧 ledger 元数据该副本作废，所以应该用 DaemonSet 或  StatefulSet 和域名部署。

```shell
zk> ls /ledgers/cookies
[10.13.48.57:3181, 10.13.48.57:3182, 10.13.48.57:3183, 10.13.48.57:3184, 10.13.48.57:3185]

zk> get /ledgers/cookies/10.13.48.57:3181
4
bookieHost: "10.13.48.57:3181"
journalDir: "/tmp/bk1-txn"
ledgerDirs: "1\t/tmp/bk1-data"
instanceId: "a93b0c0c-344c-414f-b451-7b94be8ec59f"
```



#### PV 和 LPV

k8s 部署将 AutoRecovery 组件 run 在独立的 pod 中，物理机集群部署则直接在 bookie 中开启即可。





















