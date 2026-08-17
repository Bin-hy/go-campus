# 熔断器状态机：Closed → Open → Half-Open

## 难度：⭐⭐⭐ 困难

## 考点
- 熔断器三态状态机
- 连续失败触发熔断、冷却后探测恢复

## 题目描述

实现一个简单熔断器：

- `NewCircuitBreaker(threshold, cooldown)`：连续失败 `threshold` 次进入 Open 状态
- `Allow()`：Closed/Half-Open 放行；Open 冷却期内拒绝，冷却结束后进入 Half-Open 放行一次
- `RecordResult(ok)`：记录一次调用结果，驱动状态转换
- `State()`：返回当前状态

## 函数签名

```go
type State int
const ( StateClosed State = iota; StateOpen; StateHalfOpen )

type CircuitBreaker struct { ... }
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordResult(ok bool)
func (cb *CircuitBreaker) State() State
```

## 提示

状态转换规则：
- Closed：失败计数达 threshold → Open（记录 openedAt）
- Open：冷却结束 → Half-Open；冷却内 Allow 返回 false
- Half-Open：Allow 放行一次；RecordResult(true) → Closed，RecordResult(false) → 重新 Open

## 运行测试

```bash
cd code/backend/04-microservice/01_circuit_breaker && go test -v
```
