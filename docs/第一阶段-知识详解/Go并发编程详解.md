# Go 并发编程详解（Goroutine / Channel / sync / 并发模式）

> 本专题整合自多轮学习对话，按「原理 → 组件 → 实战模式 → 面试要点」组织，覆盖 Go 并发面试核心知识体系。
> 学习顺序建议：先通读「一、Goroutine 原理」建立模型，再学「二、Channel」与「三、sync 包」，最后通过「四、Goroutine Pool 实战」把知识串成生产级组件。

---

## 一、Goroutine 原理

::: tip 配套基础
本节涉及的 goroutine 栈、栈增长、上下文切换，其底层原理（PC/SP/Stack Frame、逃逸分析）见 [Go 函数调用与栈详解](/第一阶段-知识详解/Go函数调用与栈详解)。
:::

### 1.1 goroutine 是什么

goroutine 是 Go runtime 管理的轻量级用户态线程。

| 维度 | goroutine | OS Thread |
| ---- | --------- | --------- |
| 调度 | Go runtime 调度（用户态） | 操作系统调度（内核态） |
| 栈大小 | 初始约 2KB，动态增长 | 通常 MB 级（固定 1~8MB） |
| 创建成本 | 约 0.3μs | 约 10μs（需系统调用） |
| 数量 | 可轻松百万级 | 数量有限（万级就很吃力） |

核心优势：

> 使用少量线程（M）执行大量 goroutine（G），即 M:N 调度模型。

### 1.2 goroutine 栈里存什么

goroutine 栈保存：

- 函数调用帧（stack frame）
- 参数
- 返回地址
- 局部变量

```go
func main() {
    foo()
}

func foo() {
    a := 10
}
```

栈结构：

```
main frame
  |
  └─ foo frame
        a = 10
```

### 1.3 为什么 goroutine 栈会增长

注意：**栈增长 ≠ 所有内存增长**。

增长原因：

- 函数调用深度增加（每层调用产生一个 stack frame）
- 栈上的局部变量增加

例如递归：

```go
func dfs() {
    dfs() // 每层递归都压入一个 frame
}
```

```
dfs()
 |
dfs()
 |
dfs()
 |
...
```

每层函数都有 stack frame，于是栈从 2KB → 4KB → 8KB → 16KB … 不断翻倍增长。

### 1.4 栈增长机制：连续栈（contiguous stack）

Go 1.4+ 使用连续栈。当空间不足时：

1. 编译器在函数入口插入栈检查代码（stack check prologue）
2. 栈空间不够时，申请一块更大的新栈（通常 `new size = old size * 2`）
3. 把旧栈内容拷贝到新栈，更新所有指向旧栈的指针
4. 释放旧栈

```
旧栈:  [========]
        │ 空间不足
        ▼
新栈:  [================]  ← copy(old, new) 后释放旧栈
```

**为什么不用分段栈（segmented stack）？**
分段栈在栈边界处反复扩容/收缩会造成 "hot split" 问题——一个函数调用恰好落在栈边界，每次调用都触发扩容、返回又收缩，性能抖动严重。

### 1.5 栈收缩

一个 goroutine 曾经递归很深（栈 1MB），任务结束后其实只需要 4KB。

GC 时检查：如果栈使用率低于 1/4，就把栈缩小，释放多余内存。

### 1.6 stack 与 heap 的区别

| | 栈（stack） | 堆（heap） |
| --- | --- | --- |
| 生命周期 | 函数调用期间 | 更长，由 GC 管理 |
| 分配成本 | 零成本（函数返回自动释放） | 需要 GC 介入 |
| 典型对象 | 局部变量、调用帧 | 逃逸对象、大对象 |

```go
func test() {
    x := 10 // x 在 goroutine stack 上
}

func test2() []int {
    x := make([]int, 10000)
    return x // x 逃逸到 heap
}
```

**重要区别：** goroutine 栈增长关注的是「函数调用 + 局部变量」，不是「大对象内存」：

- 归并排序的临时数组：通常在 heap
- 快速排序的递归调用：导致 stack 增长

### 1.7 G-M-P 调度模型

Go 调度器三个核心：

```
G = goroutine（执行单元）
M = machine（OS 线程）
P = processor（调度资源）
```

```
              Scheduler
                 |
    G      G      G        ← 等待执行的 goroutine
    |      |      |
    P      P      P        ← 每个 P 拥有 local run queue
    |      |      |
    M      M      M        ← OS 线程
 线程
```

**G（goroutine）：** 包含 stack、状态、调度信息。

**M（machine）：** 真正执行代码的 OS 线程。

**P（processor）：** 调度资源。每个 P 拥有一个 local run queue（本地运行队列），存放等待执行的 goroutine。P 还绑定 mcache 做无锁内存分配。

**P 的三重作用（面试答法）：**
1. 控制真实并行度（`GOMAXPROCS` 个 P = 最多同时运行的 G 数）
2. 提供本地 run queue 减少全局锁竞争
3. 绑定 mcache 做高效内存分配

> 补充：P 是 GM 模型升级到 GMP 的关键——没有 P 就只能用全局队列加锁。
>
> 深入：GMP 的完整调度机制（G 状态机、gopark、Handoff、并发与并行）见 [Go GMP 调度详解](/第一阶段-知识详解/Go GMP 调度详解)。

### 1.8 goroutine 泄漏

定义：goroutine 创建后无法退出，一直占用资源（栈 + 调度），可能引发内存泄漏和性能下降。

**场景 1：无人接收 channel**

```go
func leak() {
    ch := make(chan int)
    go func() {
        ch <- 1 // 没人接收 → 阻塞 → 永久存在
    }()
}
```

流程：发送 → 没人接收 → 阻塞 → 永久存在。

**场景 2：无人发送 channel**

```go
go func() {
    v := <-ch // 没有发送方 → 永久等待
}()
```

**场景 3：无限循环**

```go
go func() {
    for {
        // 没有退出条件
    }
}()
```

> 生产环境还有一种常见泄漏：忘记关闭 `resp.Body`，goroutine 挂在 read 上。

### 1.9 泄漏排查

1. **`runtime.NumGoroutine()`**：观察 goroutine 数量是否只增不减。

```go
fmt.Println(runtime.NumGoroutine())
```

2. **pprof**：查看 `/debug/pprof/goroutine`，定位阻塞点：

```bash
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

堆栈中通常能看到 `chan receive`、`chan send` 字样，即阻塞在 channel 上的 goroutine。

---

## 二、Channel 深入

### 2.1 Channel 本质

Channel 是 runtime 提供的**并发安全队列**，用于 goroutine 之间通信。

```
goroutine A          goroutine B
    │                    ▲
  send                  receive
    │                    │
    └──────▶ channel ────┘
```

### 2.2 hchan 结构

```go
type hchan struct {
    qcount   uint             // 环形缓冲区中的元素数量
    dataqsiz uint             // 环形缓冲区大小（make 的第二个参数）
    buf      unsafe.Pointer   // 环形缓冲区指针
    elemsize uint16           // 元素大小
    closed   uint32           // 是否已关闭
    sendx    uint             // 发送索引（环形队列写位置）
    recvx    uint             // 接收索引（环形队列读位置）
    recvq    waitq            // 等待接收的 goroutine 队列
    sendq    waitq            // 等待发送的 goroutine 队列
    lock     mutex            // 保护 hchan 所有字段
}
```

- **buf**：环形缓冲区。有缓冲 channel（`make(chan int, 3)`）才有，结构为：

```
+---+---+---+
|   |   |   |   ← 环形，sendx 写 / recvx 读
+---+---+---+
```

- **sendq**：等待发送的 goroutine 队列（channel 满了，发送者挂在这里）
- **recvq**：等待接收的 goroutine 队列（channel 空了，接收者挂在这里）
- **lock**：保护 buffer、队列、状态的互斥锁

### 2.3 发送流程（`ch <- value`）

三种情况：

**情况 1：有接收者等待（recvq 非空）**

直接把手里的 value 拷贝到接收者的栈上，**不经过 buffer**，唤醒接收者。这是 Go channel 的优化——少一次拷贝。

```
发送者 ──value──▶ 接收者栈（跳过 buffer）
```

**情况 2：buffer 未满**

value 拷贝进 buf，`sendx++`。

**情况 3：buffer 已满**

当前 goroutine 包装成 sudog 挂入 sendq，然后 `gopark()` 让出 M，阻塞等待被唤醒。

### 2.4 接收流程（`<-ch`）

1. **buffer 有数据**：直接从 buf 取，`recvx++`。
2. **sendq 有等待发送者**：取数据，同时唤醒发送者。
3. **都没有**：当前 goroutine 挂入 recvq，阻塞等待。

### 2.5 Channel 如何保证并发安全

核心是 `hchan.lock`。多个 goroutine 通过这把锁保证对以下状态的修改原子化：

- qcount / sendx / recvx
- buffer
- sendq / recvq
- closed 状态

### 2.6 close 语义（操作总结表）

| 操作 | nil channel | closed channel | 正常 channel |
| ---- | ----------- | -------------- | ------------ |
| 发送 | 永久阻塞 | **panic**: send on closed channel | 正常 / 阻塞 |
| 接收 | 永久阻塞 | 返回零值 + false | 正常 / 阻塞 |
| 关闭 | **panic** | **panic**（重复 close） | 正常 |

close 后接收：

```go
v, ok := <-ch // ok == false，v 为零值
```

### 2.7 谁关闭 channel？

原则：**谁生产数据，谁关闭**。

```
producer ──▶ channel ──▶ consumer
```

producer 才知道"没有数据了"。消费者盲目 close 会导致发送方 panic。多个发送者时，用额外的 done channel 协调关闭时机，不直接 close 数据 channel。

### 2.8 如何判断 channel 是否关闭

#### Go 没有直接的 IsClosed()

```go
IsClosed(ch) // Go 不提供
```

原因：**判断和使用之间存在竞态**。

```go
if !IsClosed(ch) {   // goroutine A：检查未关闭
    ch <- data       // goroutine B 此刻 close(ch) → panic
}
```

> 判断 channel 状态再操作 channel，在并发环境下是不安全的。channel close 是**通知机制**，不是**状态查询机制**。

#### 接收端通过 ok 判断关闭（标准方式）

```go
value, ok := <-ch
if !ok {
    // channel 已关闭
}
```

| 状态 | 结果 |
| --- | --- |
| 有数据 | `value, true` |
| 关闭且无数据 | 零值, `false` |
| 未关闭但无数据 | 阻塞 |

#### for range 自动处理关闭

```go
for v := range ch {
    process(v)
}
```

等价于：

```go
for {
    v, ok := <-ch
    if !ok {
        break
    }
    process(v)
}
```

#### 发送端能不能知道 channel closed？

> **发送端不能可靠判断 channel 是否关闭。**

发送端最关心的是"我现在能不能安全发送？"而不是"channel 当前是不是 closed？"——因为"判断 → 状态变化 → 发送"不是原子操作，即使判断时没关闭，发送瞬间也可能已经被 close。

### 2.9 发送端安全关闭方案：done channel（推荐）

不要用"先判断再发送"，而是用 select 同时等待"发送机会"和"关闭信号"：

```go
select {
case tasks <- task:  // 发送成功
    return nil
case <-done:         // 关闭信号先到
    return ErrClosed
}
```

含义：两个事件（任务可以发送 / 关闭信号到达）**谁先发生执行谁**。

> 单独用 `atomic.Bool` 判断 closed 仍存在竞态（Load false → close(ch) → ch <- task panic），所以要用 select 做原子化的"可发送"判断。

### 2.10 select 原理

#### select 不是 switch

switch 的 case 是**值匹配**：

```go
switch x {
case 1:
}
```

select 的 case 是 **channel 通信操作是否可以完成**：

```go
select {
case <-ch:       // 等待：channel 有数据
case ch <- v:    // 等待：channel 可以发送
}
```

#### select 支持三种通信

1. **接收**：`case value := <-ch:` — 等待 channel 有数据
2. **接收但忽略数据**：`case <-done:` — 常用于退出通知，`close(done)` 后所有监听者都会收到
3. **发送**：`case ch <- value:` — **不是立即执行发送**，而是等待"发送现在可以完成"

#### select 的本质

```
等待多个事件：
  事件1: done 关闭
  事件2: channel 可以发送
  事件3: channel 可以接收

哪个先发生，就执行哪个。
```

### 2.11 select 常见使用场景

**1. goroutine 优雅退出**

```go
select {
case <-ctx.Done():
    return
case task := <-tasks:
    handle(task)
}
```

**2. 超时控制**

```go
select {
case result := <-rpc:
    return result
case <-time.After(time.Second):
    return timeout
}
```

**3. 非阻塞读取**

```go
select {
case data := <-ch:
default: // 无数据立即走 default，不阻塞
}
```

**4. 非阻塞发送（有界队列）**

```go
select {
case queue <- task:
default:
    return ErrFull
}
```

**5. Worker Pool**

```go
for {
    select {
    case task := <-tasks:
        task()
    case <-stop:
        return
    }
}
```

### 2.12 Generator 模式与背压

#### 为什么不能这样写？

```go
// ❌ 错误版本
for i := 0; ; i++ {
    select {
    case <-done:
        return
    default:
        ch <- i // default 执行后 select 已结束，这里是普通发送
    }
}
```

问题：`default` 执行以后 select 已经结束，之后的 `ch <- i` 是普通发送。如果消费者没就绪，它会阻塞；此时即使 done 关闭了也没有机会处理。

> 原则：**任何需要响应取消的阻塞操作，都应该放进 select case 里。** select 提供的是并发事件竞争，而不是 if 判断。

#### 正确版本

```go
func FixedGenerator(done <-chan struct{}) <-chan int {
    ch := make(chan int)
    go func() {
        defer close(ch)
        for i := 0; ; i++ {
            select {
            case <-done:
                return
            case ch <- i: // 等待"可以发送"成立
            }
        }
    }()
    return ch
}
```

#### 为什么 `case ch <- i` 不会疯狂发送？

无缓冲 channel，没有消费者时：

```
select
  │
case ch <- i
  │
等待（不会进入下一轮 i++）
```

只有消费者出现，发送成功后才进入下一轮：

```
producer              consumer
    │                    │
ch <- i  ──────────▶  <-ch
    │
  i++ 进入下一轮
```

#### 背压（backpressure）

Generator 的本质是生产者-消费者模型：

```
生产者 ──▶ channel ──▶ 消费者
```

channel 的阻塞天然提供**背压**：消费者慢，生产者自动减速，不会无界堆积。

#### 本节最重要的理解

1. channel close 是一种**通知机制**，不是状态查询机制
2. 任何需要响应取消的阻塞操作，都应该放进 select case
3. `case ch <- value` 不是执行发送，而是**等待发送条件成立**
4. select 提供的是并发事件竞争，而不是 if 判断

---

## 三、sync 包

### 3.1 Mutex（互斥锁）

作用：保护共享数据。

```go
mu.Lock()
count++
mu.Unlock()
```

底层结构：

```go
type Mutex struct {
    state int32 // 锁状态（低位表示是否被锁定，高位记录等待者数等）
    sema  uint32 // 信号量，用于唤醒等待者
}
```

#### 两种模式

**正常模式（Normal）：**
- 新来的 goroutine 可以和刚被唤醒的 goroutine 竞争锁
- 新来的有优势（已经在 CPU 上运行，cache 热）
- 性能高，但等待队列中的 goroutine 可能被**饥饿**

**饥饿模式（Starvation）：**
- 触发：等待者等待超过 1ms 还没拿到锁
- 行为：锁释放后**直接交给等待队列队首**，新来的不参与竞争，保证公平
- 退出：等待者拿到锁后发现自己已是队列最后一个，或等待时间 < 1ms

#### 自旋（spinning）

短时间等待时不立即休眠，而是自旋重试。

条件：
- 多核机器
- 自旋次数 < 4
- 至少有一个 P running（保证自旋期间有 goroutine 在推进）

目的：避免 park（休眠）和 wake（唤醒）的系统调用开销。

### 3.2 RWMutex（读写锁）

| 操作 | 结果 |
| --- | --- |
| 读 + 读 | 允许（并发） |
| 读 + 写 | 阻塞 |
| 写 + 写 | 阻塞 |

**写优先**：防止写饥饿。如果读不停进入，写永远执行不了。所以**有写等待时，新的读请求会阻塞**，让写者优先获得锁。

### 3.3 WaitGroup

用途：等待多个 goroutine 完成任务。

```go
wg.Add(1)
go func() {
    defer wg.Done()
    // 任务
}()
wg.Wait()
```

底层：**计数器 + 信号量**。

- `Add(n)`：计数 +n
- `Done()`：计数 -1
- `Wait()`：阻塞直到计数归零

**关键理解：WaitGroup 本质是一个计数器，不是"专门等待 goroutine 的机制"。** 它可以计数任何事件（任务、请求、worker……），详见第四章协程池。

注意：**Add 必须在 go 之前调用**。

```go
// ❌ 错误：可能在 Wait 之后才 Add
go func() {
    wg.Add(1)
}()

// ✅ 正确：先 Add 再启动 goroutine
wg.Add(1)
go func() { ... }()
```

否则可能出现 Wait 提前返回。

### 3.4 sync.Once

用途：保证只执行一次（如单例初始化）。

```go
once.Do(func() {
    init()
})
```

底层：`atomic`（标志位）+ `mutex`。

特点：如果 `f` panic，**也算执行过**，不会再次执行。

### 3.5 sync.Pool

#### 本质

不是缓存，而是**临时对象复用池**（"临时对象回收站"）。

- ❌ 错误理解：`Pool = 对象仓库`
- ✅ 正确理解：`Pool = 临时对象回收站`

#### 为什么 GC 会清理？

Pool 里存的是临时对象。如果不清，可能长期占用内存，所以 **GC 会清空 Pool**（每次 GC 都会清）。下一次 `Get` 可能拿到 nil，触发 `New` 重建。

#### 使用场景（三个条件都要满足）

1. **创建频繁**（如 `bytes.Buffer`）
2. **生命周期短**（如单次 HTTP 请求）
3. **可以重置**（如 `buf.Reset()` 后复用）

#### 不适合

- ❌ 做缓存
- ❌ 存数据库连接（连接需要长期保活，会被 GC 清掉）
- ❌ 保存长期对象

#### 使用方式

```go
var pool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

// 使用
buf := pool.Get().(*bytes.Buffer)
buf.Reset()
// ... 使用 buf
pool.Put(buf) // 归还复用
```

GC 清掉后，下一次 `Get` 会重新 `New`。

### 3.6 sync.Map

适合：**读多写少**（如配置缓存、只增不减的注册表）。

底层：`read map`（原子读）+ `dirty map`（写）。

- 读取：优先走 read map，**无需锁**
- 写入：修改 dirty map，**需要锁**，并触发提升（promote）机制

不适合频繁写。频繁写场景更适合 **map + 分片锁**（ShardedMap）：N 个分片，每片独立 mutex，写性能好、实现简单。

| | sync.Map | 分片锁 (ShardedMap) |
| --- | --- | --- |
| 适合场景 | 读多写少、key 基本不变 | 高并发读写均衡 |
| 原理 | read map(atomic) + dirty map(mutex) | N 个分片，每片独立 mutex |
| 优势 | 读路径完全无锁 | 写性能好，实现简单 |
| 劣势 | 写要加锁 + promote 开销 | 需要好的 hash 分散到各 shard |

---

## 四、Goroutine Pool 实战（协程池）

> 这一章是多轮对话的核心成果：从零设计一个固定 worker 数量的 goroutine pool，并逐步演进到生产级设计。

### 4.1 为什么需要协程池

直接为每个任务开 goroutine：

```go
for {
    go task() // 任务百万个 → 百万 goroutine
}
```

问题：造成调度压力 + 内存压力。

核心思想：**固定数量 worker，任务排队执行**。

```
             Submit()
                │
                ▼
          jobs channel
                │
        ─────────────────
        │       │       │
        ▼       ▼       ▼
    worker1 worker2 worker3
```

### 4.2 核心组件

- **jobs channel**：任务队列
- **worker**：循环取任务执行
- **WaitGroup**：等待任务 / worker 完成

```go
for task := range jobs {
    task()
}
```

### 4.3 worker 与 task 的区别（最关键的概念）

**task**：一次性工作。

```
Submit → taskWG.Add(1) → worker 执行 → taskWG.Done() → 结束
```

**worker**：长期执行的 goroutine。

```
创建 → 等待任务 → 执行 task1 → 等待任务 → 执行 task2 → ... → channel 关闭 → 退出
```

一个 worker 可以执行很多 task：

```
worker1: task1 task2 task3 task4 ...
```

所以：**task 完成 ≠ worker 退出**。

### 4.4 channel 在 pool 中的作用

`jobs chan func()` 存放等待执行的任务，多个 worker 共同消费。

```go
for task := range p.jobs { // 本质：task, ok := <-p.jobs; if !ok { break }
    task()
}
```

没有任务时 worker 阻塞在 `<-jobs`，不会退出。只有 `close(p.jobs)` 之后 range 结束、for 退出、goroutine 结束。

发送任务时 channel 会唤醒**某一个**等待的 worker。**一个发送对应一个接收**，不会出现两个 worker 同时拿到同一个任务。

### 4.5 为什么需要两个 WaitGroup

因为要管理两个**不同生命周期**：

- **taskWG**：记录"已提交但未完成的任务数"。Submit 时 `Add(1)`，任务执行完 `Done()`。
- **workerWG**：记录"存活的 worker 数"。创建 worker 时 `Add(1)`，worker 退出时 `Done()`。

```go
type Pool struct {
    jobs     chan func()

    taskWG   sync.WaitGroup // 任务生命周期
    workerWG sync.WaitGroup // worker 生命周期
}
```

### 4.6 close(channel) 是通知机制

`close(jobs)` 发送的是**退出信号**，不是强制杀死 goroutine：

```
close(jobs) → worker 收到关闭 → range 退出 → goroutine 结束
```

所以标准流程是：

```go
close(jobs)     // 1. 通知 worker 退出
workerWG.Wait() // 2. 等待 worker 真正退出
```

### 4.7 panic 处理：recover 的位置

初版代码的问题：

```go
// ❌ 错误：task panic 时 Done 不执行，taskWG.Wait 永久阻塞
for task := range p.jobs {
    task()
    p.taskWG.Done()
}
```

正确做法：**任务级 recover**，让每个任务独立处理 panic。

```go
for task := range p.jobs {
    func() {
        defer p.taskWG.Done()
        defer func() {
            recover() // 任务 panic 在这里被吞掉
        }()
        task()
    }()
}
```

**为什么 recover 不能放 worker 外层？**

```go
// ❌ 错误：panic 后 worker 函数退出，worker 死亡
go func() {
    defer recover()
    for task := range jobs {
        task()
    }
}()
```

```
task panic → recover → worker 函数整体退出 → worker 死亡（workerWG.Done 无法执行）
```

正确（任务级 recover）时，一个任务 panic 只影响该任务，worker 继续处理下一个任务：

```
worker
 │
 ├─ task1: panic → recover（隔离）
 │
 ├─ task2: 继续执行
```

### 4.8 defer 不能写在循环内

```go
// ❌ 错误：defer 作用域是整个 goroutine，不是单次循环
for task := range p.jobs {
    defer p.taskWG.Done()  // 百万任务 → 累积百万个 defer
    defer recover()
    task()
}
```

`defer` 是 goroutine 级的（函数返回时执行），不是循环级的。正确做法是用匿名函数包住每次循环体（见 4.7），让每个任务的 defer 在任务结束时就释放。

### 4.9 Submit 与 Close 的竞争

核心问题：`jobs <- task` 与 `close(jobs)` 不能同时发生，否则：

```
goroutine A: jobs <- task
goroutine B: close(jobs)
→ panic: send on closed channel
```

所以必须保证：

```
Submit：加锁 → 检查 closed → Add 任务 → 发送 → 解锁
Close：加锁 → closed = true → close(jobs) → 解锁 → 等 worker 退出
```

生产级需要状态保护：

```go
type Pool struct {
    jobs     chan func()

    taskWG   sync.WaitGroup
    workerWG sync.WaitGroup

    mu     sync.Mutex
    closed bool
}
```

### 4.10 Wait 与 Close 的职责设计

**模型 A：生产级 Pool（长运行服务）**

生命周期：`RUNNING → Close() → CLOSED → Wait()`。

```go
pool.Submit(task)
pool.Close() // 关闭入口，禁止新任务
pool.Wait()  // 等待所有任务和 worker 退出
```

适合：HTTP Server、后台任务系统。

**模型 B：批处理 Pool（你的设计）**

`Wait()` = 等待任务完成 + 关闭 pool。

```go
for i := 0; i < 10000; i++ {
    pool.Submit(job)
}
pool.Wait() // 任务全完成，pool 已关闭
```

适合：一次性批量任务。

**Wait+Close 模型的正确顺序（关键）**

不能先 `close(jobs)` 再 `taskWG.Wait()`，否则可能：

```
Submit: taskWG.Add(1) → 准备发送
Wait:   close(jobs)
Submit: jobs <- task → panic: send on closed channel
```

正确顺序：

```
1. 禁止新的 Submit（标记 closed）
2. 等待已有任务完成（taskWG.Wait）
3. close(jobs)（通知 worker 退出）
4. 等待 worker 退出（workerWG.Wait）
```

### 4.11 生产级设计：状态机与生命周期

```go
type Pool struct {
    jobs     chan func()

    // 任务生命周期
    taskWG   sync.WaitGroup
    // worker 生命周期
    workerWG sync.WaitGroup

    // 状态保护
    mu     sync.Mutex
    closed bool
}
```

完整生命周期：

```
NewPool：创建 worker
   │
   ▼
worker 等待 jobs（阻塞在 range）
   │
Submit：taskWG.Add(1) → worker 执行 → taskWG.Done()
   │
Wait：等待所有任务完成（taskWG.Wait）
   │
Close：close(jobs) → worker 退出 → workerWG.Done()
   │
workerWG.Wait()：全部退出，pool 关闭
```

**当前实现评价**（演进方向）：

| 部分 | 评价 |
| --- | --- |
| worker 模型 | ✅ 正确 |
| channel 任务队列 | ✅ 正确 |
| WaitGroup 使用 | ✅ 正确 |
| panic 恢复思想 | ✅ 正确 |
| 关闭控制 | ⚠️ 需注意顺序 |
| 状态管理 | ⚠️ 需加锁保护 |
| defer 使用 | ❌ 不能写在循环内 |

**继续提升方向（ants 等生产级协程池的核心设计）：**

1. 明确状态机：`RUNNING → CLOSING → CLOSED`
2. 用 `atomic` 管理状态
3. 支持阻塞 / 非阻塞 Submit
4. 支持 context 取消
5. worker 动态扩缩容

### 4.12 通用思想：一套生命周期管理模型

goroutine pool 的本质不是"创建 goroutine"，而是**管理三个生命周期**：

1. 任务生命周期（Submit → Add → 执行 → Done）
2. worker 生命周期（start → range → close → exit）
3. pool 生命周期（RUNNING → CLOSING → CLOSED）

这套思想不仅适用于 goroutine pool，也适用于：

- HTTP Server 优雅关闭
- 数据库连接池
- Kafka 消费者
- 消息队列 worker
- 定时任务系统
- Agent 调度系统

本质都是：

> 管理一组长期运行的 goroutine，并安全地控制它们的启动、工作和退出。

#### 最终理解重点（五条）

1. **worker 是长期执行者，task 是一次性工作。**
2. **WaitGroup 不是等待 goroutine，而是等待计数归零。**
3. **channel close 是通知机制，不是强制杀死 goroutine。**
4. **任务完成和 worker 退出是两个不同事件。**
5. 生产级并发组件必须考虑：生命周期、状态转换、并发关闭、panic 隔离、资源释放。

---

## 五、面试高频一句话总结

### Goroutine

> goroutine 是 Go runtime 管理的轻量级用户态线程，通过 G-M-P 模型调度，初始栈约 2KB，可以动态增长。

### Channel

> channel 通过 hchan 结构、mutex 和 sendq/recvq 队列实现并发安全的数据传递；close 是通知机制，发送端要安全退出需借助 select + done channel。

### select

> select 的 case 是 channel 通信操作是否可以完成，等待多个事件中先发生者；`case ch <- value` 是等待发送条件成立，天然提供背压。

### Mutex

> Mutex 通过 state 和 semaphore 实现锁竞争控制，并支持正常模式和饥饿模式平衡性能和公平；短等待用自旋，长等待挂队列。

### sync.Pool

> sync.Pool 用于临时对象复用降低 GC 压力，不保证对象存在，GC 可以清理其中对象。

### Goroutine Pool

> 协程池用固定数量 worker 消费任务队列，用双 WaitGroup 分别管理任务与 worker 生命周期，任务级 recover 隔离 panic，Submit/Close 需加锁保护状态避免 send on closed channel。

---

## 六、下一步学习衔接

本章覆盖了 Go 并发三大核心：**生命周期管理（close + done + context）、任务调度（channel + select）、资源回收（close + wg.Wait）**。

下一步建议按顺序学习，把并发能力扩展到真实服务场景：

1. **context 包**：`WithCancel` / `WithTimeout` / `WithValue`，与 done channel 的关系，取消传播
2. **Pipeline 模式**：`generator → worker → output` 阶段式流水线
3. **Fan-in / Fan-out**：多个 worker 消费一个 channel（fan-out），多个 channel 合并（fan-in）
4. **errgroup**：`golang.org/x/sync/errgroup`——并发任务 + 第一个错误取消 + context 传播
5. **实战巩固**：用本专题的协程池思想实现一个带 context 取消、优雅关闭的 worker 服务

这三个模式组合起来，就是生产级 Go 服务（任务队列、RAG pipeline、RPC 服务）的并发骨架。
