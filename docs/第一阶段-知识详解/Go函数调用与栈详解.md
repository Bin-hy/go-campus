# Go 函数调用与栈详解（PC / SP / Stack Frame / 逃逸 / goroutine 栈）

> 本专题整理自一次学习对话，是理解 **goroutine 栈、栈扩容、上下文切换、抢占、逃逸分析** 的基础。
> 一句话概括：一个函数到底是怎么被 CPU 执行起来的？局部变量放在哪里？调用另一个函数时发生了什么？
>
> 配套阅读：[Go 并发编程详解](/第一阶段-知识详解/Go并发编程详解)（Goroutine 原理 / GMP）、[Go 内存分配详解](/第一阶段-知识详解/Go内存分配详解)（逃逸与堆）。

---

## 零、先记住核心因果链

不要死记 PC、SP、BP、morestack、gogo、mcall 这些名词，先建立因果链：

> **函数为什么需要 Stack？** 因为函数调用需要保存参数、局部变量、返回位置、执行状态 → 形成 **Stack Frame**。
>
> **为什么需要 SP？** 因为需要知道当前 Stack Frame 在哪里。
>
> **为什么需要 PC？** 因为需要知道 CPU 当前执行到哪里。
>
> **为什么 Goroutine 能切换？** 因为可以保存 G1 的 PC + SP + Registers，再恢复 G2 的。
>
> **为什么需要逃逸分析？** 因为如果数据的生命周期超过当前 Stack Frame，不能让它随函数返回失效，要放到 Heap。

```
                         CPU
                          │
                          ▼
                         PC（当前执行位置）
                          │
                          ▼
                    当前 Goroutine
                          │
             ┌────────────┴────────────┐
             ▼                         ▼
            SP（当前栈位置）          Stack
                                       │
                                       ▼
                                  Stack Frame
                                       │
                           ┌───────────┴───────────┐
                           ▼                       ▼
                       局部变量                 函数调用
                           │
                           ▼
                     生命周期分析
                           │
                  ┌────────┴────────┐
                  ▼                 ▼
                Stack              Heap
                                    │
                                    ▼
                           Go Memory Allocator
```

---

## 一、CPU 如何执行程序：PC（Program Counter）

CPU 可以理解成一个不停工作的机器：取一条指令 → 执行 → 取下一条 → 执行 ……

```text
地址       指令
1000       LOAD A
1001       LOAD B
1002       ADD
1003       STORE C
1004       RETURN
```

CPU 必须知道"我现在执行到哪里了"，所以有 **PC（程序计数器）**：

- `PC = 1002` 表示 CPU 正在执行地址 1002 的指令（ADD）
- 执行完自动 `PC = 1003`，继续下一条

> **PC = CPU 当前执行位置。**

---

## 二、函数调用到底是什么：CALL 与返回地址

```go
func main() {
    x := add(10, 20)
}
```

假设 main 的机器指令：

```text
1000: 准备参数
1001: 准备参数
1002: CALL add
1003: 把返回值赋给 x   ← 返回地址
1004: ...
```

add 的机器指令从 2000 开始。执行 `CALL add` 时：

- CPU 的 PC 从 1002 跳到 2000（开始执行 add）
- **问题：add 执行完怎么知道回到 main 的哪里？**
- 答案：**保存返回地址**（1003）

```text
main
  │  CALL add
  │  保存：1003
  │  跳转到 add
  ▼
add
  │  RETURN
  │  恢复 PC = 1003
  ▼
回到 main 中调用后的下一步
```

---

## 三、为什么需要 Stack 与 Stack Frame

```go
func main() { a() }
func a()     { b() }
func b()     { c() }
```

执行到 `c()` 时，计算机需要知道完整调用链：c 执行完回 b，b 回 a，a 回 main。所以必须保存**调用链 + 每个函数的局部变量、参数、返回地址、部分执行状态**。

于是出现 Stack（栈）：

```text
main 调用 a             a 调用 b               b 调用 c
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ a 的信息       │   │ b 的信息       │   │ c 的信息       │
├───────────────┤   ├───────────────┤   ├───────────────┤
│ main 的信息    │   │ a 的信息       │   │ b 的信息       │
│               │   ├───────────────┤   ├───────────────┤
└───────────────┘   │ main 的信息    │   │ a 的信息       │
                    └───────────────┘   ├───────────────┤
                                        │ main 的信息    │
                                        └───────────────┘
```

**每个函数占据的一块区域就是 Stack Frame（栈帧）。**

例如：

```go
func add(a, b int) int {
    c := a + b
    return c
}
```

add 的栈帧逻辑上保存：

```text
┌──────────────────┐
│ add Stack Frame   │
├──────────────────┤
│ 返回地址          │
├──────────────────┤
│ 参数 a = 10       │
├──────────────────┤
│ 参数 b = 20       │
├──────────────────┤
│ 局部变量 c = 30   │
└──────────────────┘
```

> ⚠️ 这是**逻辑模型**。真实 Go 编译器 / CPU ABI 不一定严格按这个顺序存（部分参数可能放寄存器）。但模型可以这样记：
> **一个函数执行时，需要一块属于自己的空间保存执行状态，这块空间就是 Stack Frame。**

---

## 四、SP：Stack Pointer（栈指针）

假设 Stack 从高地址向低地址增长：

```text
高地址
┌──────────────────┐
│ main Frame       │
├──────────────────┤
│ a Frame          │
├──────────────────┤
│ b Frame          │
└──────────────────┘
          ↑
          SP
低地址
```

**SP 指向当前栈帧的位置。**

- 调用 `a()`：调整 SP，为 a 腾出栈空间 → SP 指向 a Frame
- 调用 `b()`：再调整 SP → SP 指向 b Frame
- 函数返回：恢复 SP，当前栈帧空间重新可用

**栈分配为什么快？** 因为通常只是移动栈指针：

```text
进入函数：SP -= frameSize
函数返回：SP += frameSize
```

> 核心思想：**栈分配通常只需要移动栈指针**（具体汇编因架构而异）。

---

## 五、完整看一次函数调用

```go
func main() {
    x := add(10, 20)
}
func add(a, b int) int {
    c := a + b
    return c
}
```

**第一步：进入 main**

```text
┌─────────────────────┐
│ main Stack Frame    │
│  x                  │
└─────────────────────┘
```

**第二步：main 调用 add** —— 保存 main 的返回位置，为 add 准备执行环境：

```text
┌─────────────────────┐
│ add Stack Frame     │
│  a = 10             │
│  b = 20             │
│  c                  │
├─────────────────────┤
│ main Stack Frame    │
│  x                  │
└─────────────────────┘
PC → add 的第一条指令
```

**第三步：执行 add** —— `c := a + b` → c = 30。

**第四步：return** —— 返回 30，add Frame 失效：

```text
┌─────────────────────┐
│ main Stack Frame    │
│  x = 30             │
└─────────────────────┘
PC 回到 main 中 add 调用之后的位置
```

---

## 六、为什么 Stack 适合局部变量

```go
func f() {
    x := 10
    y := 20
}
```

x、y 的生命周期完全被 `f()` 包住：

```text
进入 f() → 建立 f 的 Stack Frame → x、y 使用这块空间 → f 返回 → 整个 Frame 直接失效
```

完全不需要逐个 `free(x)`、`free(y)`、GC 介入。

> **栈非常适合生命周期明确、和函数调用绑定的数据。**

---

## 七、逃逸：生命周期超过栈帧

```go
func f() *int {
    x := 10
    return &x // 返回指向栈上变量的指针
}
```

按之前的逻辑 x 应该在 f 的 Stack Frame，但 f 返回后帧消失，调用者 `p := f()` 还需要 `*p`。

编译器在编译时发现：**x 的生命周期 > f() 的生命周期**，不能放在 f 的栈上，于是 x 逃逸到 Heap：

```text
f Stack                      Heap
┌───────────────┐           ┌───────────────┐
│ 临时变量       │           │ x = 10        │
└───────────────┘           └───────────────┘
                                    ↑
p ──────────────────────────────────┘
```

> **逃逸分析的本质：判断一个数据是否能安全地随着当前栈帧一起消失。**
> 逃逸后对象进入堆，由 Go Memory Allocator（mcache/mcentral/mheap）管理——详见 [Go 内存分配详解](/第一阶段-知识详解/Go内存分配详解)。

---

## 八、Go 的特殊点：每个 G 都有自己的 Stack

传统线程模型：每个 OS Thread 一个 Stack。

Go 模型：**每个 goroutine（G）一个 Stack**：

```text
Go Process
G1 └── Stack     G2 └── Stack     G3 └── Stack     ...
```

```go
go func() { a() }()   // G1: Stack 里是 func() → a() 的帧链
go func() { b() }()   // G2: Stack 里是 func() → b() 的帧链
```

不同 goroutine 的局部变量**天然隔离**：

```go
go func() { x := 1 }() // x 属于 G1 Stack
go func() { x := 2 }() // x 属于 G2 Stack，和上面的 x 不是同一个变量
```

---

## 九、GMP 上下文切换：保存 / 恢复 PC + SP + Registers

假设 M 当前执行 G1，G1 阻塞（如 `ch <- 1`）：

```text
G1 状态 = G1 Stack + G1 PC + G1 SP + 寄存器状态
```

Runtime 保存 G1 的执行状态，然后切换到 G2，恢复 G2 的 PC/SP/Stack/Registers。这就是 **Goroutine Context Switch**。

**为什么 goroutine 切换快？** 对比：

```text
OS Thread 切换：Thread A → OS Kernel → 保存 A → 选择 B → 恢复 B → 返回用户态
                （涉及操作系统调度，陷入内核）

goroutine 切换：G1 → Go Runtime → 保存 G1 Context → 选择 G2 → 恢复 G2
                （很多情况下不需要 OS Thread Switch）
```

例如同一个 M 上：

```text
M（同一个 OS Thread）
G1 → G2 → G3
```

底层线程不变，只是 Go Runtime 换了"执行哪个 G"。切换只保存少量寄存器（用户态完成），而 OS 线程切换要陷入内核，所以 goroutine 切换快得多。

---

## 十、为什么 Go Stack 可以增长 + morestack

### 递归把栈撑满

```go
func f(n int) {
    var arr [100]int
    f(n + 1) // 每层递归一个新的 Stack Frame
}
```

```text
G Stack
┌───────────────┐
│ f(1)          │
├───────────────┤
│ f(2)          │
├───────────────┤
│ f(3)          │
├───────────────┤
│ f(4)          │
└───────────────┘
```

空间不够时，Runtime 检测到 → **扩容**：申请更大的 Stack → 复制旧 Stack → 调整相关指针 → 继续执行。

这正是 goroutine 初始栈可以很小（约 2KB）的原因——按需增长，所以百万级 goroutine 在栈空间角度可行：

```go
for i := 0; i < 1_000_000; i++ {
    go task()
}
```

### morestack：扩容是怎么被发现的

每个函数编译后会知道"自己大概需要多少栈空间"。**进入函数前检查当前 G Stack 剩余空间够不够**：

```text
当前 G Stack 剩余空间不足
      │
      ▼
morestack
      │
      ▼
申请更大的 Stack → 复制旧 Stack → 调整相关指针 → 继续执行函数
```

这也解释了协作式抢占里的"函数入口 + morestack"：函数入口本来就要检查栈空间（安全点），Go 早期利用这类安全点机制做调度和抢占。

---

## 十一、把 PC / SP / Stack 和 Goroutine 串起来

一个正在执行的 G：

```text
G
┌──────────────────────────┐
│ G Metadata               │
│  当前状态                 │
│  PC（从哪继续执行）        │
│  SP（当前栈在哪）          │
│  Registers Context       │
│  Stack Pointer ────────┐ │
└────────────────────────┼─┘
                         ▼
                    Goroutine Stack
              ┌──────────────────┐
              │ 当前函数 Frame    │
              ├──────────────────┤
              │ 上层函数 Frame    │
              ├──────────────────┤
              │ ...              │
              └──────────────────┘
```

G1 被暂停时 Runtime 保存 PC、SP、执行上下文；恢复时：

- PC → 从哪里继续执行
- SP → 当前栈在哪里

于是 G1 可以从原来的位置继续运行。

**更准确地说：**

> **G 是一个可被 Go Runtime 调度的执行单元，它拥有自己的执行上下文和自己的栈。**

```text
G   = Status + Stack + PC + SP + 当前执行函数 + 其他调度信息
P   = Local Run Queue + mcache（负责调度哪些 G）
M   = 真正执行 G 的 OS Thread（被 CPU 执行）

G → 运行在 M → M 是 OS Thread → OS Thread 被 CPU 执行
```

---

## 十二、完整过程演示

```go
func main() {
    go worker()
    x := add(10, 20)
    fmt.Println(x)
}
func worker() { fmt.Println("hello") }
func add(a, b int) int { c := a + b; return c }
```

**① main goroutine 正在运行**：`M0 + P0` 执行 G0，G0 Stack 里是 main()。

**② 创建 worker goroutine**：`go worker()` → Runtime 创建 G1 → 分配 G1 Stack → G1 放入 P0 的 Local Run Queue。

**③ G0 调用 add**：

```text
G0 Stack
┌───────────────────┐
│ add()             │  a=10, b=20, c=30
├───────────────────┤
│ main()            │
└───────────────────┘
执行完 add Frame 失效 → main Frame 里 x = 30
```

**④ G0 让出 CPU**（时间片 / 阻塞 / 抢占）：Runtime 保存 G0 的 PC + SP + Registers → `schedule()` → `findRunnable()` → 找到 G1。

**⑤ M0 切换到 G1**：恢复 G1 的 PC + SP + Stack，执行 worker()：

```text
G1 Stack
┌───────────────────┐
│ worker()          │
└───────────────────┘
```

---

## 十三、面试答题要点

**问：为什么 goroutine 比线程轻量？**

> "三个层面：① 栈——goroutine 初始栈约 2KB 且按需增长（morestack 检测扩容），OS 线程固定 MB 级；② 切换——goroutine 切换由 Go Runtime 在用户态完成，只保存 PC/SP/寄存器，不陷入内核；③ 创建成本——goroutine 创建约 0.3μs，线程约 10μs。归根结底，G 是一个拥有自己栈和执行上下文的调度单元，可以跑在任意 M 上。"

**问：为什么栈上分配比堆上快？**

> "栈分配通常只是移动 SP 指针（SP -= frameSize），函数返回 SP 恢复即可，不需要 GC 介入。但前提是数据生命周期不逃出函数——这就是逃逸分析判断的事情：生命周期超过栈帧就放堆。"

---

## 十四、下一步学习衔接

下一步最适合继续深入的：**Go 函数调用如何与 goroutine 栈结合** —— 包括：

- `g0`（每个 M 的系统栈 goroutine，调度时切换用）
- `g.stack`（G 的栈描述：lo/hi 边界）
- `g.sched`（G 保存的调度上下文：PC/SP/BP 等）
- `gogo()`（恢复 G 执行）
- `mcall()`（切换到 g0 执行调度逻辑）

学完这一层，之前看到的 GMP 调度源码流程（`schedule` → `execute` → `gogo` → 抢占）就会真正看懂。
