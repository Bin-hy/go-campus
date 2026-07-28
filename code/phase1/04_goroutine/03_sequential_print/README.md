# N 个 Goroutine 顺序打印

## 难度：⭐⭐ 中等

## 考点
- 多 goroutine 严格同步
- channel 传递令牌实现顺序控制
- 环形令牌传递

## 题目描述

用 N 个 goroutine 按顺序打印 1~max。
例如 N=3, max=10：
- goroutine 0 打印 1, 4, 7, 10
- goroutine 1 打印 2, 5, 8
- goroutine 2 打印 3, 6, 9
最终输出顺序必须是 1, 2, 3, 4, 5, 6, 7, 8, 9, 10

## 函数签名

```go
func SequentialPrint(n int, max int) []int
```

## 提示
1. 创建 N 个 channel 形成环形：ch[0] → ch[1] → ... → ch[N-1] → ch[0]
2. 每个 goroutine 等待自己的 channel 收到令牌，打印后传给下一个
3. 初始令牌放入 ch[0] 启动流程
