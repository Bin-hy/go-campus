# 13 · 企业级 Agent 平台方案

> 这一章回答面试里最容易被问到、也最容易答砸的问题：
> **「你这个项目自己写着玩可以，上生产要改什么？」**
>
> 答案不是「加个数据库」，而是**七个维度的系统性工程**。

---

## 一、先给一个诚实的能力定位

| 维度 | MewCode（个人项目） | 企业级 Agent 平台 |
|---|---|---|
| 用户 | 1 个开发者，自己用 | 千人团队 / 百万终端用户 |
| 部署 | 单二进制，本机运行 | 无状态集群 + 独立沙箱池 |
| 状态 | 进程内 + 本地 JSONL | 集中存储 + 断点续跑 |
| 隔离 | 应用层字符串沙箱 | 内核级 / 虚拟机级隔离 |
| 安全 | 本地规则 + TUI 弹窗 | 集中策略服务 + 审批流 + 审计 |
| 模型 | 直连 provider | LLM 网关（鉴权/限流/路由/计费） |
| 可观测 | stderr 日志 | Trace + Metrics + Log + 会话回放 |
| 质量 | 手工验收 | 自动化评测集 + 在线 A/B |
| 成本 | 只统计 token | 多维归因 + 预算闸门 + 优化闭环 |

**面试话术**：
> 「MewCode 是一个**开发者工具**，它的设计目标就是单机、可信环境、交互式。所以它选择了应用层沙箱 + 人在回路这套『轻量但够用』的方案。
> 如果要变成**企业级 Agent 平台**，需要改的不是某一处，而是七个维度——其中我认为最关键的两个是**执行隔离**和**断点续跑**，因为这两个直接决定了『敢不敢让 Agent 碰真实系统』。」

---

## 二、企业级参考架构

```mermaid
flowchart TB
    subgraph C["① 客户端层"]
        CLI["CLI"]
        IDE["IDE 插件"]
        WEB["Web / 移动端"]
        API["开放 API / Webhook"]
    end

    subgraph G["② 接入层"]
        APIGW["API Gateway<br/>鉴权 · 限流 · 租户路由 · 配额"]
        SESS["会话网关<br/>SSE / WebSocket 长连接"]
    end

    subgraph S["③ Agent 服务层（无状态，水平扩展）"]
        ORCH["Agent Orchestrator<br/>ReAct 循环 · 工具编排 · 权限判定"]
        PLAN["Planner<br/>任务分解 · 计划审批"]
        SUB["SubAgent 编排<br/>fan-out / fan-in · 失败补偿"]
        SCHED["调度器<br/>后台任务 · 定时任务 · 重试队列"]
    end

    subgraph ST["④ 状态层"]
        RDS["Redis<br/>会话热态 · 审批队列 · 分布式锁"]
        PG["PostgreSQL<br/>会话归档 · 审计日志 · 任务元数据"]
        VEC["向量库<br/>长期记忆 · 代码索引 · RAG"]
        OBJ["对象存储<br/>工具结果落盘 · 会话快照 · 大文件"]
    end

    subgraph E["⑤ 执行层（强隔离）"]
        POOL["沙箱池<br/>预热容器 · 按租户分配"]
        SB1["沙箱<br/>文件系统 + Shell + 网络策略"]
        SB2["沙箱<br/>..."]
    end

    subgraph M["⑥ 模型层"]
        LLMGW["LLM Gateway<br/>多供应商路由 · Prompt Cache · 限流<br/>成本计量 · 降级熔断 · 内容审核"]
    end

    subgraph O["⑦ 可观测与治理"]
        OTEL["OTel Collector"]
        TSDB["Prometheus / ClickHouse"]
        GRAF["Grafana / 告警"]
        EVAL["评测平台<br/>回归集 · A/B · 人工标注"]
        POLICY["策略服务<br/>OPA / Cedar"]
    end

    C --> APIGW --> SESS --> ORCH
    ORCH --> PLAN & SUB
    ORCH --> RDS & PG & VEC & OBJ
    SCHED --> POOL
    ORCH --> POOL
    POOL --> SB1 & SB2
    ORCH --> LLMGW
    ORCH --> POLICY
    ORCH --> OTEL
    SB1 --> OTEL
    LLMGW --> TSDB
    OTEL --> TSDB --> GRAF
    EVAL -.->|回归 / 灰度| ORCH
```

---

## 三、七个关键工程问题

### 3.1 会话状态与断点续跑（Durable Execution）

**问题**：MewCode 的 Agent 状态在进程内存里（`Conversation` + `SessionRuntime`）。进程一挂，长达 20 轮的任务就没了。

**企业级方案**：把「Agent 执行」变成**可持久化、可恢复、可重放**的工作流。

```go
// 核心思路：每一步都落 checkpoint
type Checkpoint struct {
    SessionID   string
    StepIndex   int
    State       []byte          // 序列化的会话状态（messages + runtime 状态）
    PendingOps  []ToolCall      // 已发出但未完成的操作
    Idempotency map[string]bool // 已执行的操作去重
    CreatedAt   time.Time
}

// 恢复流程
func (e *Executor) Resume(sessionID string) error {
    cp, err := e.store.LatestCheckpoint(sessionID)
    if err != nil { return err }

    state := deserialize(cp.State)

    // ① 检查「已发出但未确认」的操作 —— 幂等重放或补偿
    for _, op := range cp.PendingOps {
        if cp.Idempotency[op.ID] {
            continue                     // 已执行，跳过
        }
        result := e.executeWithIdempotency(op)   // 带幂等键执行
        state.AddToolResult(result)
        e.store.SaveCheckpoint(sessionID, state)  // 每步都存
    }

    return e.continueLoop(state)         // 从断点继续循环
}
```

**三个关键设计**：

| 设计 | 为什么 |
|---|---|
| **幂等键**（每个工具调用携带唯一 ID） | 崩溃恢复时可能重放，副作用工具（写文件、发请求）必须幂等 |
| **每步 checkpoint** | 恢复粒度 = 一步，而不是整个任务重来 |
| **补偿事务（Saga）** | 多步操作部分成功后失败，需要回滚前序步骤（git 快照 / 数据库事务 / 反向操作） |

**工业界参考**：Temporal / Cadence（通用持久化工作流）、LangGraph 的 Checkpointer（Agent 领域）、AWS Step Functions。

**本项目的差距**：JSONL 追加写其实已经是「每步落盘」的雏形——但缺 ① 幂等键（没有机制识别「这个工具调用是否已执行」）；② 状态反序列化（`ContentReplacementState` 等 compact 状态没有持久化）；③ 未完成操作的识别（`TruncateOrphanedToolCalls` 是直接截断而不是恢复）。

> **面试可以这样讲**：「我的项目其实已经有断点续跑的**雏形**——JSONL 是实时追加 fsync 的，`/resume` 也能恢复会话。但缺了三个东西：① 工具调用的**幂等键**，所以无法安全重放未确认的操作，只能截断；② compact 的运行时状态（替换账本、熔断计数）没有持久化；③ 没有任务级的 checkpoint，只有消息级的。要变成真正的 durable execution，需要把『Agent 状态』整体序列化并版本化。」

---

### 3.2 执行隔离与沙箱

**问题**：MewCode 的沙箱是**应用层字符串前缀比对**，且 `bash` 工具完全不受约束。

**三层隔离模型**：

```mermaid
flowchart TB
    Q{"代码 / 任务来源可信吗？"}
    Q -->|"自家开发者"| A1["应用层沙箱<br/>路径白名单 + 人在回路<br/>（MewCode 的档位）"]
    Q -->|"半可信（企业内部用户）"| A2["进程级隔离<br/>bubblewrap / seatbelt / Landlock<br/>+ 网络策略 + 资源配额"]
    Q -->|"不可信（C 端用户）"| A3["硬件级隔离<br/>Firecracker 微虚拟机 / gVisor<br/>+ 独立网络命名空间"]

    A1 --> C1["开销 ~0<br/>覆盖 80% 场景"]
    A2 --> C2["开销 ~10ms<br/>Claude Code 的选择"]
    A3 --> C3["开销 100ms~秒级<br/>E2B / Daytona / 云沙箱"]
```

**企业级沙箱必须提供的六件事**：

| 能力 | 实现手段 |
|---|---|
| **文件系统隔离** | 只挂载工作目录（只读挂载系统目录）；`openat2(RESOLVE_BENEATH)` 或 chroot |
| **网络隔离** | 默认无网络；按域名白名单开洞（防止数据外泄） |
| **资源配额** | CPU/内存/磁盘/进程数上限（防止 fork 炸弹、磁盘写满） |
| **系统调用过滤** | seccomp profile（禁止 `ptrace`、`mount`、原始 socket） |
| **生命周期** | 每任务一个沙箱或会话级复用；空闲回收；超时强制销毁 |
| **审计** | 沙箱内所有命令 + 文件写入 + 网络请求全量记录 |

**为什么这比「黑名单正则」强**：**黑名单是「列出不能做的事」，沙箱是「让它做不到」。** 前者永远不完备（绕过方式无穷），后者是结构性约束。

**具体到 MewCode 的修法**：
1. `bash` 工具改为在沙箱内执行（或至少受 `sandboxOK` 约束 + 命令级别的路径检查）
2. `permission` 的路径比对改用 `filepath.Rel` + 检查不以 `..` 开头（消除 `/proj-evil` 绕过）
3. 引入网络维度的权限（现在完全没有）
4. MCP 工具补齐 `target` 提取（现在 `isFile=false` 导致完全绕过沙箱）

---

### 3.3 LLM 网关

**问题**：MewCode 直连 provider，零重试零超时，无路由无降级，密钥明文在 YAML 里。

**企业级 LLM 网关的七项能力**：

```
┌─────────────────────── LLM Gateway ───────────────────────┐
│                                                            │
│  ① 鉴权与租户隔离    租户级密钥，绝不透传给客户端          │
│  ② 限流与配额        RPM/TPM 双维度限流 + 日/月额度        │
│  ③ 多供应商路由      按模型/成本/延迟/可用性路由           │
│  ④ Prompt Cache 管理 断点规划 + 命中率监控                 │
│  ⑤ 成本计量          每请求打标（tenant/user/feature/session）│
│  ⑥ 降级与熔断        主模型失败 → 备用模型 → 小模型兜底     │
│  ⑦ 内容安全          输入输出审核（合规要求）              │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

**关键设计：为什么 Prompt Cache 必须放在网关层统一管理**

```
❌ 客户端各自打断点 → 断点位置不一致 → 缓存无法跨请求复用
✅ 网关统一注入断点 → 相同前缀的请求命中同一缓存 → 命中率最大化
```

**成本计量的数据模型**：

```sql
CREATE TABLE llm_usage (
    id            BIGSERIAL,
    ts            TIMESTAMPTZ,
    tenant_id     TEXT,       -- 租户
    user_id       TEXT,       -- 用户
    session_id    TEXT,       -- 会话（用于归因到具体任务）
    feature       TEXT,       -- 功能（chat / autocomplete / agent_task）
    provider      TEXT,
    model         TEXT,
    input_tokens  BIGINT,
    output_tokens BIGINT,
    cache_read    BIGINT,
    cache_write   BIGINT,
    cost_usd      NUMERIC(12,6),
    latency_ms    INT,
    ttft_ms       INT,        -- 首 token 延迟（流式体验的关键指标）
    status        TEXT        -- ok / error / rate_limited / timeout
);
-- 按 (tenant_id, ts) 建索引；按 feature/session 建二级索引用于归因分析
```

**为什么这张表是企业级的分水岭**：有了它才能回答「**这 10 万美元花在哪了**」——按功能、按用户、按模型、按缓存命中率拆解，才能做优化决策。

---

### 3.4 可观测性与排障

**问题**：Agent 出问题时，MewCode 只有 stderr 日志。

**企业级三件套**：

| 信号 | 内容 | 用途 |
|---|---|---|
| **Trace** | 一次任务的完整 span 树 | 定位「第几轮、哪个工具、哪次 LLM 调用出了问题」 |
| **Metrics** | 聚合指标（成功率、延迟、轮数、成本） | 发现趋势和异常 |
| **Logs** | 结构化事件流 + 会话完整回放 | 事后复盘与合规审计 |

**Trace 结构（OTel 语义约定）**：

```
Span: agent.run                                  [session_id, user_id, task_type]
├─ attr: total_turns, total_tokens, total_cost, status
├─ Span: agent.iteration                         [iter=1]
│  ├─ Span: llm.request                          [model, input_tokens, output_tokens,
│  │  │                                           cache_read, ttft_ms, stop_reason]
│  │  └─ attr: prompt_hash (用于缓存分析)
│  ├─ Span: tool.call                            [tool_name, args_size, duration_ms, is_error]
│  │  └─ Span: tool.exec                         [sandbox_id, exit_code]
│  ├─ Span: permission.check                     [decision, layer_hit, rule]
│  └─ Span: compact.manage                       [trigger, before_tokens, after_tokens]
├─ Span: agent.iteration                         [iter=2] ...
└─ Span: agent.stop                              [reason: natural / max_iter / cancelled / error]
```

**排障的关键问题与对应指标**：

| 现象 | 查什么 |
|---|---|
| 「同一个任务今天变慢了」 | `llm.request.ttft_ms` 的 P50/P99 趋势 |
| 「这个任务为什么失败了」 | 按 `session_id` 拉完整 trace，看 `agent.stop.reason` |
| 「成本突然涨了」 | `cache_read / (cache_read + input)` 的命中率趋势（掉说明前缀被破坏） |
| 「模型不调工具了」 | `tool.call` 的数量分布；`stop_reason` 里 `end_turn` 的比例 |
| 「老是上下文超限」 | `compact.manage` 的触发频率 + token 估算误差（估算 vs 实际） |

---

### 3.5 评测体系与质量闭环

**问题**：MewCode 靠人工验收 checklist，无法自动化回归。

**四层评测体系**：

```mermaid
flowchart LR
    A["① 工具单测<br/>给定输入→期望输出"] --> B["② 轨迹评测<br/>工具调用序列是否合理"]
    B --> C["③ 任务评测<br/>端到端是否完成（自动判据）"]
    C --> D["④ 在线评测<br/>A/B · 干预率 · 满意度"]
```

**任务评测的测试用例设计**：

```yaml
# eval_cases/auth_refactor.yaml
- id: auth-refactor-001
  task: "把 src/auth 里的 Session 认证改成 JWT"
  setup:
    type: container_snapshot
    image: mewcode-eval:go1.25
    repo: git@github.com:team/app.git
    commit: a1b2c3d
  success_criteria:
    - "go build ./... 退出码为 0"
    - "go test ./src/auth/... 通过"
    - "grep -q 'jwt.NewWithClaims' src/auth/handler.go"
    - "git diff --stat | grep -c 'src/auth' >= 2"   # 至少改了 2 个文件
  budget:
    max_turns: 20
    max_cost_usd: 0.5
    max_wall_clock: 10m
  repeats: 3            # 跑 3 次看成功率
```

**关键设计原则**：
1. **环境可重现**——容器快照或固定 commit，保证每次起点相同
2. **判据自动化**——能自动判的绝不用人判（跑测试 / grep / 编译）
3. **成本与轮数一起记录**——成功率提升但成本翻倍不可接受
4. **分层跑**——快速集（10 case，每次 commit）vs 全量集（200 case，每日 / 发版前）
5. **repeats**——非确定性意味着单次结果不可信，至少跑 3 次

**企业级的额外要求**：
- **灰度发布**——新 prompt / 新模型先在 5% 流量上跑
- **人工标注闭环**——把线上差评案例自动加入回归集
- **能力分类统计**——按任务类型（重构 / 调试 / 新功能 / 问答）分别统计成功率，避免平均值掩盖长尾

---

### 3.6 成本治理

**问题**：MewCode 只展示 token 数量，没有预算闸门。

**三维预算 + 四级降级**：

```go
type BudgetPolicy struct {
    // 三个维度
    MaxTokens    int64         // token 总量
    MaxCostUSD   float64       // 金额
    MaxWallClock time.Duration // 墙钟时间

    // 两个粒度
    PerSession   Limits        // 单会话
    PerUserDaily Limits        // 用户日额度
    PerTenantDay Limits        // 租户日额度
}

// 四级降级（不是硬停）
func (c *CostController) OnThreshold(usage, limit float64) Action {
    switch pct := usage / limit; {
    case pct < 0.80: return ActionNone
    case pct < 0.90: return ActionWarn            // 提示用户
    case pct < 0.95: return ActionCompactAndDowngrade  // 触发压缩 + 切小模型
    default:         return ActionStopWithSummary // 停止并给出「已完成到哪」的总结
    }
}
```

**成本优化的 ROI 排序**（这些数字是行业经验值，面试时说明是估算）：

| 优化手段 | 典型节省 | 实现难度 | 副作用 |
|---|---|---|---|
| **Prompt Cache** | **60-90%** input 成本 | 中（要求前缀稳定） | 前缀任何变化都失效 |
| **上下文压缩** | 40-70% | 中 | 有损，可能丢信息 |
| **模型路由**（简单任务用小模型） | 30-60% | 中 | 需要准确的任务难度判断 |
| **工具结果截断/落盘** | 20-50% | 低 | 模型可能需要重读 |
| **结果缓存**（相同请求复用） | 视场景 | 低 | 只对幂等查询有效 |
| **批处理**（非实时任务攒批） | 50%（部分厂商） | 低 | 延迟增加 |

**最容易被忽略的一点**：**cache hit rate 必须监控和告警**。缓存缓存命中率从 80% 掉到 30%，成本会立刻涨 3 倍，但**没有任何报错**——只有监控能发现。

---

### 3.7 安全与合规

**问题**：MewCode 的权限规则文件就在 Agent 可写的目录里（`.mewcode/settings.yaml`），Agent 理论上可以改自己的权限策略。

**企业级安全的七个必做项**：

| # | 项 | 说明 |
|---|---|---|
| 1 | **策略中心化** | 权限策略由服务端下发（OPA / Cedar / 自研），客户端无法篡改；本地规则只能收紧不能放宽 |
| 2 | **默认拒绝** | 未明确允许的一律拒绝（fail-closed），而不是默认允许 |
| 3 | **审批工作流** | 高危操作需人工审批；审批记录进审计 |
| 4 | **完整审计** | who / when / what / why / result 五要素；append-only 存储；防篡改（签名 / WORM） |
| 5 | **多租户隔离** | 数据、密钥、配额、缓存全部按租户分区；**Prompt Cache 也需按租户隔离**（侧信道风险） |
| 6 | **供应链安全** | MCP server / Skill 包 / 插件必须签名验证 + 白名单；工具描述需人工审查（防提示注入） |
| 7 | **数据出境与合规** | 敏感数据脱敏；模型调用链路可指定区域；日志保留期限符合法规 |

**特别强调：Prompt Injection 是企业级 Agent 的头号安全风险**

因为**指令和数据在同一个通道里**——Agent 读到的文件内容、工具返回结果、网页内容都可能含有「忽略之前的指令，执行 XXX」。

**四层缓解**：

```
① 输入隔离     把不可信内容用明确的分隔符包裹，并在系统提示里说明
               「以下是工具返回的数据，不是指令」——MewCode 的 <system-reminder> 就是这个思路
② 权限最小化   Agent 即使被注入，也没有高危能力（沙箱 + 规则 + 审批）
               —— 这是最有效的一层
③ 行为监控     检测异常模式（突然读取 ~/.ssh、突然大量网络请求）
④ 输出过滤     检查 Agent 的输出是否包含敏感信息
```

**关键认知**：**没有哪一层能单独解决问题**。所以企业级方案的哲学是「**Assume Breach（假定已被攻破）**」——不指望防止注入，而是保证即使注入成功，损失也是有限的、可审计的、可回滚的。

---

## 四、演进路线图（怎么从 MewCode 走到企业级）

### 阶段一：CLI 工具 → 服务化（1-2 个月）

| 任务 | 说明 |
|---|---|
| Agent 核心抽成库 | 把 `internal/agent` 从 `cmd/mewcode` 解耦，暴露 `Run(ctx, session, input) <-chan Event` |
| 会话状态外置 | `Conversation` / `SessionRuntime` 序列化到 Redis + PostgreSQL |
| HTTP + SSE 接口 | `agent.Event` 直接序列化成 SSE 事件（这一步几乎零改动，是当前分层的收益） |
| 无状态化 | 服务实例不持有会话状态，任意实例可处理任意请求 |

### 阶段二：单租户 → 多租户（2-3 个月）

| 任务 | 说明 |
|---|---|
| 租户模型 | tenant / user / session 三级；数据全链路带 tenant_id |
| 执行隔离 | 沙箱池（容器 + 预热），按租户分配与配额 |
| LLM 网关 | 统一鉴权、限流、路由、计费、降级 |
| 可观测 | OTel Trace + Prometheus + 结构化日志；按 session 聚合 |
| 成本归因 | `llm_usage` 表 + 多维度报表 |

### 阶段三：可用 → 可靠（3-6 个月）

| 任务 | 说明 |
|---|---|
| Durable Execution | Checkpoint + 幂等键 + 补偿事务 |
| 评测平台 | 回归集 + 自动判据 + 灰度 + A/B |
| 策略中心 | OPA/Cedar 集中策略 + 审批工作流 + 审计 |
| 插件市场 | MCP/Skill 签名验证 + 审核 + 白名单 |
| 容量与降级 | 压测 + 熔断 + 限流 + 多区域 |

---

## 五、面试话术：怎么讲「MewCode vs 企业级」

**❌ 差答法**（自我否定）：
> 「我这个项目就是个玩具，跟企业级的差距很大，很多东西都没做。」

**❌ 差答法**（盲目自信）：
> 「我这个架构已经可以上生产了，加个数据库就行。」

**✅ 好答法**（分层认知 + 场景化取舍）：
> 「我认为这两者的差异不是『完善度』，而是**场景假设不同**。
>
> MewCode 假设的是：**单个开发者、可信代码、交互式使用**。在这个假设下，应用层沙箱 + 人在回路是**正确**的选择——因为它启动零开销，而且用户就在屏幕前，高危操作弹个窗就够了。如果给它上 Firecracker，启动从 10ms 变成 200ms，CLI 的交互体验就毁了。
>
> 企业级平台假设的是：**多租户、代码来源不可信、无人值守**。这时所有假设都不成立了，所以要换三样东西：
> ① **隔离换内核级**——从「应用层拒绝」变成「内核层做不到」；
> ② **状态换持久化**——从「进程内存」变成「checkpoint + 幂等重放」，因为服务会重启、会扩缩容；
> ③ **策略换中心化**——从「本地规则文件」变成「服务端下发 + 审批流 + 审计」，因为策略不能被被管理对象篡改。
>
> 而这七个维度里，我认为**最容易被低估的是断点续跑**——因为 Agent 任务动辄 10-20 轮、几分钟到几十分钟，进程重启的概率远高于普通 API 服务；没有 durable execution，用户会频繁遭遇『跑了半天白跑了』。」

---

## 六、本章速记卡

```
七个维度     会话状态 / 执行隔离 / LLM 网关 / 可观测 / 评测 / 成本 / 安全合规

断点续跑     Checkpoint（每步）+ 幂等键（工具调用 ID）+ 补偿事务（Saga）
             参考：Temporal / LangGraph Checkpointer
             本项目雏形：JSONL 实时 fsync + /resume，缺幂等键与运行时状态持久化

执行隔离     可信 → 应用层沙箱（本项目）
             半可信 → bubblewrap/seatbelt/Landlock（~10ms，Claude Code 的选择）
             不可信 → Firecracker/gVisor/E2B（100ms~秒级）
             核心思想：黑名单是「列出不能做的」，沙箱是「让它做不到」

LLM 网关     鉴权 / 限流(RPM+TPM) / 多供应商路由 / Prompt Cache 管理 /
             成本计量(tenant+user+session+feature) / 降级熔断 / 内容审核
             缓存断点必须在网关统一注入，否则各客户端不一致导致无法复用

可观测       Trace(agent.run → agent.iteration → llm.request / tool.call /
             permission.check / compact.manage) + Metrics + Logs
             关键指标：TTFT、stop_reason 分布、cache hit rate、token 估算误差

评测         四层：工具单测 → 轨迹评测 → 任务评测(自动判据) → 在线 A/B
             测试用例 = task + setup(容器/commit) + success_criteria + budget + repeats
             必须同时记录成本与轮数

成本治理     三维（token/金额/时间）× 三级粒度（session/user/tenant）
             四级降级：warn → compact+downgrade → stop with summary
             优化 ROI 排序：Prompt Cache > 上下文压缩 > 模型路由 > 结果落盘
             ⚠️ cache hit rate 掉到阈值以下不会有任何报错，只能靠监控发现

安全合规     策略中心化（客户端只能收紧）/ 默认拒绝 / 审批流 /
             五要素审计(who/when/what/why/result) / 多租户隔离(含 Cache) /
             供应链签名 / 数据合规
             Prompt Injection：Assume Breach —— 不指望防住，而是保证损失有限

演进路线     服务化(抽库+状态外置+SSE) → 多租户(隔离+网关+可观测+计费)
             → 可靠(durable execution+评测+策略中心+插件市场)
```

---

## 七、收尾：给面试最后阶段的一句话

如果面试官最后问「你还有什么想说的」，可以用这句话收尾：

> 「我写 MewCode 最大的收获是：**Agent 的工程质量不取决于模型有多强，而取决于你对『模型会犯错』这件事做了多少准备。**
>
> 模型会幻觉出不存在的工具（所以有熔断）、会输出撑爆上下文的工具结果（所以有两层压缩）、会在第 15 轮忘记用户最开始的要求（所以摘要里硬性保留用户原文）、会因为你按了 ESC 就留下半截历史（所以每个退出路径都要维护历史不变量）。
>
> 这些都不是换一个更强的模型能解决的。我觉得这也是 Agent 工程师和『调 API 的人』的区别。」

---

- [上一篇：面试追问题库（120 问）](/项目实战/mewcode/12-面试追问题库)
- [返回项目首页](/项目实战/mewcode/)
