# Go 内存分配详解（mcache / mcentral / mheap / mspan / size class）

> 本专题整理自一次学习对话：围绕「mcache、mcentral、size class 的职能是什么」展开，最终彻底厘清 Go 内存分配器的分层模型。
> 建议配合[第一阶段知识点总览](/第一阶段-知识点详解)中「4.1 内存分配器架构」阅读——本专题是对该节的深入展开。
> 配套基础：栈帧与逃逸分析的底层原理（为什么变量会从栈逃逸到堆）见 [Go 函数调用与栈详解](/第一阶段-知识详解/Go函数调用与栈详解)。

---

## 零、学习的起点：你最初的困惑

> goroutine 请求内存 → 找 mcache 分配 → mcache 直接找 size class 要吗？
> 为什么说"该 size class 的 span 用完"就去找 mcentral？
> mcentral 是管理 size class 的吗？span 是 8B、16B、32B 的固定大小连续内存吗？
> 需要 32B 空间但 mcentral 没有时，是直接找 OS 分配 32B 到 mheap 上吗？mheap 和 span 又是什么关系？

**核心纠结点（也是本文要解决的一句话）：**

> **size class 不是"一块内存"或"一组 span"，而是"对象大小规格"。**
> **span 才是真正承载内存的一大片连续空间，内部被切成很多相同大小的 object slot。**

---

## 一、整体模型：一个仓库系统

把 Go 的内存分配器想象成一个仓库系统：

```mermaid
flowchart TD
    OS[OS 操作系统] -->|"给 Go Runtime 大块内存"| MHEAP[mheap]
    MHEAP -->|"切分成很多 span"| MSPAN[mspan]
    MSPAN -->|"一个 span 切成很多相同大小的 object slot"| OBJ[object slot]
```

例如，一个 mspan 内部：

```
一个 mspan

┌──────────────────────────────────────┐
│ 32B │ 32B │ 32B │ 32B │ 32B │ ...   │
└──────────────────────────────────────┘
```

这个 mspan 服务于 **32B size class**。

> **size class 决定"一个对象 slot 多大"；span 是真正承载这些 slot 的一大片连续内存。**

---

## 二、核心概念逐个拆解

### 2.1 size class = 对象大小规格

size class 是 Go 预先定义的一组**对象大小规格**（可以理解为"货架分类标准"）：

```text
Size Class 1  → 8B
Size Class 2  → 16B
Size Class 3  → 24B
Size Class 4  → 32B
Size Class 5  → 48B
...
```

假设要分配一个 16B 的对象（如两个 int64 的 struct），流程是：

```text
需要分配的对象大小 → 查找对应 size class → 16B size class
```

**注意：size class 是"内存规格编号"，不是一块真实存在的内存。** 它回答的问题是："这个对象应该按多大的格子分配？"

### 2.2 object slot = 真正给对象的小格子

- 32B 的对象 → 拿一个 32B object slot
- 16B 的对象 → 拿一个 16B object slot

### 2.3 mspan = 承载 slot 的连续内存（最容易误解的概念）

**常见误解：** "span 是 8B、16B、32B 的固定大小连续内存"。

**正确理解：** span 的大小是 KB 级别（例如 8KB / 16KB / 若干个 Go page），内部再切成**相同大小**的 object slot：

```text
一个 8KB span
│
├── 32B object
├── 32B object
├── 32B object
├── 32B object
└── ...（8192 / 32 = 256 个 object）
```

类比：

| 类比 | 对应概念 |
| --- | --- |
| 一栋公寓楼 | span |
| 每个房间 | object slot |
| 房间大小规格 | size class |

> **span ≠ object。span 是一大块连续内存，object 是 span 里切出来的小格子。**

### 2.4 mcache = 每个 P 的本地 span 缓存（为什么无锁）

一个 P 一个 mcache，P 只能访问自己的 mcache：

```text
P0 ─── mcache0      P1 ─── mcache1      P2 ─── mcache2
```

一个 mcache 内部缓存了**多个 size class 对应的"当前可用 span"**：

```text
P0.mcache
│
├── 8B size class   → 当前 mspan
├── 16B size class  → 当前 mspan
├── 32B size class  → 当前 mspan
├── 64B size class  → 当前 mspan
└── ...
```

**为什么不需要锁？** 即使 P0 上轮流运行 G1、G2、G3，某一时刻 `M0 + P0` 只能执行一个 G，它们串行访问同一个 mcache，天然无竞争。

> **mcache 是 P 的本地内存分配缓存，目的：高频小对象分配尽量不访问共享结构、避免锁竞争。**

### 2.5 mcentral = 所有 P 共享的 span 中转仓库

当 P 的 mcache 没有可用 span 时，从这里拿。**mcentral 按 size class 管理 span**：

```text
mcentral
│
├── 8B size class
│      ├── span A
│      ├── span B
│      └── span C
│
├── 16B size class
│      ├── span D
│      └── span E
│
├── 32B size class
│      ├── span F
│      ├── span G
│      └── span H
└── ...
```

**关键纠正：** mcentral 管理的不是"8B 内存、16B 内存"，而是"**8B size class 的一批 span、16B size class 的一批 span**"。size class 决定 span 内部 object 的大小。

因为多个 P 的 mcache 都可能来访问，**mcentral 需要锁/同步机制**。

### 2.6 mheap = 整个 Go Runtime 的"大地主"

mheap 是全局堆内存管理器，负责：

- 管理 Runtime 向 OS 获取的大块内存
- 管理 page
- **创建/切分 span**
- 内存不够时向 OS 申请更多内存

```text
OS Memory
┌────────────────────────────────────────────────┐
│                 Go Runtime                     │
│                                                │
│                    mheap                       │
│   ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ │
│   │ span A │ │ span B │ │ span C │ │ span D │ │
│   └────────┘ └────────┘ └────────┘ └────────┘ │
└────────────────────────────────────────────────┘
```

层级关系：

```text
mheap → page → span → object slot
```

---

## 三、一个 32B 对象从头到尾怎么来

### 完整分配流程图

```mermaid
flowchart TD
    G[G 请求 32B] --> CLASS[找到 32B size class]
    CLASS --> MCACHE[P.mcache]
    MCACHE -->|"当前 32B span 有空闲 slot"| DIRECT[直接分配，无锁 ✅]
    MCACHE -->|"当前 span 满了"| MCENTRAL[mcentral]
    MCENTRAL -->|"有 32B class 的 span"| GIVE1[span 交给 mcache]
    MCENTRAL -->|"没有可用 span"| MHEAP[mheap]
    MHEAP -->|"有空闲 page"| NEWSPAN[切页创建新 span → 交给 mcentral]
    MHEAP -->|"没有足够 page"| OS[OS 通过 mmap 申请更多内存]
    OS --> MHEAP
    GIVE1 --> DIRECT
    NEWSPAN --> GIVE1
```

### 逐步走一遍

**第一步：找 mcache**

```text
G → P0.mcache → 32B size class → 当前 span 满了（✓✓✓✓✓✓✓✓）
```

**第二步：找 mcentral**

```text
P0.mcache → mcentral → 寻找 32B class 的可用 span → 找到 span X → 交给 P0.mcache
```

之后 G 就从 span X 里拿一个 32B slot。

**第三步：mcentral 也没有 → 找 mheap**

mheap 检查有没有足够的空闲页：

- 有 → 切出一部分页，组成一个新的 span，初始化为 32B size class，切成多个 32B object slot，交给 mcentral，再给 P0.mcache
- 没有 → 通过 `mmap` 等系统调用向 OS 申请更多内存，mheap 扩容后再切 span

### 一个重要的纠正：绝不会"找 OS 要 32B"

**错误理解：** 需要 32B，mcentral 没有，就直接 `OS.allocate(32B)`。

**真实逻辑：** 32B 的需求会向上传导，最终**一次向 OS 申请一大块内存**，mheap 切出新的 span（含大量 32B slot）供后续长期使用：

```text
OS → mheap（拿大块内存）→ 创建 span（切成大量 32B object）→ mcentral → mcache → G
```

按需向 OS 申请 32B 性能会非常差，分层缓存的意义就是**批量获取、按规格复用**。

---

## 四、六个概念最终区分（速查表）

| 概念 | 本质 | 回答的问题 / 作用 |
| --- | --- | --- |
| **size class** | 对象大小规格（编号） | "这个对象应该按多大的格子分配？" |
| **object slot** | 真正给对象的小格子 | 32B 对象拿一个 32B slot |
| **mspan** | 一大片连续内存，切成相同大小的 slot | 真正承载内存的单元 |
| **mcache** | 每个 P 的本地 span 缓存 | 快速无锁分配 |
| **mcentral** | 多 P 共享的 span 中转仓库（按 size class 管理） | mcache 不够时从这里拿（有锁） |
| **mheap** | 全局堆内存管理器 | 管理 page、创建 span、向 OS 要内存 |

### 一张图记住

```text
                 OS
                 │  提供大块内存
                 ▼
              mheap
                 │  管理 page / span，没有合适 span 时创建
                 ▼
              mcentral
                 │  按 size class 管理 span，mcache 不够时从这里拿
                 ▼
               mcache
                 │  每个 P 一个，缓存每个 size class 当前正在使用的 span
                 ▼
             object slot
                 │
                 ▼
              Go object
```

### 分配流程一句话速记

> G 请求 32B → 找 32B size class → **mcache**（当前 span 有空闲 slot 就直接拿，无锁）→ 当前 span 满了 → **mcentral**（有对应 size class 的 span 就补给 mcache）→ 没有 → **mheap**（切 page 创建新 span）→ 页不够 → **OS**（mmap 申请大块内存）。

---

## 五、面试答题模板

**问：讲讲 Go 的内存分配器？**

> "Go 采用多级缓存分配，核心是减少锁竞争：**mcache 绑定 P 无需加锁处理 99% 的小对象分配**，只有 mcache 耗尽时才需要向 mcentral 加锁获取新 span。
> 分配对象时先按大小找到对应的 size class，从 mcache 缓存的那个 span 里切一个 object slot；span 用完了去 mcentral 按 size class 领一个新 span；mcentral 也没有就找 mheap 切页创建新 span；mheap 页不够才向 OS 用 mmap 申请大块内存。
> size class 是对象大小规格，span 是按规格切好的一整片连续内存，两者一个是'货架分类'，一个是'真正的货'。"

**常见追问：** tiny 对象（<16B 且无指针）走 tiny allocator 合并分配；大对象（>32KB）直接由 mheap 分配，跳过 mcache/mcentral。这两点与本节模型互补，详见总览 4.1。
