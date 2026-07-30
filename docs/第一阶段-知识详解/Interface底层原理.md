# 第一阶段知识详解：Interface 底层原理

> 本文从 interface 的运行时表示出发，把 eface、iface、itab、方法表、nil 陷阱串成一条完整的理解路径。重点回答"底层到底长什么样"，而不只记结论。

::: info 版本范围
本文基于 **Go 1.22** 的 runtime 源码结构。核心数据结构（`eface`、`iface`、`itab`）自 Go 1.0 以来一直稳定，面试中可安全使用。
:::

## 阅读路线

1. 先理解 interface 的本质：运行时的 `(动态类型, 动态值)`。
2. 再理解空接口（eface）和非空接口（iface）的内部差异。
3. 最后学习 itab 的缓存机制、nil 陷阱、以及与 reflect 的关系。

关联内容：

- [第一阶段知识点总览](/第一阶段-知识点详解)
- [Slice、Map 与内存布局](/第一阶段-知识详解/Slice-Map与内存布局)
- [代码练习指南](/练习指南)

---

## 一、Interface 的本质

Go 的 interface 本质是运行时的一个二元组：

```text
(动态类型, 动态值)
```

运行时必须知道两件事：当前里面存的是**什么类型**，以及当前里面的**数据是什么**。

根据接口是否包含方法，Go 在 runtime 中分成两种实现：

| 接口形式 | runtime 结构 | 特点 |
|---------|-------------|------|
| `interface{}` (any) | `eface` | 无方法，只需记录类型 |
| 带方法的接口 | `iface` | 需要方法表来支持动态调度 |

---

## 二、eface（空接口）

### 结构定义

```go
type eface struct {
    _type *_type          // 动态类型信息
    data  unsafe.Pointer  // 指向真实数据
}
```

### 内存布局

```text
eface
+----------------+
| _type          | ---> _type{size, kind, hash, gc信息, reflect信息}
+----------------+
| data           | ---> 实际数据（如 int(100)）
+----------------+
```

### 为什么空接口不需要 itab？

`interface{}` 没有任何方法要求，它只需要知道"当前数据是什么类型"。`_type` 指针已经足够回答这个问题，不需要额外的方法表。

---

## 三、iface（非空接口）

### 结构定义

```go
type iface struct {
    tab  *itab            // 接口类型 + 具体类型 + 方法表
    data unsafe.Pointer   // 指向真实数据
}
```

### 内存布局

```text
iface
+----------------+
| tab            | ---> itab{inter, _type, hash, fun[]}
+----------------+
| data           | ---> 实际数据
+----------------+
```

---

## 四、itab：连接接口与具体类型的桥梁

### 结构定义

```go
type itab struct {
    inter *interfacetype  // 接口类型，保存接口的方法集合
    _type *_type          // 具体动态类型
    hash  uint32          // 具体类型的 hash，来源于 _type.hash，用于 itab 缓存快速查找
    fun   [1]uintptr      // 方法表（变长数组），保存具体类型实现接口方法的函数地址
}
```

### fun 方法表

以如下代码为例：

```go
type ReadCloser interface {
    Read()
    Close()
}

type File struct{}
func (File) Read()  {}
func (File) Close() {}
```

生成的 itab：

```text
itab (ReadCloser + File)
+----------------+
| inter          | ---> ReadCloser 的方法集合 {Read, Close}
+----------------+
| _type          | ---> File 的类型信息
+----------------+
| hash           | ---> File._type.hash
+----------------+
| fun[0]         | ---> File.Read 的函数地址
| fun[1]         | ---> File.Close 的函数地址
+----------------+
```

调用 `r.Read()` 实际执行的是：

```text
itab.fun[0](data)
```

类似 C++ 的虚函数表（vtable），但 Go 的方法表是按 `(接口类型, 具体类型)` 组合生成的，不是绑定在具体类型上。

---

## 五、itab 缓存与 hash 的作用

### hash 到底是什么？

`itab.hash` 来自 `_type.hash`，即**具体类型自身的 hash**，不是接口方法集合的 hash。

### 为什么需要 hash？

runtime 维护了一个全局的 itab 缓存表：

```text
itab cache: (interfacetype, _type) → *itab
```

当发生接口赋值或类型断言时，runtime 查找缓存：

- **有 hash**：用 `_type.hash` 快速定位桶，再精确匹配 → O(1) 平均
- **无 hash**：需要线性遍历所有已缓存的 itab → O(n)

### 方法匹配何时发生？

| 阶段 | 操作 |
|------|------|
| 第一次（冷路径） | 遍历接口方法 + 具体类型方法，逐一匹配，生成 `fun[]`，创建 itab 并写入缓存 |
| 之后（热路径） | 通过 hash 直接从缓存中取出已有 itab，不再重新匹配 |

---

## 六、普通类型不会天然拥有 iface

常见误解：

> "实现了 interface 的类型都会拥有 iface。"

**错误。** iface 只在**赋值给 interface 变量时**才由 runtime 创建。

```go
type MyError struct{}
func (MyError) Error() string { return "" }

var err MyError       // 普通 struct，内存中只有数据，没有 itab
var e error = err     // 此时 runtime 创建 iface{tab: itab(error+MyError), data: &err}
```

---

## 七、nil interface 陷阱

这是 Go 面试的经典考点。

### 问题代码

```go
type MyError struct{}
func (*MyError) Error() string { return "" }

func getErr() error {
    var p *MyError = nil
    return p          // 返回的 error 不是 nil！
}

err := getErr()
fmt.Println(err == nil) // false
```

### 原因分析

`return p` 时发生隐式接口赋值，生成的 iface：

```text
iface
+------+
| tab  | ---> itab(error + *MyError)  ← 不为 nil
+------+
| data | ---> nil
+------+
```

因为 `tab != nil`，所以 `err != nil`。

### 真正的 nil interface

必须 `tab` 和 `data` **同时为 nil**：

```go
var err error = nil   // tab == nil, data == nil → 真正的 nil
```

### 安全写法

```go
func getErr() error {
    var p *MyError = nil
    if p == nil {
        return nil    // 直接返回无类型 nil
    }
    return p
}
```

---

## 八、为什么普通 struct 不能和 nil 比较？

```go
var u User
u == nil  // 编译错误
```

Go 中只有以下类型可以为 nil：

| 可以为 nil | 不可以为 nil |
|-----------|------------|
| pointer, slice, map, func, channel, interface | struct, array, int, bool, string |

struct 是值类型，它的零值是所有字段的零值，不是 nil。

---

## 九、与 reflect 的关系

reflect 包的核心操作就是**读取 interface 内部保存的类型和值**：

```text
interface variable
       │
       ▼
  eface / iface
       │
       ├── _type ──→ reflect.TypeOf()   返回类型信息
       │
       └── data  ──→ reflect.ValueOf()  返回值的可操作包装
```

这就是为什么 `reflect.TypeOf()` 和 `reflect.ValueOf()` 的参数都是 `interface{}`——它们需要拿到完整的 `(type, value)` 二元组才能工作。

---

## 十、整体关系图

```text
                    interface 变量

              ┌───────────────────┐
              │                   │
              ▼                   ▼

          eface                iface
     (空接口 any)          (非空接口)

     +-----------+        +-----------+
     | _type     |        | tab(itab) |
     | data      |        | data      |
     +-----------+        +-----------+
                                │
                                ▼
                     ┌──────────────────┐
                     │ itab             │
                     ├──────────────────┤
                     │ inter            │ → 接口类型的方法集合
                     │ _type            │ → 具体类型的完整信息
                     │ hash             │ → _type.hash（缓存定位用）
                     │ fun[0..n]        │ → 方法地址表
                     └──────────────────┘
                                │
                                ▼
                     ┌──────────────────┐
                     │ _type (runtime)  │
                     ├──────────────────┤
                     │ size             │
                     │ kind             │
                     │ hash             │
                     │ GC 信息          │
                     │ reflect 信息     │
                     └──────────────────┘
                                │
                                ▼
                     reflect.TypeOf()
                     reflect.ValueOf()
```

---

## 十一、常见错误总结

| 错误理解 | 正确理解 |
|---------|---------|
| 实现了 interface 的类型都拥有 iface | 只有赋值给 interface 变量后，runtime 才创建 iface |
| hash 是接口方法集合的 hash | hash 是具体类型 `_type.hash`，用于 itab 缓存查找 |
| hash 用来代替方法比较 | hash 用于缓存定位；方法比较只在首次创建 itab 时发生 |
| nil interface 就是 data 为 nil | 只有 tab 和 data 都为 nil，interface 才是真正的 nil |

---

## 十二、推荐下一步

继续深入以下 runtime 函数，可以把本文所有知识点串联起来：

- `getitab()` — itab 缓存查找与创建的入口
- `assertE2I()` — 空接口 → 非空接口的类型断言
- `assertI2T()` — 接口 → 具体类型的类型断言

它们正好覆盖了 eface、iface、itab、hash、类型断言和 reflect 的完整链路。

---

## 一句话总结

> Go 的 interface 本质是运行时的 `(动态类型, 动态值)`。空接口（eface）保存 `_type + data`；非空接口（iface）保存 `itab + data`。`itab` 连接接口类型与具体类型，包含方法表 `fun`，并缓存具体类型的 `_type.hash` 用于快速查找对应的 itab。真正的方法匹配只发生在第一次创建 itab 时，之后都会复用缓存。
