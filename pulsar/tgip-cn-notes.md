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
  - broker 也在内存记录 non-durable 的 cursor
  - 根据 cursor 可直接取差值得 LAG，Reader 的概念在 kafka 中 LAG 需在 consumer 端自己计算，如 flink
  
- 多租户：三层层级化
  - `domain(persistent) ://  tenant / namespace / topic`
  - domain 分为 non-persistent 不保存消息、默认 persistent（简写 p）
  - 如 topicA => 对应的 Fully Qualified Name 为 `p://public/default/topicA`



---



## 2. Data Lifecycle

### 2.1 Data Flow

#### 计算与存储分离

- broker：处理生产消费请求，stateless 无状态
- bookie：作为 bookkeeper 节点持久化消息

元数据存储：zookeeper（插件接口可替换为 etcd）

 ![image-20200414135154659](https://images.yinzige.com/2020-04-14-055155.png)



#### 生产流程

- Producer Route Partition：选定要写的分区
  - producer 默认 round robin 选择，batch interval 内复用同一 partition
  - 若提供 KEY 则 hash
  - 可自定义 router 规则
- **Producer Topic Discovery**：首次向任意 broker LookUP 目标分区所在的 broker（LB 层），发起长连接（复杂设计如连接池）

- Broker Write Bookie：并发调用 BK 客户端写数据，大多数 ACK 则认为写成功

 ![image-20200414140907444](https://images.yinzige.com/2020-04-14-060908.png)





#### 消费流程

- Topic Discovery：按消费模式找出分区对应的 broker 并进行长连接
- Cache Consume：consumer 发起 FlowRequest，broker 等待 bookie 成功持久化后，dispatch 对应 consumer 的 ReceiveQueue 中，后者可选择同步阻塞、异步通知来 receive 消息
- NonCache Consumer：broker 选择任意 bookie 副本读数据。注：
  - bookie 分层存储无 leader / follower 概念
  - 若某个 broker 过慢，consumer 可配置 speculation timeout 重发读取请求给副本 broker，同时等待，以达到预期超时内的读操作

 ![image-20200414141918249](https://images.yinzige.com/2020-04-14-061918.png)



#### Failure Handling

DESIGN：各环节 failure 由各层次组件独自处理，快速 failover

- producer：对已发消息 pending queue 进行 send timeout 内重发（v2.5 前创建 producer 有异常抛出不会重试）
- broker：crash 对 partition 的 ownership 会被快速转移到其他 broker，producer 会断开重连到新 broker
- bookie：crash 被 notify 后其余副本会被 broker 选中，开新的 segment 继续写，对生产消费端透明
  - 注：随后 bookkeeper 会和其他 bookie 执行 auto recover 进行副本数复制恢复
- consumer：重连（注：broker 的缓存数据已被大多数 bookie ACK，不丢消息）

四个组件解耦：

 ![image-20200414145511881](https://images.yinzige.com/2020-04-14-065512.png)



### 2.2 Data Retention

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
  - 注：bookie 并非同步过期，空间释放是后台进程定期 15min 清理或 compact，与 kafka 一致



---



## 3. Topic Discovery, ServiceURL and Cluster

问题：

- pulsar assign topics to broker
- client find broker for target topic

### 3.1 Topic Lookup

#### 多租户与 topic 分配

多租户：`tenant -> namespace -> topic` 相互对应  develop_department -> app -> topic

分配流程：

- namespace 维护一个 bundle ring，该环会被切分为可配置数量的 bundle 区间（最大 split 为 128，建议为 broker 数量 2~3 倍，topic 分布更均匀）
  - v2.6 新 feature：可选按 topic 数量绑定 bundle，避免少量 topic 时落在单个 broker
- hash(topic.Name) 取 mod 后落到某个 bundle 区间中
- broker 的 LoadManager 组件，负责：
  - 向 zk 汇报负载指标
  - 若为 LoadManager Leader
    - 获取其他 broker 负载情况
    - 根据负载情况将含有多个 topic 的 bundle 分配给其他 broker
- 被分配 bundle 的 broker 执行 acquire bundle 加载 topic，完成后处于可服务状态

例：4 个 bundle，6 个 topic，据负载按 2,2,1,1 分配 bundle

 ![image-20200414161951632](https://images.yinzige.com/2020-04-14-081952.png)



按 bundle 分配的优点：

- zk 元数据数量极少：支持百万量级 topic，不必记录 topic 到 broker 的映射关系，而是取 hash 值定位 bundle



#### 客户端查找 topic 对应的 broker

topic owner 是某个 broker

- 因为按 bundle 批量分配 topics 给 broker，也是 bundle owner
- ownership 会以非持久化 znode 存在 zk 中，若 broker crash，LoadManager leader 会将 bundle 的 topic 转移到另一个 broker，后者成为新 topic owner

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

#### Global Config Store

global zk 只存储多个集群的 service URL 配置信息、namespace 配置信息，不存储其余元数据。

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

- zookeeper: metadata sotre, (bookies, brokers) service discovery center
- bookeeper & bookies: message store
  - Role 平等，均可读写
  - 磁盘用量超过配额如 95% 变为 read-only 状态，可 listbookies rw/ro 检查状态，simpletest 检测读写
- broker: message serve
  - 配置 zk 拉取 bookie 地址、cluster-name、minimum bookie 启动数
- Proxies，Producer, Consumer

![img](https://images.yinzige.com/2020-04-15-055607.png)



---

## 6. Bookeeper

概览

![image-20200415144215159](https://images.yinzige.com/2020-04-15-064215.png)



### 6.1 Ledger

DESIGN

- 类文件的 append-olny 日志存储系统：打开关闭文件，追加写数据
- 元数据存储：ledger 状态、副本信息

结构：

- 元数据（类 iNode）

  - State：open、close（当前 ledger 已写满）
  - Last Entry ID

  - Ensemble 大多数读写配置

- entry

  - MetaData
    - Lid， Eid（唯一标识 entry）
    - LAC（最后一条已确认添加记录）：一致性检查
    - Digest：data 校验和
  - Data：字节数组消息

![1](https://images.yinzige.com/2020-04-15-121011.png)





### 6.2 BookKeeper 架构

- 元数据存储、bookies 注册与服务发现：zk / etcd

- 存储节点 bookie：注册到 zk 后供 Client 写 ledger，复杂性低

- 高复杂性 client：实现一致性复制策略

 ![image-20200416122214606](https://images.yinzige.com/2020-04-16-042215.png)



### 6.3 Bookie Server

#### DESIGN

- 概念：非通用性的轻量级 KV DB，键是`（Lid, Eid)`，值是 `ledger entry`
- 实现：只提供 kv 的读写，不保证一致性、可用性

#### ADD 写

- journal：只负责缓存日志，将来自不同 ledger 的 entry 直接追加写文件
  - 磁盘高速顺序写：fsync 低时延刷盘（interval 可配置），写满后新开 journal 文件写
- 问题：不支持随机读。必须在外围维护 `(lid, eid) -> entry `的索引结构

#### READ 读

- bookie 在 JVM 进程内存中维护 write cache 缓冲池，存放已成功追加到 journal 的 Entry
- 在缓存变满前，对池中对 entry 进行排序，保证刷盘时按 ledger 顺序写
- flush 两部分
  - Index：entry 索引
    - DB ledger storage：使用 rocksdb 存储 index
    - Sorted ledger storage：使用文件
  - EntryLog：顺序存储 ledger
- ***NOTICE***：flush 完成到 ledger storage 后，journal 的 entry 文件会被清理

#### QA

- entry id 如何产生
  - 与 bookie 无关，由 client（broker） 生成
  - 一个 ledger 只会有一个 client writer，顺序递增
- write cache：skip-list 实现

![1](https://images.yinzige.com/2020-04-16-043438.png)





### 6.4 Bookie Client

#### DESIGN

负责副本复制、读写 entry、一致性保证



#### Write Quorums

- Ensemble Size = 5：client 会选 5 台 bookie 来存 ledger entry

- Write Quorum Size = 3：每条 entry 按 round robin 策略顺延分布存在 3 台 bookie 上（黑点）

- ACK Quorum Size = 2：每条 entry 有 2 个 Write 响应 ACK 即认为写成功

- LastAddConfirmed LAP：client 发出的最后一条的 eid

- LastAddConfirmed LAC：client 最后一条 ACK 成功的 eid，是一致性的边界

  - LAC 必须根据 LAP ***顺序确认***，故能保证 LAC 前的 entry 都已成功存储在 bookie 上
  - `(LAC, LAP]` 区间的 entries 正在被 bookie 存储，还未收到响应

  - 注：发送 entry 时会将 LAC 附加在 metadata 中，发往 bookie，并不存在 zk 中



#### Ensemble Change

bookie failover 流程

- 直接加入新 bookie 继续写，后续旧 entry 再异步从其他 bookie auto recovery
- 修改 ledger metadata 中的 ensembles，记录每次 failover 导致 LAC 重分布哪些新 bookie 上

![bookkeeper-protocol](https://images.yinzige.com/2020-04-16-050710.png)





#### Read

LAC 保证其之前的 entry 都可读，其后消息不可读。

- 均摊读吞吐，不存在 leader 瓶颈
  - 均摊读：根据 entry 的 ledger id 能找出该消息分布在哪几个 bookie 上，又因为写时为 round robin 均摊写，故读请求能均匀分散到 bookies 上，而不是像 kafka 那样只能从 leader 读写，即 bookie 身份平等
  - 后备机制：若某个 broker 读响应确实很慢，client 会向其他副本 bookie 发起读请求，同时等待。类似机制与 consumer 配置 speculation timeout 同理，保证读可以预期返回，从而实现低延时响应

- entry 顺序性由 client 自行组装：entry 从 bookie 读取没有顺序性



#### 总结

- 计算节点易扩展：bookie 功能简单，只接收客户端发来的 entry（不在乎顺序），存储在 ledger 后直接响应
- TODO：ledger 类 Raft 实现



## 7. BookKeeper Continue

TODO









