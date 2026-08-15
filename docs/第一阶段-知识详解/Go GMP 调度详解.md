# Go GMP 调度详解（GMP 关系 / G 状态机 / Park / Handoff / 并发与并行）

> 本专题整理自一次学习对话，是 GMP 调度的深入讲解，建立在以下两个专题之上：
> - [Go 函数调用与栈详解](/第一阶段-知识详解/Go函数调用与栈详解)：PC/SP/Stack Frame/逃逸/上下文切换的底层原理
> - [Go 并发编程详解](/第一阶段-知识详解/Go并发编程详解)：Goroutine 原理与 G-M-P 模型概览
>
> 学完本专题，即可进入 **GMP 调度循环源码**（`schedule()`、`findRunnable()`、`gopark()`、`goready()`、抢占式调度、sysmon、syscall 处理流程）。

---

## 零、整体知识框架

先建立完整链路：

```text
Go 源代码
    ↓
编译阶段
    ↓
逃逸分析
    ↓
决定变量放在 Stack 还是 Heap
    ↓
运行阶段
    ↓
GMP 调度
    ↓
M + P 执行 G
    ↓
函数调用产生 / 切换 Stack Frame
    ↓
遇到阻塞、主动让出或抢占
    ↓
保存 G 的上下文
    ↓
调度其他 G
```

核心涉及 10 个概念：

```text
1. Stack 和 Heap        2. Goroutine Stack
3. Stack Frame          4. SP / PC
5. G / M / P            6. Goroutine 创建与调度
7. 单 P 下的并发        8. Park / gopark
9. Handoff              10. 系统调用与 Runtime 调度
```

---

## 一、逃逸分析与 Stack / Heap（回顾）

Go 在**编译阶段**进行逃逸分析。核心问题：

> 一个变量在当前函数返回后，是否还可能被访问？

- 编译器能证明 **变量生命周期 ≤ 当前函数生命周期** → 通常放在当前 Goroutine 的 Stack 上
- 不能保证（如 `return &x`）→ 放到 Heap

```go
func f() *int {
    x := 10
    return &x // x 函数返回后仍被调用者持有 → 逃逸到 Heap
}
```

**Stack 上的变量：** 函数返回 → Stack Frame 弹出 → 变量自动失效，不需要 GC。

**Heap 上的变量：** 函数返回后仍存在，最终由 **Go GC** 判断何时回收。

```text
Stack:  f Stack Frame（x? ✘ 已逃逸）
Heap:   ┌─────────┐
        │ x = 10  │ ← 返回的 *int 指向这里
        └─────────┘
```

---

## 二、Goroutine Stack 与 Stack Frame（回顾）

### 每个 Goroutine 都有自己的 Stack

```text
G0 Stack
┌─────────────────┐
│ add Frame       │ ← 当前
├─────────────────┤
│ main Frame      │
└─────────────────┘
```

- 函数调用 = 不断创建新的 Stack Frame
- 函数返回 = 当前 Frame 消失，回到上层 Frame

### Stack Frame 里有什么

简化理解：

```text
┌──────────────────────┐
│ 局部变量              │
├──────────────────────┤
│ 参数                  │
├──────────────────────┤
│ 返回地址              │
├──────────────────────┤
│ 保存的寄存器状态       │
└──────────────────────┘
```

```go
func add(a, b int) int {
    c := a + b
    return c
}
```

```text
add Stack Frame
┌────────────┐
│ a = 10     │
│ b = 20     │
│ c = 30     │
│ 返回地址    │
└────────────┘
```

> ⚠️ 实际编译器实现更复杂：参数和局部变量不一定全部真实存在于栈上，有些可能直接放在寄存器中。

### SP 与 PC

- **SP（Stack Pointer）**：当前 Goroutine Stack 中当前执行位置相关的栈指针。函数调用压帧、返回回退：
  ```text
  Frame 1
  Frame 2
  Frame 3 ← 当前
            ↑
            SP
  ```
- **PC（Program Counter）**：CPU 正在执行的指令位置。Goroutine 切换 = 保存 PC/SP/寄存器 → 恢复 PC/SP/寄存器 → 从之前的位置继续执行。

### 代码中的逃逸与 Goroutine

```go
func main() {
    go worker()
    x := add(10, 20)
    fmt.Println(x)
}
func worker() {
    fmt.Println("hello")
    t := callObj{} // 对象创建 ≠ 一定在 Heap，由逃逸分析决定
}
func add(a, b int) int { c := a + b; return c }
```

若全部未逃逸：

```text
Main G Stack                Worker G Stack
┌──────────────────┐       ┌──────────────────┐
│ add()            │       │ worker()         │
│ a  b  c          │       │ t                │
├──────────────────┤       └──────────────────┘
│ main()           │
│ x                │
└──────────────────┘
```

> 注意：`t := callObj{}` 是对象创建，**不等于一定在 Heap**，是否在 Heap 由逃逸分析决定。

---

## 三、GMP 核心关系

```text
G = Goroutine（任务）
M = Machine（OS Thread）
P = Processor（逻辑处理器 / 执行资源）
```

实际运行：**M 绑定 P，P 执行 G**：

```text
M → 绑定 P → 执行 G     即：M + P → Running G
```

---

## 四、Goroutine 创建后不会自动分配到某个 P

**常见误区：**

```text
Main G → P0
Worker G → P1
```

**正确理解：** `go worker()` 只是创建 Worker G 并放入调度队列：

```text
P0
Running:     Main G
Local Run Queue: [Worker G]
```

```text
Worker G
    ↓
_Grunnable（可运行状态）
    ↓
进入调度队列
    ↓
什么时候执行，由 Runtime 调度决定
```

> **Worker G 创建成功，不代表立即执行。**

---

## 五、并发与并行（GOMAXPROCS）

**并发（concurrency）：** 一个 P，同一时刻只能执行一个 G，但多个 G 可以交替推进：

```text
P0           时间 ──────────────────▶
Main G        ───────      ───────
Worker G             ──────
```

**并行（parallelism）：** `GOMAXPROCS = 2` 时，两个 M 各绑定一个 P，若运行在不同 CPU Core 上，同一时刻真正同时执行：

```text
CPU Core 0: Main G
CPU Core 1: Worker G
```

> **GOMAXPROCS 大致控制同一时刻最多有多少个 Goroutine 可以真正并行执行 Go 用户代码。**

---

## 六、为什么你的 Worker 可能不执行？

```go
func main() {
    go worker()
    x := add(10, 20)
    fmt.Println(x)
}
```

可能发生：创建 Worker G（进入 Runnable）→ Main G 继续执行 → add() → fmt.Println() → **main() 返回 → 整个进程退出** → Worker G 还没获得执行机会。

> `go worker()` 不代表立即执行 worker，而是"创建 Worker G → 交给 Go Runtime 调度 → 等待执行机会"。

**如何让 Worker 执行？** 核心不是 Main 把 M 交给 Worker，而是 **Main G 停止运行，进入 Runtime 调度器，选择下一个 Runnable G**：

```go
done := make(chan struct{})
go func() {
    worker()
    close(done)
}()
<-done // done 未关闭时 Main G 阻塞
```

```text
Main G 执行 <-done
    ↓
done 未关闭，不能继续
    ↓
Main G 进入 Waiting → gopark()
    ↓
schedule()
    ↓
Worker G 被调度执行
```

---

## 七、Park（gopark）机制

**Park 的定义：**

> 当前 G 暂时不能继续执行，于是让当前 G 停止运行，把执行权交回给 Go Runtime。

典型流程：

```text
G Running
    ↓
遇到等待条件
    ↓
gopark()
    ↓
_Gwaiting（等待状态）
    ↓
schedule()
    ↓
寻找 Runnable G，执行其他 G
```

**关键：G 被 Park 不意味着 M 被阻塞、P 被阻塞。**

```text
Main G Waiting
但 M0 + P0 继续执行 Worker G
```

### 哪些情况会 Park？（8 种）

| # | 场景 | 说明 |
| --- | --- | --- |
| 1 | Channel 读阻塞 `<-ch` | 无数据 → G Waiting → Park |
| 2 | Channel 写阻塞 `ch <- x` | 无缓冲无接收者 / buffer 已满 → Park |
| 3 | Mutex 获取失败 `mu.Lock()` | 先短暂自旋，仍失败 → Park，等待 Unlock |
| 4 | WaitGroup.Wait() | 计数器未归零 → Park，等待 Done() |
| 5 | Cond.Wait() | 释放 Mutex → Park → 等待 Signal/Broadcast → 唤醒 → 重新获取 Mutex |
| 6 | select 无 case 可执行 | 挂到相关 Channel 的等待队列 → Park |
| 7 | time.Sleep() | 注册 Timer → Park → Timer 到期 → 变 Runnable（所以 Sleep 时其他 G 可执行） |
| 8 | 网络 I/O 等待 `conn.Read(buf)` | 注册到 Netpoll → Park；数据到达 → 唤醒 G（_Gwaiting → _Grunnable） |

---

## 八、runtime.Gosched()：主动让出

```go
runtime.Gosched()
```

与普通阻塞（Waiting）不同：

```text
当前 G：Running → Runnable（主动让出执行权）→ schedule()
```

> Gosched 不是"等待某个资源"，而是"我现在可以继续执行，但主动让调度器先运行其他 G"。

---

## 九、Park 之后一定立即运行其他 G 吗？

**不一定。**

如果所有 G 都 Waiting（G0/G1/G2 全 Waiting），没有任何 Runnable G：

```text
schedule()
    ↓
找不到可运行 G
    ↓
M 进入休眠，等待事件：Timer / Network / Channel / Syscall 返回 / 其他
```

但只要有一个 Runnable G（如 Main 变 Waiting、Worker 仍 Runnable）：

```text
Main G Park → schedule() → Worker G Running
```

---

## 十、Handoff：P 的转交

Handoff 和 Park 不一样。核心问题：

> **M 被 OS 阻塞了，但 P 很宝贵，不能跟着 M 一起浪费。**

最典型场景：**阻塞系统调用（blocking syscall）**。

```text
M0 + P0 + G1
    ↓
G1 执行阻塞 syscall
    ↓
M0 被 OS 卡住
```

如果 P0 还一直绑定 M0，P0 就浪费了。于是：

```text
M0（阻塞在 syscall）+ G1     ← M0 等 syscall 返回

M1 + P0（P0 转交给 M1）→ 执行 G2

原来:  M0 + P0 + G1 → syscall
之后:  M0 + G1（阻塞）        M1 + P0 → G2
```

这就是 **P 的 Handoff**。

### 哪些情况通常不需要 Handoff？

```go
<-ch // Channel 阻塞
```

发生的是 G Park，M 没有被 OS 阻塞，P 也没有被占死：

```text
M0 + P0 → schedule() → 执行 G2
```

不需要换 M。以下场景都是 **G Waiting** 而非 **M 被 OS 卡死**，通常不需要 Handoff：

```text
Mutex / WaitGroup / Cond / Channel / Timer / 通常的 Network I/O
```

---

## 十一、Park vs Handoff 对比表

| 场景 | G | M | P | Handoff |
| --- | --- | --- | --- | --- |
| Channel 阻塞 | Waiting | 可继续调度 | 可继续使用 | 否 |
| Mutex 等待 | Waiting | 可继续调度 | 可继续使用 | 否 |
| WaitGroup | Waiting | 可继续调度 | 可继续使用 | 否 |
| Sleep | Waiting | 可继续调度 | 可继续使用 | 否 |
| 网络 I/O | Waiting | 可继续调度 | 可继续使用 | 通常否 |
| **阻塞 syscall** | Syscall | **被 OS 阻塞** | 需要转交 | **可能发生** |

**最核心的判断方式（两个问题）：**

1. **只是 G 在等待吗？** → G Park，M + P 继续调度其他 G（如 `<-ch`）
2. **M 被 OS 卡住了吗？** → M 无法继续执行 Runtime，P 不能跟着浪费 → Handoff → 其他 M 接管 P

---

## 十二、G 生命周期状态机（完整模型）

```text
创建 G
    ↓
Runnable（可运行）
    ↓
被 M + P 调度
    ↓
Running（运行中）
    │
    ├── 正常执行
    │
    ├── 阻塞资源
    │      ↓ gopark
    │    Waiting
    │      ↓ 资源就绪
    │    Runnable
    │
    ├── 主动让出（Gosched）
    │      ↓
    │    Runnable
    │
    ├── 被抢占
    │      ↓
    │    Runnable
    │
    ├── 进入阻塞 syscall
    │      ↓
    │    Syscall
    │      ↓ M 可能被阻塞
    │    P Handoff（转交其他 M）
    │
    └── 执行完成
           ↓
         Dead
```

---

## 十三、当前最核心的认知（四句话）

### 1

> **G 是执行任务，M 是 OS 线程，P 是执行 Go 代码所需要的逻辑资源。**

```text
M + P → Running G
```

### 2

> **一个 P 同一时刻只能运行一个 G，但多个 G 可以通过上下文切换实现并发。**

### 3

> **Park 是当前 G 停止运行，通常进入 Waiting，然后 M + P 可以去执行其他 Runnable G。**

```text
G1 Park → schedule() → G2 Running
```

### 4

> **Handoff 是 M 被 OS 阻塞时，把 P 转交给其他 M，避免 P 跟着阻塞。**

```text
G1 → blocking syscall → M0 被阻塞 → P0 不应浪费 → M1 接管 P0 → 执行 G2
```

---

## 十四、面试答题要点

**问：goroutine 调度中 Park 和 Handoff 的区别？**

> "Park 是 G 层面的：当前 G 暂时无法继续（等 channel、锁、timer 等），调用 gopark 进入 _Gwaiting，把执行权交回调度器，M 和 P 不受影响，继续调度其他 G。Handoff 是 P 层面的：G 发起阻塞 syscall 导致 M 被 OS 卡住时，把 P 从当前 M 解绑转交给其他 M，避免 P 空转浪费。判断标准：只是 G 在等待 → Park；M 被 OS 卡住 → Handoff。"

**问：为什么 `go func()` 创建后不一定会执行？**

> "`go` 只是创建 G 并放入调度队列（_Grunnable），执行权还在当前 G 手里。如果 main 很快返回，进程直接退出，新 G 可能从未被调度。要让新 G 执行，需要当前 G 让出执行权（阻塞、Gosched 或等待 channel）。"

---

## 十五、下一步学习衔接

本专题建立的模型，是继续学习 **GMP 调度循环源码** 的基础：

- `schedule()`（调度主循环）
- `findRunnable()`（找可运行 G：本地队列 → 全局队列 → 窃取）
- `gopark()` / `goready()`（Park 与唤醒）
- 抢占式调度（sysmon 监控、异步抢占）
- syscall 处理流程（entersyscall / exitsyscall / Handoff 落地）
