# 事件循环：epoll 语义模拟（LT / ET）

## 难度：⭐⭐⭐ 中等偏难

## 考点
- IO 多路复用核心流程：注册 → 等待就绪 → 分发
- 水平触发 LT vs 边缘触发 ET 的区别
- 就绪事件集合 + 回调分发的工程写法

## 题目描述

用内存结构模拟 epoll 事件循环（真实的 `syscall.EpollCreate1/EpollCtl/EpollWait` 版见文档 2.7，仅 Linux 可运行；本练习跨平台可直接 `go test`）。

实现 `Loop`：

1. `Add(fd, handler, edgeTriggered)`：注册 fd 与其事件回调（`edgeTriggered=true` 模拟 `EPOLLET`）
2. `Remove(fd)`：注销
3. `Notify(fd)`：模拟"fd 就绪"（相当于 `epoll_wait` 返回该 fd）
4. `Consume(fd)`：手动消费掉该 fd 的就绪状态
5. `Process()`：把所有就绪事件分发给对应 handler，然后按触发模式处理：
   - **LT**（默认）：就绪状态**保留**——未 `Consume` 前每次 `Process` 都会再次回调（对应"数据没读完就绪一直存在"）
   - **ET**：分发**一次**后自动清除就绪状态（对应"边缘只触发一次，必须循环读到 EAGAIN"）

## 函数签名

```go
type Loop struct{ /* 自行设计 */ }

func NewLoop() *Loop
func (l *Loop) Add(fd int, handler func(fd int), edgeTriggered bool)
func (l *Loop) Remove(fd int)
func (l *Loop) Notify(fd int)
func (l *Loop) Consume(fd int)
func (l *Loop) Process()
```

## 提示
1. 内部维护两个 map：`handlers map[int]func(int)` 与 `ready map[int]bool`，再记一个 `et map[int]bool`
2. `Notify` 只对**已注册**的 fd 生效
3. `Process` 遍历就绪集合分发；ET 模式下分发后 `delete(ready, fd)`，LT 模式保留
4. 思考：真实 epoll 中为什么 ET 要求"循环读到 EAGAIN"？（边缘只触发一次，读不完数据就丢了）

## 验收
- [ ] LT 下未 Consume 会重复触发，Consume 后停止
- [ ] ET 下无论是否 Consume 只触发一次
- [ ] Remove 后的 fd 即使 Notify 也不会被分发
