# Pipeline 流水线模式

## 难度：⭐⭐⭐ 困难

## 考点
- 多阶段 channel 流水线
- Fan-out / Fan-in
- 优雅关闭与 goroutine 生命周期管理
- context 取消传播

## 题目描述

实现一个数据处理流水线：
1. `Generator` — 生成阶段：产生 start 到 end 的整数
2. `Square` — 处理阶段：将每个数字平方
3. `Filter` — 过滤阶段：只保留满足条件的数字
4. `Merge` — 合并阶段：将多个 channel 合并为一个（Fan-in）
5. `Pipeline` — 组合以上阶段，构建完整流水线

所有阶段必须：
- 接受 done channel 用于取消
- 在 done 关闭后立即退出，不泄漏 goroutine

## 函数签名

```go
func Generator(done <-chan struct, start, end int) <-chan int
func Square(done <-chan struct{}, in <-chan int) <-chan int
func Filter(done <-chan struct{}, in <-chan int, predicate func(int) bool) <-chan int
func Merge(done <-chan struct{}, channels ...<-chan int) <-chan int
func Pipeline(start, end int, predicate func(int) bool) []int
```

## 提示
1. 每个阶段启动 goroutine，返回输出 channel
2. goroutine 中用 select 同时监听 done 和 output channel
3. Merge 用 sync.WaitGroup 等待所有输入 channel 关闭
4. Pipeline 创建 done channel，可以通过 close(done) 取消整条流水线
