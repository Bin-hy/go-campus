# RabbitMQ 架构全景：Connection / Channel / Exchange / Queue / Binding

> 专题第 3 篇 · 上一篇：[Kafka 架构全景](./02-Kafka-架构全景) · 下一篇：[可靠性对照](./04-可靠性对照)
> 目标：搞清 AMQP 的每个对象负责什么、一条消息如何被路由与确认，以及「队列是单点」到底意味着什么。

## 一、全景：从连接到队列

```mermaid
flowchart LR
    P[Producer] -->|TCP| C1["Connection"]
    C1 --> CH1["Channel 1"]
    C1 --> CH2["Channel 2"]
    CH1 -->|basic.publish routingKey| EX["Exchange<br/>direct/fanout/topic/headers"]
    EX -->|Binding| Q1["Queue: order.create"]
    EX -->|Binding| Q2["Queue: audit"]
    Q1 -->|push + prefetch| CO1[Consumer A]
    Q1 --> CO2[Consumer B]
    Q2 --> CO3[Consumer C]
    Q1 -.->|nack / TTL| DLX["Dead Letter Exchange"] --> DLQ["Queue: order.dlq"]
```

和 Kafka 的第一眼区别：**RabbitMQ 在「生产者」和「队列」之间多了一个 Exchange**。生产者从不直接写队列，而是把消息交给交换机，由 Binding 规则决定落到哪些队列。

| 对象 | 一句话 | 关键属性 |
|------|--------|----------|
| **Broker / Node** | 一个 RabbitMQ 进程（Erlang 节点） | 集群由多个节点组成 |
| **vhost** | 虚拟主机，逻辑隔离单元 | 类似命名空间，权限按 vhost 授予 |
| **Connection** | 客户端到 Broker 的 TCP 连接 | 一条连接可复用多个 Channel |
| **Channel** | 连接上的逻辑通道 | 发消息、声明队列、ack 都在 Channel 上；**非线程安全** |
| **Exchange** | 路由器，按规则把消息分发到队列 | 四种类型 + 是否持久化 |
| **Queue** | 消息的存储与派发单元 | durable / exclusive / auto-delete / 类型 |
| **Binding** | Exchange 与 Queue 之间的路由规则 | 可带 routing key 与 arguments |

## 二、Connection 与 Channel：为什么要分两层

**Q：直接一个 TCP 连接发消息不行吗，Channel 是不是多此一举？**

TCP 连接建立成本高（三次握手、TLS 握手、认证），而一个应用往往有几十个线程要发消息。如果每个线程一条连接，连接数爆炸；如果共用一条连接又要自己加锁串行化。

AMQP 的解法是：**一条 TCP 连接上复用多个 Channel**，每个 Channel 有独立的会话状态（未确认消息、事务、confirm 序号），互不干扰。

两个必须知道的坑：

1. **Channel 不是线程安全的**。多个 goroutine/线程共用一个 Channel 会串包（协议帧交错）。正确做法是每线程一个 Channel，或加锁，或用连接池。
2. **Channel 泄漏比连接泄漏更隐蔽**。忘了 close 的 Channel 会一直占着服务端的资源，`channels` 指标会缓慢上涨。

```go
// Go：一条连接，多个 Channel 的典型结构
conn, _ := amqp.Dial("amqp://guest:guest@localhost:5672/")
defer conn.Close()

ch, _ := conn.Channel()   // 每类工作一个 Channel
defer ch.Close()
```

**Q：那 Kafka 有类似 Channel 的东西吗？**

没有。Kafka 客户端是「连接 + 请求复用」，协议本身是无状态的请求-响应（除了长轮询），不存在 Channel 这种会话对象。这也是 AMQP 协议更「重」的原因之一。

## 三、Exchange：四种路由方式

**Q：为什么不让生产者直接写队列？**

因为「消息发给谁」和「消息存在哪」是两个问题。把路由抽出来之后：

- 生产者只声明「发生了什么事件」（`order.created`），不关心谁消费；
- 新增下游只要**加一个队列并绑定**，生产者代码零改动；
- 同一个事件可以同时进入多个队列（广播），也可以按规则只进某个队列。

四种类型：

| 类型 | 路由规则 | 典型用途 |
|------|----------|----------|
| **direct** | routing key **完全相等** | 按事件类型分发：`order.created` → 订单队列 |
| **fanout** | 忽略 routing key，**广播**给所有绑定队列 | 缓存失效通知、配置刷新 |
| **topic** | routing key 按 `.` 分段，支持 `*`（一段）和 `#`（多段） | `order.*.paid`、`log.error.#` |
| **headers** | 按消息 header 的键值匹配（`x-match: all/any`） | 路由维度不是字符串时 |

两个容易忽略的机制：

- **默认交换机 `""`**：routing key 等于队列名时直接投递，适合最简单的「点名投递」。
- **mandatory + basic.return**：消息路由不到任何队列时，默认会被**静默丢弃**。开启 `mandatory` 后 Broker 会退回给生产者，这是排查「消息发了但没人收到」的第一步。
- **Alternate Exchange**：给交换机配置备用交换机，路由失败时转过去，避免静默丢弃。

**Q：一个队列能绑定多个 Exchange 吗？一个 Exchange 能绑多个队列吗？**

都能。绑定是多对多关系。但要清楚 fanout/topic 的「广播」是**在 Broker 内复制消息**：绑 5 个队列，一条消息就存 5 份。下游越多，存储与网络放大越明显——这正是 Kafka 用「多消费组共享一份日志」解决掉的问题。

## 四、Queue：消息真正待的地方

### 4.1 队列的属性

| 属性 | 含义 | 注意 |
|------|------|------|
| `durable` | 队列元数据持久化，重启后还在 | 只保证队列在，不保证消息在 |
| `exclusive` | 仅声明它的连接可用，连接断开自动删除 | 适合临时回调队列 |
| `auto-delete` | 最后一个消费者断开后删除 | 常与 exclusive 搭配 |
| `arguments` | `x-message-ttl`、`x-max-length`、`x-dead-letter-exchange`、`x-max-priority`、`x-queue-type` 等 | 声明后不可随意修改 |

### 4.2 三种队列类型（选型重点）

| 类型 | 存储模型 | 适用 | 限制 |
|------|----------|------|------|
| **classic** | 单节点队列（旧版可镜像，4.0 已移除） | 短生命周期、可重建的任务 | 节点故障时队列不可用 |
| **quorum** | Raft 复制，多数派确认 | 生产默认，要求不丢 | 不支持优先级、exclusive、非持久化 |
| **stream** | 只追加日志，支持多消费者按 offset 读 | 大吞吐、多下游、需要回放 | 不是传统队列语义，消费模型不同 |

**Q：为什么说「队列是单点」？quorum queue 不是有副本吗？**

- classic 队列：整个队列只存在于**一个节点**上，其他节点只知道它在哪。节点挂了，队列就不可用（4.0 之前靠镜像队列，现已移除）。
- quorum 队列：有 Raft 副本，能容忍少数节点故障，但**一个队列对应一个 Raft 组，写入仍要经过该组的 Leader**。所以单队列的吞吐上限还是单节点级别，不能靠「加节点」让同一个队列更快。

扩展方式是**把不同队列分散到不同节点**，而不是让一个队列横向扩展。这与 Kafka「一个分区一个 Leader、分区可分散」的思路不同。

### 4.3 消息在队列里的状态

```text
ready（等待投递） → 推给消费者，进入 unacked（已投递未确认）
                    ├─ basic.ack    → 删除
                    ├─ basic.nack/reject(requeue=true) → 回到 ready（redelivered=true）
                    └─ 消费者断开 / Channel 关闭 → 自动回到 ready
```

**Q：消息在内存还是磁盘？**

两者都可能。持久化消息会写磁盘（消息体可能留在内存供快速投递），非持久化消息只在内存。队列的**索引结构在内存**，所以消息数量极多时内存压力大——这就是「堆积会出问题」的根源（见 06 篇）。

## 五、ack、prefetch 与 confirm：三个不同的确认

初学者最常把这三个混成一个。它们分别在**不同方向、不同环节**：

| 机制 | 方向 | 确认什么 | 不确认什么 |
|------|------|----------|------------|
| **publisher confirm** | 生产者 → Broker | Broker 已接收并完成路由/入队 | 消费者处理成功；classic 队列下也不保证已 fsync |
| **consumer ack** | 消费者 → Broker | 这条消息处理完了，可以删 | 业务一定成功（取决于你什么时候 ack） |
| **prefetch（basic.qos）** | Broker 侧限制 | 每个消费者最多多少条「未确认」 | 不控制发送速率本身，只控制在途量 |

### 5.1 消费 ack 的三种模式

- `autoAck=true`：消息一发出就视为确认。**消费者崩溃 = 消息丢失**，生产环境基本不该用。
- 手动 ack（推荐）：业务处理成功后 `basic.ack`；失败 `basic.nack(requeue=true)` 重投，或 `requeue=false` 进死信。
- `basic.reject`：一次拒绝一条；`nack` 支持 `multiple=true` 批量拒绝。

**Q：重投的消息怎么知道是重复的？**

Broker 会给重投消息打上 `redelivered=true` 标记，但它**不保证只投一次**。真正的幂等要靠业务键去重（见 05 篇）。

### 5.2 prefetch 到底怎么算

假设 `prefetch=10`，消费者正在处理 10 条还没 ack，Broker 就不会再推第 11 条。所以：

- prefetch 太大：消费者积压一批在本地，宕机后全部重投，浪费且放大重复；
- prefetch 太小：网络往返占比高，吞吐上不去；
- 经验：处理快的任务可以大（几十到几百），处理慢且重的任务要小（1~10）。

`basic.qos(global=true)` 是 Channel 级配额，`global=false`（默认）是每个消费者独立配额，别配错。

### 5.3 publisher confirm 的真实语义

开启 confirm 后，Broker 在**消息入队成功后**回 `basic.ack`（持久化到磁盘/多数派是另一回事）：

- classic 队列 + 持久化消息：confirm 可能在 fsync 之前返回，机器掉电仍可能丢；
- quorum 队列：消息写入多数派 Raft 日志后才 confirm，可靠性更高。

所以「开了 confirm 就绝对不丢」是错的，正确说法见 04 篇。

## 六、流控：RabbitMQ 如何防止自己被压垮

RabbitMQ 是**推模型**，必须有限流机制，否则生产者的洪峰会把 Broker 和消费者一起冲垮：

1. **credit 流控**：连接/Channel 级别的信用额度，Broker 按消费能力发放。
2. **内存水位告警**：默认内存使用超过 40%（`vm_memory_high_watermark`）时，**阻塞所有发布连接**（客户端收到 `connection.blocked`），消费不受影响。
3. **磁盘水位告警**：剩余磁盘低于 `disk_free_limit`（默认 50MB）同样阻塞发布。
4. **lazy queue**：尽量把消息放磁盘、内存只留索引，缓解堆积时的内存压力。

**Q：被阻塞时生产者会怎样？**

连接进入 blocked 状态，发布调用挂起（不是报错），恢复后继续。应用层要监控 `connection.blocked` 事件，否则会表现为「接口莫名变慢」。

## 七、集群：元数据复制与队列归属

```text
Node A ── Node B ── Node C
  │ 元数据（队列/交换机/绑定/用户）全量复制
  ├─ queue-a（classic，只存在于 A）
  ├─ queue-b（quorum，Raft 组 Leader 在 B，副本在 A/C）
  └─ queue-c（stream，分区日志）
```

要点：

- **元数据全量复制**到所有节点，节点越多同步成本越高（新版用 Khepri 改善）；
- **客户端可以连任意节点**，如果队列不在该节点，节点会内部转发（会增加一跳延迟）；
- **quorum queue** 用 Raft 保证队列级高可用，是 4.0 之后的生产推荐；
- **跨机房**不要直接组集群（脑裂与延迟问题），用 **Federation / Shovel** 做单向/双向同步。

## 八、顺序与并发：单队列的 FIFO 何时失效

RabbitMQ 保证的是**单队列的出队顺序**，但：

- 一个队列挂多个消费者 → 消息被并发处理，**完成顺序不可控**；
- nack 重投 → 消息回到队尾（或队头，取决于实现），顺序被打乱；
- 优先级队列 → 高优先级插队，本来就不是 FIFO。

要严格顺序，三个办法：

1. **单队列单消费者**：吞吐受限，但最简单；
2. **Single Active Consumer（SAC）**：一个队列同时只有一个活跃消费者，故障时自动切换，兼顾顺序与高可用；
3. **按业务 key 拆队列**：一致性哈希交换机把同一订单的消息路由到同一队列，再单消费者处理。

## 九、常见误解

1. **「Exchange 存消息」**——错，Exchange 只路由，不存储；没绑定队列的消息直接丢（除非 mandatory/alternate）。
2. **「durable 队列 = 消息不丢」**——错，消息还要 `delivery_mode=2`，且要等 confirm。
3. **「autoAck 更简单更快」**——快，但消费者一崩就丢消息。
4. **「加消费者就能无限提升单队列吞吐」**——队列本身是瓶颈，且并发会破坏顺序。
5. **「集群 = 队列高可用」**——classic 队列不在所有节点上；要高可用得用 quorum queue。

## 十、本篇自测

1. 画出 producer → exchange → 两个队列 → 三个消费者的拓扑，标出 prefetch 与 unacked 的位置。
2. direct / fanout / topic 各举一个业务例子，并说明绑定关系怎么配。
3. 为什么 Channel 不是线程安全的？Go 服务里应该怎么组织 Channel？
4. publisher confirm、consumer ack、prefetch 分别防止什么问题？能不能互相替代？
5. 为什么「单队列吞吐」不能靠加节点解决？quorum queue 解决的是可用性还是吞吐？

---

下一篇把两者放到同一张可靠性表上：消息到底在哪一段丢、每段怎么补，以及 `acks=all`、confirm、quorum 各自的真实边界。
