---
title: Pulsar 文档笔记
date: 2020-04-17 14:37:11
tags: pulsar
---

「持续更新」[Pulsar docs/en](https://pulsar.apache.org/docs/en/pulsar-2.0/) 文档简要笔记。

<!-- more -->

## 1. Get Started

### 1.1 Pulsar 2.0

FQN 中废弃了 cluster 和 property，进一步抽象出 tenants 概念

简写 topic name `my-topic` 会转换到默认的 FQN：`persistent://public/default/my-topic`

无需持久化的 topic 必须指定全部 FQN：`non-persistent://public/default/my-topic`

### 1.2 Run Pulsar locally

standalone 部署时所有组件 zk, broker, bookkeeper 均在单个 JVM 进程中，默认 JVM heap 2GB 可配置，可选 pulsar-daemon start

启动时自动创建 `public/default` namespace 管理所有 topics

### 1.3 Run Pulsar in Docker

指定 service 和 HTTP 端口、data 和 conf 的 volume：

```dockerfile
docker run -it -p 6650:6650 -p 8080:8080 \
--mount src=pulsardata,dst=/pulsar/data \
--mount src=pulsarconf,dst=/pulsar/conf \
apachepulsar/pulsar:2.5.0 \
bin/pulsar standalone
```

DEMO 中 producer 发出的消息必须是 bytes，pulsar/schema/schema.py#48 会做类型校验。

Admin API：提供 API 查看和操作 FQN 内的资源。如 GET topic/stats，返回整个 topic 吞吐、最大堆积、现有数据量、订阅列表、各订阅组内消费模式等详细信息

```json
{
  "msgThroughputIn": 0,
  "msgThroughputOut": 1109.6379138631598,
  "storageSize": 68398,
  "backlogSize": 46,
  "subscriptions": {
    "test-sub-name": {
      "msgThroughputOut": 1109.6379138631598,
      "msgBacklog": 0,
      "type": "Exclusive",
      "consumers": [{
          "msgThroughputOut": 1109.6379138631598,
          "address": "/172.17.0.1:47872",
        }],
      "isReplicated": false
    }
  },
  "replication": {},
}
```

### 1.4 Run Pulsar in k8s

**TODO**





## 2. Overview

### 2.1  特性

- 原生支持跨集群间数据复制
- 百万级别的 topic 支持
- 生产消费延迟极低
- 多种订阅模式
- 通过 bookkeeper 保证数据持久化可靠性存储
- 对 functions，connector 的支持，存储分层支持多种存储引擎



### 2.2  Messaging

#### 2.2.1  消息体（Message）

- **Value / data payload**：即使消息满足 data schemas，依旧以 raw bytes 传输。
- **Properties：**可选，存放自定义 kv
- **Producer name：**可选
- **Sequence ID：**本条消息在该 topic 的全局顺序 ID
- **Publish time：**消息发布时间
- **Event time：**可选，消息生成时间等



#### 2.2.2  Producer

##### 两种 Send 模式

- Sync Send：每条消息都发送后都必须等 broker 返回 ACK
- Async Send：producer 往消息队列 put 消息后直接返回，client 在后台读队列发送，队列（大小可配置）满则 produce 阻塞。

Compression：支持 Snappy, LZ4, ZLIB, ZSTD 等压缩算法。

Batching：支持单次请求发送一批消息，由 max_number_of_messages 与 max_publish_latency 共同配置。



#### 2.2.3  Consumer

##### 两种 Receive 模式

- Sync Receive：阻塞等待。
- Async Receive：调用返回 future 后由 consumer 自行阻塞等待，比如放入 event loop

##### 消息监听器（Listeners）

客户端只需实现监听接口如 MessageListener，新消息到达时就会调用 received

##### ACK

- 可消费时逐条 ACK，也可以消费完 ACK 最后一条
- 注：共享订阅模式下，每个 consumer 只会收到部分消息，故不能累积 ACK，只能逐条 ACK

##### 取消 ACK

若 consumer 内部消费出错，可请求重新消费，发送取消 ACK 后 broker 会重发消息

- exclusive, failover：只能取消上一次提交的 ACK，单个 consumer 可控回滚。
- shared, key_shared：consumers 各自取消各自不同的 ACK

##### ACK Timeout

若 consumer 未消费成功时想让 broker 自动重发，而非每次都要显式取消 ACK，可让 broker 自动重发超时后仍未 ACK 的消息给 consumer（unacknowledged message automatic re-delivery mechanism）

consumer client 会在 `acktimeout` 时间内跟踪还未 ACK 的消息，超时后向 broker 发起 `redeliver unacknowledged messages` 请求，请求重发这些还未 ACK 的消息。

注意：应在 ACK Timeout 前使用取消 ACK。取消 ACK 能细粒度、可控地要求 broker 重发消息，也能避免无效的自动重发。

##### 死信 topic

若 consumer 有的消息处理失败，又不得不继续消费，可将这部分消息单独放在死信 topic，后续自行处理。死信 topic 的消息来源于取消 ack 或 ack timeout 重发的消息。

命名为 `<topicname>-<subscriptionname>-DLQ`，仅 shared 订阅模式下可用。

```java
Consumer<byte[]> consumer = pulsarClient.newConsumer(Schema.BYTES)
              .topic(topic)
              .subscriptionName("my-subscription")
              .subscriptionType(SubscriptionType.Shared)
              .deadLetterPolicy(DeadLetterPolicy.builder()
                    .maxRedeliverCount(maxRedeliveryCount)
                    .build())
              .subscribe();                
```



#### 2.2.4  Topics

以 URL 形式定义：`{persistent|non-persistent}://tenant/namespace/topic`

- [non-]persistent：是否将消息数据持久化存储，默认持久化
- tenant：实现多租户，<u>可跨集群</u>
- namespace：topic 分组的管理单元
- topic：名字

注：无需显式创建 URL，当尝试读写不存在的 topic 时，pulsar 会自动按 URL 创建 topic



#### 2.2.5 Namespaces

只是 tenant 中的逻辑命名，可通过 Admin API 创建，tenant 可将其下的多个 app 映射到相应 namespace 下，再进一步管理 topics



#### 2.2.6 订阅模式（Subscription modes）

subscription mode 决定 broker 如何分发消息给 consumer，有四种：

<img src=https://images.yinzige.com/2020-04-17-064414.png style="zoom:40%;" />

- **Exclusive** **（默认）**：一对一，保证有序消费

  - 同一个 subscription 内只能有一个 consumer 来消费，否则报错 `Pulsar error: ConsumerBusy`

- **Failover**：一对一，备选多，保证有序消费

  - 消费者按 name 排序，排第一的 consumer 会先收到消息，称为 master consumer
  - 若 master consumer 下线，它还未 ACK 和后续的消息将顺延交给下一个备用 consumer 处理。
  - 场景：消息量少，解决消费者单点故障。

- **Shared**：多对多，round robin 不保证有序消费

  - 在 shared 或 round robin 模式下，同一 subscription 可包含多个 consumers，broker 轮询将消息下发给 consumer，若某个 consumer 下线，则它未 ACK 的消息会重发给其他 consumers

  - 注意：不能累积 ACK

  - 场景：可水平扩展 consumer（并非 consumer group，不像 kafka 受限 partitions 数量），来提高吞吐。

    ![Shared subscriptions](https://images.yinzige.com/2020-04-17-071342.png)

- **Key_Shared**：多对多，按 Key hash 保证有序消费

  - 同一 Key 或相同 ordering keys 会被只会被同一个消费者读取，不管该消息是否被重发。
  - 若 consumer 下线下线，会导致其他 consumer 一部分 keys 的消息新增或减少。
  - 注意：不能累积 ACK，必须在 producer 端指定 key 或 ordering keys

  ![Key_Shared subscriptions](https://images.yinzige.com/2020-04-17-072357.png)

 

#### 2.2.7  多 topic 订阅（Multi-topic subscriptions）

订阅：

- 正则匹配订阅同一 namespace 下的多个 topic：`persistent://public/default/finance-.*`

- 明确指定要消费的 topic [URL] list

注意：

- consumer 正则取匹配或 list 订阅现有的 topic，若 list 中 topic 不存在并不会报错，而是创建出来后 broker 帮其自动订阅。
- producer 能保证单一 topic 至少消费顺序可以一致，但同时往多 topic 写的消息不保证消费顺序。

DEMO：

```python
import pulsar, re

client = pulsar.Client(service_url='pulsar://localhost:6650')
consumer = client.subscribe(
    # topic=['persistent://public/default/test-topic-1', 'persistent://public/default/test-topic-2', ],
    topic=re.compile('persistent://public/default/test-topic-.*'),
    subscription_name='test-sub-name',
)
while True:
    msg = consumer.receive()
    print('received: ', msg.data())
    consumer.acknowledge(msg)
```





#### 2.2.8  分区 topics（Partitioned topics）

为提高吞吐，需将单个 topic 切分为多个（分区数）内部 partitioned topics 逐一分布在多个 broker 上，每条消息只会被 route 到一个 broker 处理，pulsar 内部会根据各 broker 负载自动分布分区，无需人工干涉。

分区对 broker `=>` 一对一，反之，broker 对分区 `=>` 多对一

##### 架构

topic 默认只有一个分区只由一个 broker 处理，分区 topic 需用 Admin API 手动创建。

- 路由策略（routing mode）：partition 分配给 broker 的分配策略，会影响吞吐。
- 订阅模式（subscription mode）：broker 推消息给 consumer 的下发策略，会影响消费逻辑。

<img src="https://images.yinzige.com/2020-04-17-075025.png" style="zoom:45%;" />

##### 路由策略（routing mode）

发送消息时必须指定路由策略，其决定每条消息会发往哪个分区。三种策略可选：

- **RoundRobinPartition（默认）**
  - 无 Key：以 batching 为单位，通过轮询将消息均匀发给 brokers，以获得最大吞吐。
  - 有 Key：只往类似 `hash(key) mod len(partitions)` 指定的分区写。

- **SinglePartition**
  - 无 Key：随机选一个分区，将后续所有消息都写入该分区。
  - 有 Key：只往 hash 指定的分区写。

- **CustomPartition**：用户可自定义针对具体到消息的分区策略，如实现 `MessageRouter` 接口。



##### 顺序保证（Ordering Guarantee）

消费者读消息的顺序，与生产方发消息顺序是否一致的问题，与路由策略、消息是否带 Key 都有关，通常需求是 Key 分区内有序。

当使用前两种路由策略时，带 KEY 的消息会被 producer 的 `ProducerBuilder.HashingScheme`  hash

| 顺序保证                        | 描述                                                       | 路由策略与消息 KEY  |
| ------------------------------- | ---------------------------------------------------------- | ------------------- |
| Key 内有序（Per-key-partition） | 所有相同 Key 的消息都会有顺序地落到同一分区                | RoundRobin / Single |
| Producer 有序（Per-producer）   | 从单个 producer 发出的消息有顺序（只落到单个分区才能保证） | 仅 Single           |

总结：强一致顺序，只能所有消息最终都落到一个分区内才能保证。



##### Hash 协议（Hashing Scheme）

HashingScheme 是生产者可用的 hash 枚举集合。有两种标准 hash 函数可用：`JavaStringHash` 和 `Murmur3_32Hash`，前者是 Java 客户端默认的 hash 函数，其他语言客户端不可用，所以跨语言的 producer 推荐使用后者。





#### 2.2.9  非持久化 topic（Non-persistent topics）

topic URL：`non-persistent://tenant/namespace/topic`

 默认非持久化功能不开启，需修改 broker 配置，可使用 pulsar-admin topics 管理，只在某些 case 下比要持久化稍快，慎用。

对于不持久化的 topic，消息仅暂存在 broker 的内存中，不会落盘。broker 会立刻将消息下发给所有的 subscribers：

- 若 broker crash 则消息则彻底丢失。
- 若其中一个 subscriber 下线，broker 将无法继续下发消息，而其他 subscribers 会收不到这部分传输中的消息（TODO）



##### 性能

non-persistent 模式下，producer 只管发，broker 也不回复 ACK，写吞吐很高，latency 也相对低。

##### Consume Mode

四种消费模式都支持非持久化 topic 的消费





#### 2.2.10  消息保留与过期（Message retention and expiry）

##### 默认策略

- 及时删除所有 subscriptions 都已 ACK 的消息
- 持久化 backlog 积压消息中还有 subscription 未 ACK 的消息

##### 自定义策略：仅 namespace 级别有效

-  Retention：指定已 ACK 的消息保留时间
- TTL：指定未 ACK 的消息保留时间（强制过期）

![Message retention and expiry](https://images.yinzige.com/2020-04-17-092103.png)



#### 2.2.11  消息去重（Message deduplication）

可选的消息精确去重机制 ，哪怕是 producer 端故意重发的，broker 都能保证每条消息只会被持久化一次。

-  默认只在 namespace 级别生效。
- 生产方写消息幂等性：需由消费方实现去重逻辑，但 pulsar 原生支持 effectively-once 语义，可与流处理引擎 SPE 结合。

![](https://images.yinzige.com/2020-04-17-095233.png)



#### 2.2.12 消息延迟下发（Delayed message delivery）

配置 consumer 延迟消费，当消息成功持久化到 bookkeeper 后，broker 的 `DelayedDeliveryTracker` 会在内存中维护时间索引（time => messageId），当 delay 延迟超时后才下发

- 延迟下发仅在 shared 消费模式下生效。反而在 exclusive 和 failover 中，延迟的消息会立刻发出。

![Delayed Message Delivery](https://images.yinzige.com/2020-04-17-100820.png)

- broker 收到消费请求后，检查消息若配置了延迟下发，则放入 `DelayedDeliveryTracker`，超时后再发出。默认开启，可配置：

  ```shell
  # Whether to enable the delayed delivery for messages.
  # If disabled, messages are immediately delivered and there is no tracking overhead.
  delayedDeliveryEnabled=true
  
  # Control the ticking time for the retry of delayed message delivery,
  # affecting the accuracy of the delivery time compared to the scheduled time.
  # Default is 1 second.
  delayedDeliveryTickTimeMillis=1000
  ```

- producer 支持指定延迟时间，<u>py 未支持</u>

  ```java
  // message to be delivered at the configured delay interval
  producer.newMessage().deliverAfter(3L, TimeUnit.Minute).value("Hello Pulsar!").send();
  ```



---

### 2.3  Architecture

组件

- brokers：负载均衡地从 producer 端接收消息并分发给 consumer，与 configuration store zk 交互执行协调任务，消息持久化到 bookkeeper
- bookkeeper：多个 bookie 持久化消息。
- zk：负责集群内部协调

 <img src="https://images.yinzige.com/2020-04-20-070926.png" alt="Pulsar architecture diagram" style="zoom:55%;" />

#### 2.3.1  Brokers

- Broker 本身无状态 stateless，内部运行这两个子组件：
  - HTTP Server：通过 REST API 方式执行 admin 操作，处理 Producer/Consumer 的 Topic Lookup 请求。
  - Dispatcher：异步处理二进制协议请求的 TCP Server

- Ledger Cache：为提高性能，Broker 直接从  write cache 中分发消息给 Consumer，若滞后的消息过多，才会从 Bookkeeper 读取。
- Replicator：用 Pulsar Java Client 将 publish 本地集群的消息，republish 到异地集群。



#### 2.3.2  Cluster

单个集群包括：多个 brokers、一个 zk 集群做集群级的配置中心和协调服务、一组 bookies 做持久化。



#### 2.3.3  元数据存储（Metadata store）

- configuration store：tenants, namespace, 及其他需保持全局一致性的数据。
- coordination：broker 与 topic 的 ownership，broker 负载报告，bookkeeper ledger 的元数据等。



#### 2.3.4   持久化存储（Persistent storage）

只要生产消息到达 broker，pulsar 会将未 ACK 的消息持久化存储，从而保证能送达 consumer



#### 2.3.5  BookKeeper

WAL 分布式日志系统，特性：

-  使 pulsar 能使用多个独立日志，称为 ledger
- 对于 entry replication 使用顺序存储，十分高效。
- 保证各 failures 下的读一致性
- 保证 IO 负载均摊到各 bookie
- 水平扩展可线性增加容量和吞吐
- bookies 被设计为支持上千 ledgers 的同时并发读写，多个磁盘一个用于 journal 日志，其他则负责存储，前者读后者写，读写隔离互不影响。

注：consumer 的消费 cursor 也持久化存储在 bookkeeper

broker 与 bookies 交互图：

![Brokers and bookies](https://images.yinzige.com/2020-04-20-080232.png)



#### 2.3.6  Ledgers

append-only 的存储结构，每个 ledger 只能有一个 bookie writer，其 entries 在多个 bookie 间复制。语义很简单：

- broker 能 create、append entries、close 掉  ledger

- 不论是显式 close 还是 writer crash，close 后直接变为只读状态。
- ledger 过期后自动从所有 bookie 副本上清除。

##### 保证读一致性

ledger 限制只能由一个 writer 进程追加写，无需达成共识故很高效。故障恢复后，recovery 进程确定 ledger 的最终状态，判定 last commit 的点，在此之前的日志对所有 ledger reader 看到的内容是一致的。

##### Managed Leger

在 ledgers 之上开发出了 managed ledger 提供单一的日志抽象，对应到单个 topic 来表示消息 stream，单个 writer 往 stream 上追加写，多个消费方在 stream 上移动 cursor 读。单个 managed ledger 内部有多个 ledgers：

- failure 之后，旧 ledger 只读，只能开新 ledger 追加写
- 当 ledger 上的 cursor 都消费完毕后会被周期性清理



#### 2.3.7 Journal storage

非易失持久化存储 bookkeeper 的事务日志。在 bookie 操作 ledger 之前，会预先确保描述本次操作的事务日志成功落盘。可配置 `journalMaxSizeMB`  滚动存储。



#### 2.3.8  Pulsar proxy

- 场景：k8s 部署等情况下 client 无法直连 broker 地址

- 原理：全权转发 brokers 的数据。为提高容错性，proxy 数量无上限

- 流程：proxy 从配置的 zk 中拉取 topic 到 broker 的 ownership，可配置具体某个集群、或全局

  ```shell
  > bin/pulsar proxy \
    --zookeeper-servers zk-0,zk-1,zk-2 \
    --configuration-store-servers zk-0,zk-1,zk-2
  ```

- 其他功能：加密、认证



#### 2.3.9 Service discovery

自己实现的 DNS，HTTP 服务发现需要能重定向到目标集群



---

### 3. Pulsar Clients

客户端实现了重连、broker failover 对上层透明，消息入队发送等待 ACK，backoff 重试等等机制。

#### 3.1 设置阶段（Setup）

初始化 producer, consumer 时，客户端会进行 2 步设置：

- 依次尝试向活跃的 broker 发送 HTTP Lookup 请求，broker 会从（缓存）的 zk 元数据中取出 ownership 找出服务该 topic 的 broker，若未找到则会尝试让负载最低的 broker 去处理。

  注：此处 topic 为 partitioned topic，即不同分区被分到不同的单个 broker 处理。

- 找到 topic partition 对应的 broker 后

  - 建立长连接（或直接复用连接池中已有连接）并认证
  - 通过内置二进制协议交换命令
  - client 创建 producer / consumer

当 TCP 连接意外断开后，客户端会直接重新 Setup 并指数 backoff 重试直到成功。



#### 3.2  Reader 接口

##### 3.2.1  Consumer 接口

标准的 `consumer` 接口包括：监听 topics（正则匹配消费），消费消息，处理完毕后回复 ACK。默认新的 subscription 从 latest 开始读，已存在的 subscription 则从最早 unack 的消息开始消费。

总结：实现 `consumer` 接口的消费者，其 cursor 由 ACK 机制 broker 代管，存于 bookie 中。

##### 3.2.2  Reader 接口

可通过 reader 接口自己管理 cursor，使用 reader 读取 topic 时必须指定要从哪条消息开始读，3 种指定：

- earliest、latest
- 精确指定 message ID 读取，此 msg id 可从旧消费记录中加载、

reader 接口能实现 **effectively-once** 语义消费，用在需要 rewind 重新消费的流处理场景中。

注：在 client 内部，reader 被实现为 exclusive、非持久化、名字随机 subscription 的 consumer

 <img src="https://images.yinzige.com/2020-04-20-102637.png" alt="The Pulsar consumer and reader interfaces" style="zoom:40%;" />

##### 3.2.3 DEMO

reader 需自行保存 byteArray 的 `messageId`，无需 acknowledge

```java
void produce() throws PulsarClientException {
    Producer<String> producer = client.newProducer(Schema.STRING)
            .topic(TOPIC_NAME)
            .create();
    for (int i = 0; i < 10; i++) {
        MessageId mid = producer.send("MESSAGE_" + i);
        logger.error("send: {}", "MESSAGE_" + i);
        if (i == 4) {
            this.lastMsgId = mid.toByteArray();
        }
    }
    producer.close();
}

void read() throws IOException {
    Reader<String> reader = client.newReader(Schema.STRING)
            .topic(TOPIC_NAME)
//          .startMessageId(MessageId.earliest) // or MessageId.latest
            .startMessageId(MessageId.fromByteArray(this.lastMsgId))
            .create();
    while (true) {
        Message msg = reader.readNext();
        logger.error("[reader]: " + msg.getKey() + "->" + msg.getValue());
    }
}
```





























