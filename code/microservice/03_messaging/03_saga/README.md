# Saga 编排：正向执行 + 逆序补偿

## 难度：⭐⭐⭐ 困难

## 考点
- Saga 是"一串本地事务 + 补偿动作"，解决跨服务长事务
- 编排（orchestration）vs 协同（choreography）
- 逆序补偿：后做的先撤销
- 补偿必须幂等：重试/崩溃恢复会重复执行
- 单步补偿失败不能阻断其他补偿（尽力而为 + 人工介入）
- 取消/超时也要触发补偿

## 题目描述

实现 `Saga.Run(ctx)`：

1. 顺序执行 `Steps[i].Do`；
2. 第 i 步失败 → 逆序补偿 `0..i-1`；
3. 返回 `*SagaError`，包含失败步骤、原因、已补偿步骤、补偿失败明细；
4. 补偿可重试 `CompensateAttempts` 次；
5. ctx 取消时停止执行并补偿已完成步骤。

## 函数签名

```go
type Step struct {
    Name       string
    Do         func(ctx context.Context) error
    Compensate func(ctx context.Context) error
}
type Config struct { CompensateAttempts int; CompensateDelay time.Duration }
type Saga struct { Steps []Step; Cfg Config }
func New(steps []Step, cfg Config) *Saga
func (s *Saga) Run(ctx context.Context) error

type SagaError struct {
    FailedStep        string
    Cause             error
    Compensated       []string
    CompensationFails map[string]error
}
```

## 提示

1. 补偿只针对**已完成**的步骤；失败的那一步自己负责回滚。
2. 用 `[]int` 记录完成步骤的下标，逆序遍历。
3. `errors.As(err, &se)` 让调用方能拿到结构化失败信息。
4. 补偿函数不要依赖 `Do` 的局部变量——崩溃恢复后可能根本拿不到。

## 验收

- [ ] 全成功不触发补偿
- [ ] 第 3 步失败 → 补偿顺序为 `undo(2), undo(1)`（逆序）
- [ ] 某步补偿失败 → 记录在 `CompensationFails`，其他步骤继续补偿
- [ ] 补偿瞬时失败会在配置次数内重试
- [ ] ctx 取消 → 已完成的步骤被补偿
- [ ] `go test -race ./03_messaging/03_saga`

## 运行

```bash
cd code/microservice
go test -race -v ./03_messaging/03_saga
```

## 面试追问

1. **Saga 和 TCC 的区别？** Saga 每步是本地事务 + 补偿（可中途可见脏状态）；TCC 是 Try 预留资源、Confirm/Cancel，隔离性更好但侵入大。
2. **补偿失败怎么办？** 重试 + 告警 + 人工补偿；所以关键业务要做到"补偿也幂等、可重入"。
3. **怎么防止"悬挂"？** 补偿先于正向请求到达（网络重排）会误撤销；需要状态机 + 幂等键约束合法状态转移。
4. **编排和协同怎么选？** 步骤多、需要集中观测/超时控制 → 编排；简单、松耦合 → 协同（事件驱动）。
