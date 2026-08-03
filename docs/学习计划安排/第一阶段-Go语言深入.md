# 第一阶段：Go 语言深入（7.28 - 8.10，2周）

## 学习目标

从"会用 Go 写代码"提升到"理解 Go 底层原理 + 能手撕并发代码"，覆盖字节跳动 Go 面试全部高频考点。

---

## 模块一：数据结构底层原理（Day 1-2）

### 1.1 Slice 切片

**配套精读：** [Slice、Map 与内存布局：Slice 篇](/第一阶段-知识详解/Slice-Map与内存布局#一、slice-从三字段视图到扩容)

**必须掌握的知识点：**
- 底层数据结构：`array unsafe.Pointer + len int + cap int`
- 扩容策略：
  - Go 1.18 之前：旧容量 < 1024 时翻倍，>= 1024 时按约 1.25 倍增长
  - Go 1.18+：旧容量 < 256 时翻倍，>= 256 时用公式 `newcap += (newcap + 3*threshold) / 4`
- 切片截取 `s[low:high:max]` 三个参数的含义
- append 何时触发拷贝（扩容时底层数组更换）
- nil slice vs empty slice 的区别（`var s []int` vs `s := []int{}`）
- 切片作为函数参数的传值行为（header 拷贝，底层数组共享）

**实战练习：**
```go
// 练习1：预测输出
func main() {
    s := make([]int, 0, 5)
    s = append(s, 1, 2, 3)
    a := s[1:3]
    a = append(a, 4)
    fmt.Println(s) // 预测结果？
    fmt.Println(a) // 预测结果？
}

// 练习2：实现深拷贝函数
func DeepCopy(src []int) []int { /* 实现 */ }

// 练习3：解释为什么这段代码有内存泄漏风险
func getFirst100(data []byte) []byte {
    return data[:100] // 底层数组无法被 GC
}
```

**面试真题：**
- "slice 扩容时原来的元素会被拷贝吗？时间复杂度？"
- "向一个 nil slice append 会 panic 吗？"
- "slice 是值传递还是引用传递？"

---

### 1.2 Map 映射

**配套精读：** [Slice、Map 与内存布局：Map 篇](/第一阶段-知识详解/Slice-Map与内存布局#三、map-经典-hmap-bmap-实现)

> 本项目使用 Go 1.22，以下为该版本的经典 Map 实现。Go 1.24+ 的版本差异见配套精读。

**必须掌握的知识点：**
- 底层结构：`hmap` + `bmap`（bucket 数组）
  - 每个 bucket 存 8 个 kv 对
  - tophash 数组用于快速定位
  - overflow 指针链接溢出桶
- 哈希冲突解决：链地址法（overflow bucket）
- 扩容机制：
  - 翻倍扩容：负载因子 > 6.5 时触发
  - 等量扩容（sameSizeGrow）：overflow bucket 太多时整理碎片
  - 渐进式迁移：后续赋值或删除操作顺带迁移相关 bucket；普通读取兼容新旧布局但不负责迁移
- 遍历无序原因：起始 bucket 随机 + 扩容迁移
- **并发不安全**：并发读写会 `fatal error: concurrent map read and map write`
- 删除操作不会缩容（只标记 tophash 为 emptyOne）

**实战练习：**
```go
// 练习1：验证并发读写 panic
func main() {
    m := make(map[int]int)
    go func() { for { m[1] = 1 } }()
    go func() { for { _ = m[1] } }()
    select {}
}

// 练习2：实现一个并发安全的 map（不用 sync.Map）
type SafeMap struct { /* 实现 */ }

// 练习3：统计一段文本中每个单词出现次数（利用 map）
```

**面试真题：**
- "map 的负载因子是什么？为什么选 6.5？"
- "sync.Map 适合什么场景？和加锁 map 的区别？"
- "map 可以边遍历边删除吗？"

---

### 1.3 Interface 接口

**必须掌握的知识点：**
- 两种底层结构：
  - `eface`（空接口）：`_type + data`
  - `iface`（非空接口）：`tab(itab) + data`
  - `itab` 包含：接口类型 + 具体类型 + 方法表
- nil interface 的陷阱：
  ```go
  var p *int = nil
  var i interface{} = p
  fmt.Println(i == nil) // false! iface 的 type 不为 nil
  ```
- 类型断言 `v, ok := i.(T)` 的底层：比较 itab 中的类型
- 接口的动态派发：通过 itab 方法表间接调用（有性能开销）
- 接口组合与嵌入
- 隐式实现 vs 显式实现（Go 是鸭子类型）

**实战练习：**
```go
// 练习1：预测输出
type MyError struct{ msg string }
func (e *MyError) Error() string { return e.msg }

func GetError() error {
    var err *MyError = nil
    return err // 返回的 error 是 nil 吗？
}

// 练习2：用接口实现策略模式
type Sorter interface { Sort([]int) []int }
// 实现 BubbleSort、QuickSort 两个策略

// 练习3：反射获取接口的动态类型和值
```

**面试真题：**
- "接口的零值是什么？接口持有 nil 指针时等于 nil 吗？"
- "接口调用比直接调用慢多少？为什么？"
- "Go 如何实现多态？"

---

### 1.4 String 字符串

**配套精读：** [String 与字节切片](/第一阶段-知识详解/String与字节切片)

**必须掌握的知识点：**
- 底层结构：`data unsafe.Pointer + len int`（不可变）
- string 与 []byte 的转换语义，以及“零分配”和“零拷贝”的区别
- Map 临时查询 key、字符串比较中的编译器转换优化
- `unsafe.String` / `unsafe.SliceData` 零拷贝的只读与生命周期约束
- `strings.Builder` 为什么高效（避免多次分配）
- rune vs byte：UTF-8 编码下中文占 3 字节
- 字符串拼接方式性能对比：`+` < `fmt.Sprintf` < `strings.Builder` < `bytes.Buffer`

**实战练习：**
```go
// 练习1：不使用额外空间反转字符串（考虑 UTF-8）
func ReverseString(s string) string { /* 实现 */ }

// 练习2：benchmark 对比四种拼接方式
func BenchmarkConcat(b *testing.B) { /* 实现 */ }
```

---

### 1.5 Struct 结构体

**配套精读：** [Slice、Map 与内存布局：内存对齐篇](/第一阶段-知识详解/Slice-Map与内存布局#二、内存对齐-容量和布局背后的约束)

**必须掌握的知识点：**
- 内存对齐规则：
  - 每个字段的偏移量必须是其类型对齐值的整数倍
  - 结构体总大小必须是最大字段对齐值的整数倍
  - 字段顺序影响内存占用（可通过重排减少 padding）
- 方法接收者：值接收者 vs 指针接收者
  - 值接收者：方法内是副本，不影响原值
  - 指针接收者：可修改原值，避免拷贝大结构体
  - 接口实现规则：值接收者实现接口，值和指针都满足；指针接收者实现接口，只有指针满足
- 结构体比较：所有字段可比较时可用 `==`，含 slice/map/func 不可比较
- 空结构体 `struct{}`：大小为 0，用于 set、信号 channel

**实战练习：**
```go
// 练习1：计算以下结构体的内存大小
type A struct {
    a bool   // 1 byte
    b int64  // 8 bytes
    c int32  // 4 bytes
}
// sizeof(A) = ? 重排后最小是多少？

// 练习2：用 struct{} 实现 Set
type Set map[string]struct{}
func (s Set) Add(key string)    { /* 实现 */ }
func (s Set) Has(key string) bool { /* 实现 */ }
```

---

## 模块二：并发编程（Day 3-6）⭐ 最高频考点

### 2.1 Goroutine 原理

**必须掌握的知识点：**
- goroutine vs 线程：
  - 栈空间：goroutine 初始 2KB（可动态增长到 1GB），线程默认 1-8MB
  - 调度：goroutine 是用户态调度，线程是内核态调度
  - 创建成本：goroutine 约 0.3μs，线程约 10μs
- 栈增长与收缩：
  - 连续栈（contiguous stack）：空间不够时分配 2 倍新栈并拷贝
  - 栈收缩：GC 时检查使用量，低于 1/4 时缩小
- goroutine 泄漏场景：
  - 向无人接收的 channel 发送
  - 从无人发送的 channel 接收
  - 死锁
  - 无限循环无退出条件
- 排查工具：`runtime.NumGoroutine()`、pprof

**实战练习：**
```go
// 练习1：写一个会导致 goroutine 泄漏的代码，然后修复它
func leakyFunction() {
    ch := make(chan int)
    go func() {
        val := <-ch // 永远阻塞，goroutine 泄漏
        fmt.Println(val)
    }()
    // 没有人向 ch 发送
}

// 练习2：实现一个 goroutine pool（限制最大并发数）
type Pool struct { /* 实现 */ }
func NewPool(maxWorkers int) *Pool { /* 实现 */ }
func (p *Pool) Submit(task func()) { /* 实现 */ }
func (p *Pool) Wait() { /* 实现 */ }
```

---

### 2.2 Channel 深入

**必须掌握的知识点：**
- 底层结构（`hchan`）：
  - buf：环形缓冲区（有缓冲 channel）
  - sendq / recvq：等待队列（sudog 链表）
  - lock：互斥锁
  - closed：关闭标志
- 发送/接收流程：
  - 发送时有等待接收者：直接拷贝到接收者栈（不经过 buf）
  - 发送时 buf 未满：放入 buf
  - 发送时 buf 已满：挂入 sendq 等待
- 关闭 channel 的规则：
  - 关闭后发送 → panic
  - 关闭后接收 → 返回零值 + false
  - 重复关闭 → panic
  - 关闭 nil channel → panic
- 优雅关闭模式：由发送方关闭，或用 done channel 通知

**实战练习：**
```go
// 练习1：用 channel 实现生产者-消费者（多生产者-多消费者）
func ProducerConsumer(numProducers, numConsumers int) { /* 实现 */ }

// 练习2：实现一个带超时的 channel 读取
func ReadWithTimeout(ch <-chan int, timeout time.Duration) (int, error) { /* 实现 */ }

// 练习3：用 channel 实现信号量（限制并发数为 N）
type Semaphore chan struct{}
func NewSemaphore(n int) Semaphore { /* 实现 */ }
func (s Semaphore) Acquire() { /* 实现 */ }
func (s Semaphore) Release() { /* 实现 */ }

// 练习4：交替打印奇偶数（两个 goroutine 配合）
func PrintOddEven(n int) { /* 实现 */ }

// 练习5：N 个 goroutine 按顺序打印 1~100
func SequentialPrint(n int) { /* 实现 */ }
```

---

### 2.3 sync 包

**必须掌握的知识点：**

#### sync.Mutex
- 两种模式：
  - 正常模式：新来的 goroutine 与被唤醒的竞争锁（新来的有优势，正在 CPU 上运行）
  - 饥饿模式：等待超过 1ms 切换，锁直接交给队头等待者
- 自旋条件：多核 + 自旋次数 < 4 + 至少一个 P 处于 running

#### sync.RWMutex
- 读锁之间不互斥，读写互斥，写写互斥
- 写优先：有写锁等待时，新的读锁也会阻塞（防止写饥饿）

#### sync.WaitGroup
- 底层：计数器 + 信号量
- 注意：`Add()` 必须在 `go func()` 之前调用

#### sync.Once
- 实现原理：atomic + mutex（双重检查）
- 注意：f 里 panic 了也算执行过，不会再执行第二次

#### sync.Pool
- 作用：对象复用，减少 GC 压力
- 生命周期：每次 GC 时会清空 Pool
- 典型使用：`bytes.Buffer`、临时对象

#### sync.Map
- 适用场景：读多写少、key 稳定
- 底层：read map（原子读） + dirty map（加锁）
- 不适合：频繁写入的场景（不如 shardedMap）

**实战练习：**
```go
// 练习1：实现一个简化版 sync.Once
type MyOnce struct { /* 实现 */ }
func (o *MyOnce) Do(f func()) { /* 实现 */ }

// 练习2：实现分片锁 map（提高并发写性能）
type ShardedMap struct { /* 实现 */ }

// 练习3：用 sync.Pool 优化频繁创建 buffer 的场景
func ProcessRequests(requests [][]byte) { /* 使用 Pool 优化 */ }
```

---

### 2.4 Context

**必须掌握的知识点：**
- 设计目的：传递取消信号、超时控制、请求级别值传递
- 四种派生：
  - `context.WithCancel`：手动取消
  - `context.WithTimeout`：超时自动取消
  - `context.WithDeadline`：到达截止时间取消
  - `context.WithValue`：传递请求级数据（trace_id 等）
- 取消传播：父 context 取消，所有子 context 自动取消
- 最佳实践：
  - context 作为函数第一个参数
  - 不要存储在 struct 中
  - WithValue 只传请求级数据，不传业务参数
  - 不要传递 nil context，用 context.TODO()

**实战练习：**
```go
// 练习1：实现一个带超时的 HTTP 请求
func FetchWithTimeout(url string, timeout time.Duration) ([]byte, error) { /* 实现 */ }

// 练习2：实现一个可取消的长时间任务
func LongTask(ctx context.Context) error { /* 实现 */ }

// 练习3：用 context 实现请求链路追踪（传递 trace_id）
func HandleRequest(ctx context.Context) {
    ctx = context.WithValue(ctx, "trace_id", uuid.New().String())
    // 传递到下游调用
}
```

---

### 2.5 并发模式

**必须掌握的知识点：**
- Pipeline 模式：多阶段处理，channel 串联
- Fan-out/Fan-in：多个 goroutine 读同一 channel，多个结果汇聚到一个
- Worker Pool：固定数量 worker 处理任务队列
- Or-Done Channel：任一完成即返回
- Errgroup：并发执行多个任务，任一出错全部取消

**实战练习：**
```go
// 练习1：Pipeline - 数字生成 → 平方 → 打印
func Pipeline() { /* 实现三阶段 pipeline */ }

// 练习2：Fan-out/Fan-in - 多 worker 并发处理 URL 列表
func ConcurrentFetch(urls []string, maxWorkers int) []Result { /* 实现 */ }

// 练习3：用 errgroup 并发调用多个 API，任一失败立即返回
func FetchAll(ctx context.Context, urls []string) ([]Response, error) { /* 实现 */ }
```

---

## 模块三：GMP 调度模型（Day 7-8）⭐ 必考

### 3.1 GMP 三大组件

**必须掌握的知识点：**

#### G（Goroutine）
- 状态：_Gidle → _Grunnable → _Grunning → _Gwaiting → _Gdead
- 包含：栈指针、PC寄存器、绑定的 M、状态标志

#### M（Machine = OS Thread）
- 数量限制：默认最大 10000（`runtime/debug.SetMaxThreads`）
- 包含：当前运行的 G、绑定的 P、系统调用状态

#### P（Processor）
- 数量：`GOMAXPROCS`（默认 = CPU 核数）
- 包含：本地运行队列（最多 256 个 G）、mcache、状态
- P 的意义：控制真正的并行度，管理本地队列减少锁竞争

### 3.2 调度流程

**必须掌握的知识点：**
- 正常调度循环：`schedule() → execute() → gogo() → goexit() → schedule()`
- G 创建：`go func()` → 创建 G → 优先放入当前 P 的本地队列 → 本地满则放全局队列
- 获取 G 的优先级：本地队列 → 全局队列（每 61 次调度检查一次）→ 偷取其他 P 的一半
- Work Stealing：P 本地队列空时，偷取其他 P 队列的一半 G
- Hand Off：M 进入系统调用 → P 与 M 分离 → P 绑定空闲 M（或创建新 M）继续运行
- 抢占式调度：
  - 协作式（Go 1.13-）：函数调用时检查栈标记
  - 信号式（Go 1.14+）：sysmon 发送 SIGURG 信号，asyncPreempt 中断

### 3.3 关键机制

**必须掌握的知识点：**
- sysmon 监控线程：
  - 不绑定 P，独立运行
  - 职责：抢占长时间运行的 G、回收 syscall 阻塞的 P、触发 GC、netpoll
- Network Poller：
  - 将网络 I/O 从阻塞变为非阻塞
  - goroutine 做网络 I/O 时挂起到 netpoller，不占用 M
  - I/O 就绪后放回可运行队列
- 系统调用处理：
  - 非阻塞 syscall：通过 netpoller 处理
  - 阻塞 syscall：M 进入阻塞 → P handoff

**面试高频问答：**
- "为什么要从 GM 模型演进到 GMP？"（减少全局锁竞争 + 提高局部性）
- "P 的数量等于什么？能动态修改吗？"
- "一个 G 运行太久会被抢占吗？怎么实现的？"
- "系统调用会阻塞 M 吗？P 怎么办？"
- "Work Stealing 偷多少？从尾部还是头部偷？"

**实战练习：**
```go
// 练习1：写代码验证 GOMAXPROCS 对并行度的影响
func TestGOMAXPROCS() {
    runtime.GOMAXPROCS(1) // 对比 1 和 runtime.NumCPU()
    // CPU 密集型任务，观察耗时差异
}

// 练习2：写一个会触发抢占的场景
func InfiniteLoop() {
    go func() {
        for { /* 纯计算无函数调用 - Go 1.14 前无法抢占 */ }
    }()
    time.Sleep(time.Second)
    fmt.Println("main done") // Go 1.14+ 能打印
}

// 练习3：用 runtime 包获取调度信息
func PrintSchedulerInfo() {
    fmt.Println("NumGoroutine:", runtime.NumGoroutine())
    fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))
    fmt.Println("NumCPU:", runtime.NumCPU())
}
```

---

## 模块四：内存管理（Day 9-10）

### 4.1 内存分配器

**必须掌握的知识点：**
- 设计灵感：TCMalloc（Thread-Caching Malloc）
- 对象分类：
  - tiny 对象（< 16B，无指针）：tiny allocator，多个对象合并到 16B 块
  - 小对象（16B ~ 32KB）：mcache → mcentral → mheap
  - 大对象（> 32KB）：直接从 mheap 分配
- 层级结构：
  ```
  mcache（每个 P 一个，无锁）
    ↓ 不够时
  mcentral（每个 sizeclass 一个，有锁）
    ↓ 不够时
  mheap（全局一个，有锁）
    ↓ 不够时
  OS（mmap 系统调用）
  ```
- mspan：内存管理基本单元，连续页组成，按 sizeclass 切割
- sizeclass：67 种预定义大小（8B, 16B, 24B, 32B, ...32KB）

### 4.2 逃逸分析

**必须掌握的知识点：**
- 定义：编译器决定变量分配在栈还是堆
- 逃逸到堆的常见场景：
  - 返回局部变量的指针
  - 发送指针到 channel
  - 闭包引用局部变量
  - interface 类型（编译期无法确定大小）
  - slice/map 动态增长
  - 变量大小超过栈帧
- 查看逃逸分析：`go build -gcflags="-m -m" .`
- 优化原则：尽量让变量分配在栈上（栈分配零成本，堆分配需要 GC）

**实战练习：**
```go
// 练习1：分析以下代码的逃逸行为
func escapeExample() *int {
    x := 42
    return &x // x 逃逸到堆
}

func noEscape() int {
    x := 42
    return x // x 在栈上
}

// 练习2：运行 go build -gcflags="-m" 分析你的项目代码
// 找出不必要的逃逸并优化

// 练习3：benchmark 对比栈分配 vs 堆分配的性能差异
```

### 4.3 struct 内存对齐

**必须掌握的知识点：**
- 对齐规则：字段偏移必须是字段类型对齐值的整数倍
- 结构体总大小：最大字段对齐值的整数倍
- 优化手段：按字段大小从大到小排列，减少 padding
- `unsafe.Sizeof`、`unsafe.Alignof`、`unsafe.Offsetof`

---

## 模块五：垃圾回收 GC（Day 11-12）⭐ 必考

### 5.1 三色标记法

**必须掌握的知识点：**
- 三种颜色：
  - 白色：未被访问（GC 结束后回收）
  - 灰色：已被访问但引用未扫描完
  - 黑色：已被访问且引用全部扫描完
- 标记流程：
  1. 所有对象初始为白色
  2. 从 root（全局变量、栈变量、寄存器）出发，标灰
  3. 取出灰色对象，扫描其引用，引用标灰，自身标黑
  4. 重复直到无灰色对象
  5. 回收白色对象

### 5.2 写屏障

**必须掌握的知识点：**
- 为什么需要：并发标记时，用户程序 mutator 可能修改引用关系
- 漏标条件（同时满足）：
  - 黑色对象引用了白色对象（新增引用）
  - 灰色对象到该白色对象的引用被删除（删除引用）
- 三种屏障：
  - 插入写屏障（Dijkstra）：新引用的对象标灰。缺点：栈上不开启，需 STW 重新扫描栈
  - 删除写屏障（Yuasa）：被删除引用的对象标灰。缺点：精度低，本轮白色对象延迟到下轮回收
  - **混合写屏障（Go 1.8+）**：
    - GC 开始时栈上对象全部标黑
    - 堆上同时开启插入和删除写屏障
    - 优势：无需 STW 重扫描栈

### 5.3 GC 流程

**必须掌握的知识点：**
- 四个阶段：
  1. **Mark Setup（STW）**：开启写屏障，所有 P 达成共识
  2. **Marking（并发）**：三色标记，与用户程序并发运行
  3. **Mark Termination（STW）**：关闭写屏障，清理标记状态
  4. **Sweeping（并发）**：回收白色对象，与用户程序并发

- GC 触发条件：
  - 堆增长达到阈值（GOGC，默认 100%，即堆翻倍时触发）
  - 距上次 GC 超过 2 分钟（sysmon 触发）
  - 手动 `runtime.GC()`

- GC 调优：
  - `GOGC=200`：提高阈值，减少 GC 频率（用内存换 CPU）
  - `GOMEMLIMIT`（Go 1.19+）：设置内存软上限
  - `debug.SetGCPercent()`：运行时动态调整
  - 减少堆分配：sync.Pool、栈分配、对象复用

### 5.4 GC 版本演进

| 版本 | 算法 | STW 时长 |
|------|------|----------|
| Go 1.3 | 标记-清除，全程 STW | 百 ms 级 |
| Go 1.5 | 三色标记 + 插入写屏障 | 10ms 级 |
| Go 1.8 | 三色标记 + 混合写屏障 | < 1ms |
| Go 1.19 | + GOMEMLIMIT soft limit | < 0.5ms |

**面试高频问答：**
- "Go 的 GC 算法是什么？为什么不用分代 GC？"
- "什么是写屏障？为什么需要？混合写屏障怎么工作？"
- "GC 的 STW 发生在什么时候？怎么减少 STW？"
- "怎么调优 GC？GOGC 和 GOMEMLIMIT 的区别？"

**实战练习：**
```go
// 练习1：观察 GC 日志
// GODEBUG=gctrace=1 go run main.go

// 练习2：写一个制造 GC 压力的程序，用 pprof 分析
func GCPressure() {
    for i := 0; i < 1000000; i++ {
        _ = make([]byte, 1024) // 大量堆分配
    }
}

// 练习3：用 sync.Pool 优化上面的程序，对比 GC 次数
```

---

## 模块六：工程实践（Day 13-14）

### 6.1 错误处理

**必须掌握的知识点：**
- error 接口：`type error interface { Error() string }`
- 错误包装：`fmt.Errorf("...: %w", err)` + `errors.Is/As`
- panic vs error：panic 用于不可恢复的错误（编程错误），error 用于预期内的错误
- sentinel error：`var ErrNotFound = errors.New("not found")`
- 自定义 error 类型：实现 Error() 和 Unwrap()

**实战练习：**
```go
// 练习1：设计一个业务错误体系
type AppError struct {
    Code    int
    Message string
    Err     error
}
// 实现 Error()、Unwrap()、Is()

// 练习2：实现一个带重试的函数包装器
func WithRetry(fn func() error, maxRetries int, backoff time.Duration) error { /* 实现 */ }
```

### 6.2 泛型（Go 1.18+）

**必须掌握的知识点：**
- 类型参数：`func Max[T constraints.Ordered](a, b T) T`
- 类型约束：`interface { int | float64 | string }`
- 常用约束：`comparable`、`any`、`constraints.Ordered`
- 泛型使用场景：容器类型、通用算法

**实战练习：**
```go
// 练习1：实现泛型 Filter/Map/Reduce
func Filter[T any](slice []T, fn func(T) bool) []T { /* 实现 */ }
func Map[T, U any](slice []T, fn func(T) U) []U { /* 实现 */ }
func Reduce[T, U any](slice []T, init U, fn func(U, T) U) U { /* 实现 */ }

// 练习2：实现泛型 LRU Cache
type LRUCache[K comparable, V any] struct { /* 实现 */ }
```

### 6.3 测试与性能分析

**必须掌握的知识点：**
- 单元测试：`testing.T`、表驱动测试、testify
- 基准测试：`testing.B`、`b.ResetTimer()`、`b.ReportAllocs()`
- pprof：CPU profile、heap profile、goroutine profile
- race detector：`go run -race .`
- 常用命令：
  ```bash
  go test -v -run TestXxx ./...
  go test -bench=. -benchmem ./...
  go test -race ./...
  go tool pprof http://localhost:6060/debug/pprof/heap
  ```

**实战练习：**
```go
// 练习1：为你写的并发安全 map 编写完整的单元测试和基准测试

// 练习2：用 pprof 分析一个有性能问题的程序并优化
// - 找出 CPU 热点
// - 找出内存分配热点
// - 优化后对比

// 练习3：用 race detector 找出以下代码的 data race
func RaceExample() {
    counter := 0
    for i := 0; i < 1000; i++ {
        go func() { counter++ }()
    }
    time.Sleep(time.Second)
    fmt.Println(counter)
}
```

### 6.4 项目结构与 Module

**必须掌握的知识点：**
- Go Module：`go.mod`、`go.sum`、版本选择规则
- 标准项目结构：
  ```
  /cmd          - 入口
  /internal     - 私有代码
  /pkg          - 可对外导出的库
  /api          - API 定义（proto/swagger）
  /configs      - 配置文件
  ```
- 依赖管理：`go get`、`go mod tidy`、`go mod vendor`
- 编译与交叉编译：`GOOS=linux GOARCH=amd64 go build`

### 6.5 常用标准库

**必须掌握的知识点：**
- `net/http`：Server/Client/Handler/Middleware 模式
- `encoding/json`：Marshal/Unmarshal、struct tag、自定义序列化
- `io`：Reader/Writer 接口、组合使用
- `os/exec`：执行外部命令
- `reflect`：基本使用（面试少考，但 Agent 开发会用）

---

## 模块七：手撕代码题（贯穿全程）

### 必会手写题

| 题目 | 考点 | 难度 |
|------|------|------|
| 实现 LRU Cache | map + 双向链表 | ⭐⭐⭐ |
| 并发安全 Map（带阻塞读） | channel + mutex | ⭐⭐⭐ |
| Goroutine Pool | channel + WaitGroup | ⭐⭐⭐ |
| 生产者-消费者 | channel + select | ⭐⭐ |
| 交替打印奇偶数 | channel 同步 | ⭐⭐ |
| Rate Limiter（令牌桶） | ticker + channel | ⭐⭐⭐ |
| 超时控制的 HTTP Client | context + select | ⭐⭐ |
| 简化版 sync.Once | atomic + mutex | ⭐⭐ |
| Pipeline 数据处理 | channel 串联 | ⭐⭐ |
| 并发下载器 | goroutine + errgroup | ⭐⭐⭐ |

---

## 每日学习计划

| 天数 | 主题 | 上午（3h）| 下午（3h）| 晚上（2h）|
|------|------|-----------|-----------|-----------|
| Day 1 | Slice/Map | 学习底层原理 | 写练习代码 | 整理笔记 |
| Day 2 | Interface/String/Struct | 学习底层原理 | 写练习代码 | 面试题模拟 |
| Day 3 | Goroutine/Channel | 学习原理 | 手撕并发题 | 复习 |
| Day 4 | Channel 深入/Select | 学习原理 | 手撕并发题 | 复习 |
| Day 5 | sync 包全家族 | 学习原理 | 实现简化版 | 面试题模拟 |
| Day 6 | Context/并发模式 | 学习原理 | Pipeline/FanOut | 复习 |
| Day 7 | GMP 模型 | 学习 G/M/P | 调度流程 | 面试题模拟 |
| Day 8 | GMP 深入 | Stealing/HandOff | sysmon/Netpoll | 复习 |
| Day 9 | 内存分配器 | 学习层级结构 | 逃逸分析实践 | 复习 |
| Day 10 | 内存对齐 | 学习对齐规则 | 优化练习 | 面试题模拟 |
| Day 11 | GC 三色标记 | 学习标记流程 | 写屏障原理 | 复习 |
| Day 12 | GC 调优 | GC 触发/调优 | pprof 实战 | 面试题模拟 |
| Day 13 | 错误处理/泛型 | 学习 + 练习 | 泛型实战 | 复习 |
| Day 14 | 测试/工程实践 | pprof + race | 项目结构 | 总复习 |

---

## 推荐学习资源

### 书籍
- 《Go 语言设计与实现》（draveness.me/golang）— 底层原理最佳参考
- 《Go 语言高级编程》— 进阶用法
- 《Go 语言学习笔记》雨痕 — 源码分析

### 在线资源
- [Go 语言原本](https://golang.design/under-the-hood/) — 源码级讲解
- [Go 夜读](https://talkgo.org/) — 视频讲解
- [GolangStar 面试题](https://golangstar.cn/) — 分类面试题
- [Go 语言面试宝典](https://golang.design/go-questions/) — 高频问题

### 工具
- `go build -gcflags="-m"` — 逃逸分析
- `go tool pprof` — 性能分析
- `go test -race` — 竞态检测
- `GODEBUG=gctrace=1` — GC 日志
- `go tool trace` — 调度追踪

---

## 阶段验收标准

完成以下全部内容视为通过第一阶段：

- [ ] 能口述 slice/map/interface 底层结构
- [ ] 能口述 GMP 模型完整调度流程
- [ ] 能口述 GC 三色标记 + 混合写屏障
- [ ] 能口述逃逸分析规则和优化方法
- [ ] 独立手写 LRU Cache（15 分钟内）
- [ ] 独立手写 goroutine pool（15 分钟内）
- [ ] 独立手写生产者-消费者（10 分钟内）
- [ ] 独立手写交替打印奇偶数（10 分钟内）
- [ ] 能用 pprof 分析并优化一个性能问题
- [ ] 能用 race detector 找出并修复数据竞争
