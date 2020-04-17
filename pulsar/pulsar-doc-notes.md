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

### 2.1 关键特性

- 原生支持跨集群间数据复制
- 百万级别的 topic 支持
- 生产消费延迟极低
- 多种订阅模式
- 通过 bookkeeper 保证数据持久化可靠性存储
- 对 functions，connector 的支持，存储分层支持多种存储引擎



### 2.2 Messaging

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

  









