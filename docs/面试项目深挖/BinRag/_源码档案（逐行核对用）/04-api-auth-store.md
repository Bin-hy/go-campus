# BinRag 后端技术档案：HTTP API 层 / 认证鉴权 / 数据存储

> 只读分析，未修改任何文件。所有结论均标注 `文件:行号`。找不到的一律写「代码中未找到」。
> 分析范围：`internal/api`（含全部 `_test.go`）、`internal/auth`、`internal/store`、`internal/config`；旁证引用 `internal/app/app.go`、`internal/mcp`、`cmd/server/main.go`、`configs/config.yaml`。

---

## 1. 路由全景表

**全局中间件**（所有路由，含 swagger 与公开登录组）：

`internal/api/router.go:65-67`
```go
gin.SetMode(gin.ReleaseMode)
r := gin.New()
r.Use(Logger(), CORS(), RateLimit(deps.Config.RateLimitQPS))
```

**v1 组认证挂载**（`router.go:95-105`）：`v1.Use(包装中间件)` → 其中 `/api/v1/eval/health`（仅 GET 且路径精确相等）豁免，其余走 `Auth(...)`。
**公开登录组**（`router.go:87-93`）`/api/v1/auth` 不挂 `Auth`。

| # | 方法 | 路径 | 中间件链 | 需认证 | 额外身份/权限约束 | 行号 |
|---|---|---|---|---|---|---|
| 1 | GET | `/swagger/*any` | Logger→CORS→RateLimit | 否 | 无 | router.go:84 |
| 2 | GET | `/api/v1/auth/providers` | 全局 3 件套 | 否 | 无 | router.go:88 |
| 3 | GET | `/api/v1/auth/oidc/:provider/login` | 全局 3 件套 | 否 | provider 必须存在且 `Type()==oidc` | router.go:89 + handler_auth.go:44-48 |
| 4 | GET | `/api/v1/auth/oidc/:provider/callback` | 全局 3 件套 | 否 | state 一次性校验 | router.go:90 + handler_auth.go:86-92 |
| 5 | GET | `/api/v1/auth/github/login` | 全局 3 件套 | 否 | provider 必须已配置 | router.go:91 |
| 6 | GET | `/api/v1/auth/github/callback` | 全局 3 件套 | 否 | state 一次性校验 | router.go:92 |
| 7 | POST | `/api/v1/auth/exchange` | 全局 3 件套 | 否 | ticket 一次性（消费即删） | router.go:93 + handler_auth.go:153-165 |
| 8 | GET | `/api/v1/auth/me` | 全局 3 件套→Auth | **是** | 无 | router.go:108 |
| 9 | GET | `/api/v1/config` | 全局 3 件套→Auth | **是** | 无（任意 Key/会话均可读） | router.go:111 + handler_config.go:143-175 |
| 10 | PUT | `/api/v1/config` | 全局 3 件套→Auth | **是** | **需 bootstrap**（`is_bootstrap` 为真） | router.go:112 + handler_config.go:194-197 |
| 11 | POST | `/api/v1/knowledge-bases` | 全局 3 件套→Auth | **是** | owner 由身份决定 | router.go:115 |
| 12 | GET | `/api/v1/knowledge-bases` | 全局 3 件套→Auth | **是** | 按身份过滤（用户仅自己） | router.go:116 |
| 13 | GET | `/api/v1/knowledge-bases/:id` | 全局 3 件套→Auth | **是** | `canAccessKB` 越权→404 | router.go:117 |
| 14 | PUT | `/api/v1/knowledge-bases/:id` | 全局 3 件套→Auth | **是** | 同上 | router.go:118 |
| 15 | DELETE | `/api/v1/knowledge-bases/:id` | 全局 3 件套→Auth | **是** | 同上 | router.go:119 |
| 16 | POST | `/api/v1/documents/upload` | 全局 3 件套→Auth | **是** | `ensureKBAccess` + kb_id 必须 UUID | router.go:122 + handler_doc.go:52-60 |
| 17 | GET | `/api/v1/documents` | 全局 3 件套→Auth | **是** | `ensureKBAccess` | router.go:123 |
| 18 | GET | `/api/v1/documents/supported-types` | 全局 3 件套→Auth | **是** | 无 | router.go:124 |
| 19 | GET | `/api/v1/documents/:id/raw` | 全局 3 件套→Auth | **是** | 经文档→KB 校验 | router.go:125 |
| 20 | DELETE | `/api/v1/documents/:id` | 全局 3 件套→Auth | **是** | 经文档→KB 校验 | router.go:126 |
| 21 | GET | `/api/v1/videos/:id/stream` | 全局 3 件套→Auth | **是** | 经文档→KB 校验 | router.go:127 |
| 22 | GET | `/api/v1/tasks/:id` | 全局 3 件套→Auth | **是** | 经任务→KB 校验 | router.go:130 |
| 23 | POST | `/api/v1/tasks/:id/retry` | 全局 3 件套→Auth | **是** | 同上 + 仅 failed 可重试 | router.go:131 |
| 24 | POST | `/api/v1/chat` | 全局 3 件套→Auth | **是** | `resolveKBScope` 越权→404 | router.go:134 + handler_chat.go:88-91 |
| 25 | GET | `/api/v1/chat/enhancements` | 全局 3 件套→Auth | **是** | 无 | router.go:135 |
| 26 | GET | `/api/v1/chat/history` | 全局 3 件套→Auth | **是** | 无归属校验（见 §11） | router.go:136 |
| 27 | GET | `/api/v1/chunks/:id` | 全局 3 件套→Auth | **是** | chunk→文档→KB 校验 | router.go:137 |
| 28 | ANY | `/api/v1/eval/*path` | 全局 3 件套→Auth（health 豁免） | **是**（health 除外） | POST `/tasks` 校验 kb_id | router.go:141 + proxy_eval.go:61-65 |
| 29 | POST | `/api/v1/api-keys` | 全局 3 件套→Auth | **是** | `requireSystemKey`（403） | router.go:144 + handler_key.go:66 |
| 30 | GET | `/api/v1/api-keys` | 全局 3 件套→Auth | **是** | `requireSystemKey` | router.go:145 |
| 31 | DELETE | `/api/v1/api-keys/:id` | 全局 3 件套→Auth | **是** | `requireSystemKey` | router.go:146 |
| 32 | POST | `/api/v1/api-keys/:id/toggle` | 全局 3 件套→Auth | **是** | `requireSystemKey` | router.go:147 |
| 33 | PUT | `/api/v1/api-keys/:id/permissions` | 全局 3 件套→Auth | **是** | `requireSystemKey` **且 bootstrap** | router.go:148 + handler_key.go:162-165 |
| 34 | GET | `/api/v1/mcp/my/status` | 全局 3 件套→Auth | **是** | `requireUser`（会话 JWT，403） | router.go:153 + handler_mcp_my.go:39-46 |
| 35 | POST | `/api/v1/mcp/my/key` | 全局 3 件套→Auth | **是** | `requireUser`；已有→409 | router.go:154 |
| 36 | POST | `/api/v1/mcp/my/key/toggle` | 全局 3 件套→Auth | **是** | `requireUser` | router.go:155 |
| 37 | DELETE | `/api/v1/mcp/my/key` | 全局 3 件套→Auth | **是** | `requireUser` | router.go:156 |
| 38 | PUT | `/api/v1/mcp/my/key/permissions` | 全局 3 件套→Auth | **是** | `requireUser` + kb_ids 归属校验 | router.go:157 + handler_mcp_my.go:263-280 |

**路由表之外的挂载点**（不在 `router.go`，但同一 Gin 引擎）：
- `internal/app/app.go:205`：`router.Any(cfg.Server.MCP.Path, gin.WrapH(mcpHandler))` — 仅 `server.mcp.enabled=true` 时挂载；MCP 侧自带独立认证（`internal/mcp/auth.go:47-79`，只认 API Key，不认会话 JWT）。
- `internal/app/app.go:173` → `internal/webui/router.go:22-38`：`GET /`、`GET /assets/*filepath`、`NoRoute`（`/api/*` 未匹配返回 JSON 404，其他 GET 回退 index.html）。

**统一分流**：`POST /api/v1/chat` 的普通/流式由 `router.go:164-169` 的 `ChatDispatch` 分流，判据 `isStreamRequest`（`handler_chat.go:198-204`：`query stream=1` 或 `Accept` 含 `text/event-stream`）。

---

## 2. 中间件链

### 2.1 执行顺序与短路行为

```
Logger()            → CORS()        → RateLimit(qps)     →  [swagger / pub 组 终止]
                                                          →  v1 组包装中间件
                                                               ├─ GET /api/v1/eval/health → c.Next() 直通
                                                               └─ Auth(store, authMgr, enabled, bootstrapKey)
```

**CORS 早于限流**（`router.go:67`）：`OPTIONS` 预检在 `middleware.go:107-110` 直接 `AbortWithStatus(204)` 返回，**不消耗限流令牌、不经过认证**。
**限流早于认证**：未认证请求同样消耗全局令牌（`middleware.go:121-122` 在 Auth 之前）。

### 2.2 认证中间件：双通道区分与校验

`internal/api/middleware.go:18-21` 定义形态判据（**刻意不以 `binrag_` 前缀作为分支条件**）：
```go
var jwtShapeRe = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
```

流程（`middleware.go:28-85`）：

| 步骤 | 条件 | 行为 | 行号 |
|---|---|---|---|
| 0 | `!enabled` | `c.Next()` 直接放行（**不写入任何 Identity**） | 30-33 |
| 1 | `Authorization` 长度 >7 且前 7 字节忽略大小写等于 `Bearer ` | 取 token；否则 token 为空 | 37-39 |
| 2 | token 为空 | `Fail(401,"缺少或无效的 Authorization 头")` + Abort | 40-44 |
| 3 | 匹配 `jwtShapeRe` | `authMgr.VerifyJWT`；失败→`Fail(401,"无效或过期的会话")`+Abort，**不再回退查 API Key**；成功→`Kind=KindUser, UserID=claims.UserID, Provider=claims.Provider` | 47-58 |
| 4 | 非 JWT 形态 | `sha256.Sum256(token)` → hex → `s.GetAPIKeyByHash`（**至多一次查询**） | 61-65 |
| 5 | 查询报错 | `Fail(500,"内部错误")`+Abort | 66-71 |
| 6 | `key==nil \|\| !key.Enabled` | `Fail(401,"无效或已停用的 API Key")`+Abort（不区分「不存在」与「已停用」） | 72-76 |
| 7 | 成功 | `TouchAPIKey`（忽略错误）→ `isBootstrap := bootstrapKey != "" && token == bootstrapKey` → 写 Identity + `c.Set("api_key_id")` + `c.Set("is_bootstrap")` | 78-83 |

Identity 模型见 `internal/auth/identity.go:16-33`（`Kind`: `apikey`/`oidc`；`IsBootstrap`；`UserID`；`Provider`），读取入口 `identity.go:41-48`。

### 2.3 各中间件细节

**CORS**（`middleware.go:102-113`）：`Allow-Origin: *`、`Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`、`Allow-Headers: Authorization, Content-Type`；**无 `Allow-Credentials`**。全放开（注释自陈「全放开，供前端调用」）。

**Logger**（`middleware.go:88-99`）：`slog.Info("HTTP 请求", method, path, status, 耗时ms)`；**只记录 `URL.Path`，不记录 query/body/Authorization**（无凭据泄漏）。

**RateLimit**（`middleware.go:116-129`）：令牌桶，`golang.org/x/time/rate`：
```go
if qps <= 0 { return func(c *gin.Context) { c.Next() } }   // qps<=0 → 不限流
limiter := rate.NewLimiter(rate.Limit(qps), qps)            // 速率=qps，突发=qps
if !limiter.Allow() { Fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试"); c.Abort(); return }
```
- 算法：**令牌桶**；阈值：`qps = server.rate_limit_qps`，burst 同为 `qps`。
- 默认值：`applyDefaults` **未给 `RateLimitQPS` 设默认**（`config.go:524-539` 的 Server 段无此项）→ 零值 0 → **默认不限流**；仓库 `configs/config.yaml:249` 亦为 `rate_limit_qps: 0`。
- 作用域：**进程内单例 limiter，全局共享**，不区分 IP/Key/用户；多副本部署时每副本各有一套配额。

**Recovery**：**代码中未找到** `gin.Recovery` 或自定义灾备中间件（`grep -rn "Recovery\|recover()" internal/ cmd/` 仅命中 `internal/eval/evaluator.go:220`）。`router.go:66` 用 `gin.New()`（非 `gin.Default()`，故不含 Logger/Recovery）。panic 由 `net/http` 每连接 recover 兜底 → 连接中断、无统一 500 响应体、无结构化日志。

**请求体大小限制**：**仅有上传接口**，`internal/api/handler_doc.go:41-43`：
```go
// 大小限制必须先于任何 body 读取（FormFile 会解析整个 multipart body）
maxBytes := int64(h.cfg.UploadMaxSizeMB) * 1024 * 1024
c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
```
超限识别在 `handler_doc.go:64-68`（`errors.As(&http.MaxBytesError)` → 400）。阈值 `server.upload_max_size_mb`，代码默认 50（`config.go:531-533`），仓库配置实际 1024（`configs/config.yaml:243`）。**JSON 接口（chat/config/api-keys）无任何 body 上限，代码中未找到全局 body 限制中间件。**

**超时**：
- HTTP Server：`cmd/server/main.go:39-42` 仅设 `Addr` 与 `Handler`，**未设 `ReadTimeout`/`ReadHeaderTimeout`/`WriteTimeout`/`IdleTimeout`/`MaxHeaderBytes`**（桌面版 `cmd/desktop/main.go:42` 同样只有 `&http.Server{Handler: a.Router()}`）。
- 请求级：**未找到** `context.WithTimeout` 包裹请求的中间件；`c.Request.Context()` 直接下传（如 `handler_chat.go:107`）。
- 组件级超时（非 HTTP 层）：OIDC discovery/token 交换 15s（`auth/oidc.go:17`）、GitHub API 10s（`auth/github.go:16`）、LLM 默认 60s（`config.go:441-443`）。
- SSE 流无超时上限（`handler_chat.go:172-194` 循环到 `done`/`error` 或 channel 关闭）。

---

## 3. API Key 机制

### 3.1 生成规则

`internal/api/handler_key.go:75-81`：
```go
raw := make([]byte, 32)                                  // 32 字节 = 256 bit
if _, err := rand.Read(raw); err != nil { ... }          // crypto/rand
token := "binrag_" + base64.RawURLEncoding.EncodeToString(raw)
```
- 前缀 `binrag_`（7 字符，**仅展示用途，认证分支不依赖它**，见 `middleware.go:18-21` 注释与 `auth_flow_test.go:46-50`）。
- 明文长度：7 + 43（32 字节 base64url 无填充）= **50 字符**；熵 **256 bit**。
- 用户 MCP 凭据同规则：`handler_mcp_my.go:114-119`（同 `binrag_` + 32 字节）。

### 3.2 存储形式

- **SHA-256，无盐，hex 编码**，不可反查（单向哈希；认证靠重算哈希后等值查询）。
- 计算处：`handler_key.go:83-87`（创建）、`handler_mcp_my.go:120-129`（用户凭据）、`middleware.go:61-62`（校验）、`app.go:320-324`（bootstrap 种子）、`mcp/auth.go:58-59`（MCP 校验）。
- 列定义：`api_keys.key_hash TEXT NOT NULL UNIQUE`（`store/schema.go:44`）。
- 查询：`store/apikey.go:70-80` `SELECT <cols> FROM api_keys WHERE key_hash = $1`（唯一索引命中，`pgx.ErrNoRows` → 返回 `(nil, nil)`）。
- 无盐在此场景可接受：明文为 256 bit 均匀随机，不存在字典/彩虹表空间；但 **bootstrap key 往往是人选弱口令**（见 §11）。

### 3.3 bootstrap key 的初始化与幂等

`internal/app/app.go:307-332`（启动装配期调用，`app.go:76`）：
```go
func seedAPIKey(ctx context.Context, st store.Store, bootstrap string) error {
	if bootstrap == "" { return nil }                 // 未配置 → 不种子
	keys, err := st.ListAPIKeys(ctx)                  // 幂等判据：表是否为空
	if err != nil { return err }
	if len(keys) > 0 { return nil }                   // 已有任意 Key → 跳过
	sum := sha256.Sum256([]byte(bootstrap))
	key := store.APIKey{ID: uuid.New().String(), Name: "bootstrap", KeyHash: hex.EncodeToString(sum[:]), Enabled: true}
	if err := st.CreateAPIKey(ctx, key); err != nil { return err }
	slog.Warn("已创建 bootstrap API Key，请立即从配置中移除 bootstrap_api_key 项")
	return nil
}
```
- 幂等性**以「`api_keys` 表整体为空」为判据**（非「bootstrap hash 是否存在」）。推论：删光所有 Key 后重启会**重新种子**；反之只要还有任一 Key，配置里改了 `bootstrap_api_key` 也**不会**重建。
- `bootstrap_api_key` 语义为「仅首次启动种子用」（`config.go:275`），明文始终留在 YAML 配置中直到人工删除。

### 3.4 bootstrap 权限判定与其副作用

`middleware.go:79`：`isBootstrap := bootstrapKey != "" && token == bootstrapKey` — **明文等值比较（非恒定时间），且依赖配置中仍存在该明文**。
→ 一旦按 `app.go:330` 的告警从配置中删除 `bootstrap_api_key`，**再没有任何 Key 具备 bootstrap 身份**，`PUT /config`（`handler_config.go:194-196`）与 `PUT /api-keys/:id/permissions`（`handler_key.go:162-165`）将永久 403。这是可复现的运维死锁。

### 3.5 启停（toggle）语义

- 接口：`POST /api/v1/api-keys/:id/toggle`（`router.go:147`）→ `handler_key.go:220-235`。
- 请求体：``toggleAPIKeyRequest{ Enabled bool `json:"enabled"` }``，注释明确「不能用 required：false 是合法值」（`handler_key.go:22-24`）。**副作用**：`{}` 或缺字段会解析为 `false` → 静默停用该 Key。
- 落库：`store/apikey.go:96-99` `UPDATE api_keys SET enabled = $2 WHERE id = $1`（**不检查影响行数**，不存在的 id 也返回 200）。
- 生效：认证时读取 `key.Enabled`（`middleware.go:72`），**无缓存**，故下一次请求立即 401；无审计记录。
- 无任何保护：可停用/删除**自身正在使用的 Key**，也可停用 bootstrap Key（无 guard），造成自锁。

### 3.6 last_used 更新时机与写放大

`middleware.go:78`：`_ = s.TouchAPIKey(ctx, key.ID)`，落库 `UPDATE api_keys SET last_used_at = now() WHERE id = $1`（`store/apikey.go:108-111`）。
- 时机：**每一个认证成功的 API Key 请求**，在认证路径内**同步**执行。
- 写放大：每次请求 1 次 `UPDATE`（行级锁 + WAL），无节流、无合并、无批量、无异步队列；错误被 `_ =` 丢弃。后果：QPS 高时 `api_keys` 成为热点行、认证链路变成「读+写」两次 DB 往返，无法用只读副本承接。会话 JWT 路径不做此写（`middleware.go:47-58`）。

### 3.7 删除与吊销

- `DELETE /api/v1/api-keys/:id`（`router.go:146`）→ `handler_key.go:194-203` → `store/apikey.go:102-105` `DELETE FROM api_keys WHERE id = $1`（**物理删除、硬删、不可恢复、无软吊销、无审计**）。
- 用户自助吊销：`DELETE /api/v1/mcp/my/key`（`handler_mcp_my.go:198-220`，先按 `owner_id` 取 id 再删）。
- **无密钥轮换机制**（代码中未找到 rotation/grace period），也无「吊销后立即失效」之外的状态机；因无缓存，删除即时生效。

### 3.8 权限字段结构

**不是 JSONB**。`internal/store/store.go:86-89`：
```go
// —— 以下为 MCP 权限（spec F4/F5/F6）——
MCPTools   []string // 允许调用的 MCP Tool 白名单；空 = 无任何 MCP 权限
MCPKBScope string   // ""（无 MCP 知识库权限）| "all"（全部）| "allowlist"（仅 MCPKBIDs）
MCPKBIDs   []string // MCPKBScope=="allowlist" 时的知识库白名单
```
列类型（`store/schema.go:86-90`，幂等 ALTER 追加）：
```sql
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS mcp_tools TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS mcp_kb_scope TEXT NOT NULL DEFAULT '';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS mcp_kb_ids TEXT[] NOT NULL DEFAULT '{}';
```
写入为**全量替换（PUT 语义）**，`nil` 切片显式转为空数组：`store/apikey.go:115-128`。请求体结构 `store.APIKeyPermissions`（`store.go:93-97`）。scope 枚举白名单校验在 `handler_key.go:171-174` 与 `handler_mcp_my.go:247-250`。解析为三态权限：`internal/mcp/permission.go:13-22`。

### 3.9 明文只返回一次的实现在哪

- 系统级创建：`internal/api/handler_key.go:96` → `OK(c, gin.H{"id": key.ID, "name": key.Name, "key": token})`（`token` 为局部明文变量，函数返回后不可再获取）。
- 用户 MCP 凭据创建：`internal/api/handler_mcp_my.go:138` → `OK(c, CreateMyKeyResult{ID: key.ID, Key: token})`。
- 列表视图**不含 hash 与明文**：`keyView`（`handler_key.go:27-37`）字段仅 id/name/enabled/last_used_at/created_at + 3 个 MCP 权限字段；`ListAPIKeys` 转换处 `handler_key.go:120-130`。
- 测试断言：`api_test.go:1065-1070` 断言列表响应体不含 `key_hash` 也不含明文。

---

## 4. OIDC / GitHub 登录

### 4.1 授权码流程全貌

```
GET /auth/oidc/:p/login
  → handler_auth.go:44-48  provider 存在且 Type()==oidc
  → Manager.BeginLogin (auth.go:117-133)
       OIDC: nonce = newToken(); state = states.New(provider, nonce, 0)   // TTL 10min
       GitHub: nonce 恒为空串
  → p.AuthCodeURL(state, nonce) → 302
GET /auth/oidc/:p/callback?code&state
  → Manager.CompleteLogin (auth.go:138-152)
       providerName, nonce, ok := states.Consume(state)   // 原子读+删，一次性；失败即中止
       p.ExchangeAndVerify(ctx, code, nonce)
GET /auth/github/callback?code&state → 同一 CompleteLogin
  → handler.finishLogin (handler_auth.go:114-135)
       store.GetOrCreateUser → authMgr.IssueTicket → 302 /login?ticket=xxx
POST /auth/exchange {ticket} → authMgr.ExchangeTicket → Signer.Sign(HS256)
```

### 4.2 state / nonce / PKCE

| 项 | 是否实现 | 依据 |
|---|---|---|
| state（CSRF） | **是**，32 字节随机、TTL 10min、原子一次性消费 | `ticket.go:13,19-25,45-58,61-73`；`auth.go:128,139` |
| provider 绑定防伪造 | **是**：provider 取自 state 记录而非 URL 参数 | `auth.go:136,139-146`（注释「防 URL 参数伪造 provider」） |
| nonce（OIDC） | **是**：登录期生成、绑入 state、授权 URL 携带、ID Token 校验 | `auth.go:122-131`；`oidc.go:120-122`；`oidc.go:160-163`；降级路径 `oidc.go:226-228` |
| PKCE | **代码中未找到**（全仓 grep `pkce/code_challenge/code_verifier/S256` 无命中） | — |
| GitHub nonce | 不适用（GitHub 无 OIDC/ID Token；注释 `github.go:18-20`） | `github.go:68-70` |

state 过期即失败且**先删后查过期**（`ticket.go:64-72`：命中即 `delete`，再判 `ExpiresAt`）→ 过期票据同样被烧掉，不可重试。

### 4.3 回调地址如何拼（public_url）

`internal/auth/auth.go:37-75`（**启动期一次性计算并固定，不随请求变化**，注释见 `auth.go:36`）：
```go
if cfg.PublicURL == "" { return nil, fmt.Errorf("oidc.enabled=true 时 public_url 必填") }
base := strings.TrimRight(cfg.PublicURL, "/")
redirect := pc.RedirectURL
if redirect == "" {
	if pc.Type == config.ProviderTypeOIDC {
		redirect = fmt.Sprintf("%s/api/v1/auth/oidc/%s/callback", base, pc.Name)
	} else {
		redirect = fmt.Sprintf("%s/api/v1/auth/github/callback", base)
	}
}
```
`config.Validate()` 亦要求 `oidc.enabled=true` 时 `public_url` 非空（`config.go:614-616`），显式 `redirect_url` 需为带 `://` 的合法 URL（`config.go:650-654`）。

### 4.4 ticket 机制：一次性与过期如何保证

`internal/auth/ticket.go:15`：`ticketTTL = 2 * time.Minute`。
```go
func (s *ticketStore) Consume(ticket string) (userID, provider string, ok bool) {  // ticket.go:118-130
	s.mu.Lock()
	defer s.mu.Unlock()
	e, exists := s.m[ticket]
	if !exists { return "", "", false }
	delete(s.m, ticket)                    // 先删：并发下仅一个 goroutine 能命中
	if time.Now().After(e.ExpiresAt) { return "", "", false }
	return e.UserID, e.Provider, true
}
```
- **一次性**：`sync.Mutex` 保护的「读+删」原子操作，成功消费后记录消失，重放必然 `!exists`。
- **过期**：`ExpiresAt` 硬判；`New` 时顺带 `cleanupLocked` 清理过期项（`ticket.go:112,132-138`）。
- **并发安全**：`ticket_test.go:64-93`（8 goroutine 并发消费，断言各仅 1 次成功）。
- **JWT 不入 URL**：回调只跳 `/login?ticket=...`（`handler_auth.go:113,134`），测试断言 URL 无 `token`（`auth_flow_test.go:308-311`）。
- **存储为进程内 map**：重启即全部失效；多副本需粘性路由（见 §11）。

### 4.5 JWT 签发参数

`internal/auth/jwt.go:12-20,42-54`：
```go
const jwtIssuer = "binrag"
type SessionClaims struct {
	UserID   string `json:"uid"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}
claims := SessionClaims{ UserID: userID, Provider: provider,
	RegisteredClaims: jwt.RegisteredClaims{
		Issuer: jwtIssuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl)) } }
return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
```
| 项 | 值 | 依据 |
|---|---|---|
| 算法 | HS256（仅此一种） | `jwt.go:53,65,71` |
| claims | `uid`、`provider`、`iss=binrag`、`iat`、`exp` | `jwt.go:44-52` |
| 无 `aud`/`sub`/`jti`/`nbf`/刷新令牌 | 代码中未找到 | — |
| 有效期 | `oidc.jwt_expire_minutes`；`NewManager` 兜底 `defaultJWTExpireMinutes = 120`（`auth.go:13,45-48`）；`applyDefaults` 默认 120（`config.go:548-550`）；仓库配置为 **1200 分钟**（`configs/config.yaml` oidc 段） | — |
| 密钥来源 | `oidc.jwt_secret`；为空则启动期 `crypto/rand` 生成 32 字节，仅进程内持有 | `jwt.go:28-39` |
| 验签 | 显式 `t.Method != HS256` 拒绝 + `jwt.WithIssuer("binrag")` + `jwt.WithValidMethods(["HS256"])` | `jwt.go:63-72` |

**留空随机生成的影响**（`jwt.go:28-39` 注释自陈「重启后旧会话失效」）：① 每次重启全部会话失效，用户被迫重登；② **多副本且各自随机 → 同一 JWT 仅在签发它的副本上验签成功**，除非显式配置共享 `jwt_secret`。

### 4.6 用户 upsert 逻辑（同邮箱不同 provider）

`internal/store/user.go:24-41`：
```go
`INSERT INTO users (id, provider, subject, name, email, created_at)
 VALUES ($1, $2, $3, $4, $5, $6)
 ON CONFLICT (provider, subject)
 DO UPDATE SET name = EXCLUDED.name, email = EXCLUDED.email
 RETURNING id, provider, subject, name, email, created_at`
```
- 唯一键为 `(provider, subject)`（`store/schema.go:66` `UNIQUE (provider, subject)`），**不是 email**。
- **同一自然人用两个不同 provider 登录 → 两条独立 users 记录、两套知识库**；**代码中未找到** email 归并/账号绑定逻辑。
- 每次登录**无条件用 provider 返回的 name/email 覆盖本地值**（`DO UPDATE SET name/email`），且**未校验 `email_verified`**（`oidc.go:151-159` 只取 `name`/`email`，无 `email_verified` 字段）→ email 仅作展示，不作为身份。
- subject 来源：OIDC `idToken.Subject`（`oidc.go:171`，要求非空）；GitHub 用**数字 ID 字符串**（`github.go:112-113` 注释「不用 email 作身份」），并要求 `u.ID != 0`（`github.go:109-111`）。

### 4.7 越权与 CSRF 防护

- **CSRF**：全流程依赖 state（OIDC 另加 nonce）；回调失败统一 302 `/login?error=...` 且**不创建会话/用户**（`handler_auth.go:86-92,102-110,138-140`）；测试 `auth_flow_test.go:347-377` 断言 nonce 被篡改时 `env.store.users` 长度仍为 0。
- **越权**：回调不接受前端可控的 user_id/provider（provider 取自 state，用户由 store upsert 得出，`handler_auth.go:116-122`）；`/auth/providers` 仅返回 `name/type/display_name`（`auth.go:15-20,102-108`），不含 client_secret。
- **登出/会话吊销**：代码中未找到（无 `/auth/logout`，无令牌黑名单）→ CSRF 层面无会话固定问题，但被盗 JWT 在 TTL 内无法作废。
- OIDC 严格控制信任面：仅信任配置 issuer、不调用 userinfo（`oidc.go:124-126`）；`permissive_sub` 降级路径仍复用同一 JWKS 全量校验 iss/aud/exp/nbf/nonce，仅放宽 sub 为数字（`oidc.go:180-238`，测试 `oidc_test.go:209-267`）。

---

## 5. 权限模型与多租户隔离

### 5.1 知识库归属

- 字段：`knowledge_bases.owner_id TEXT`（可空，**无外键**，`store/schema.go:80-83` 注释「NULL = 系统级知识库；无外键约束，用户删除不在本版范围」）。
- 结构体：`store.KnowledgeBase.OwnerID *string`（`store.go:38-46`）。
- 归属写入：**由 API 层按当前身份显式决定**（`handler_kb.go:121-124`）：
```go
// owner 由 API 层按当前身份显式决定：OIDC 用户 → user.ID；系统级 API Key → NULL
if id := auth.IdentityOf(c); id.Kind == auth.KindUser { kb.OwnerID = &id.UserID }
```
- 归属不可变更：`UpdateKB` 的 SQL 不含 owner（`store/kb.go:104-110`）。

### 5.2 「只能看自己的 KB」在哪一层实现

**两层都有，分工不同**：

| 场景 | 实现层 | 依据 |
|---|---|---|
| 列表 `GET /knowledge-bases` | **handler 分支 → SQL where 过滤** | `handler_kb.go:143-157`：`KindUser → ListKBsByOwner`（`WHERE owner_id = $1`，`store/kb.go:55-57`）；否则 `ListAllKBs`（**无 where**，`store/kb.go:35-37`） |
| 单资源（Get/Update/Delete KB、Upload/List/Delete 文档、Task、Chunk、Video、Eval 提交） | **handler 内存判断**（先查记录再判归属） | `canAccessKB` / `ensureKBAccess`（`handler_kb.go:62-83`） |
| RAG 检索范围 | **handler 展开为 KB ID 列表** | `resolveKBScope`（`handler_chat.go:28-57`）；空 kb_id 的登录用户 → `ListKBsByOwner` 的 id 集合；无库 → 400 |

`handler_kb.go:62-71`（核心判断）：
```go
// 系统级 API Key → 全部（含系统级与用户级）；登录用户 → 仅 owner_id == UserID（系统级 NULL 不可见）。
// 不匹配一律返回 false（调用方转 404，不泄露存在性）。
func (h *handler) canAccessKB(c *gin.Context, kb *store.KnowledgeBase) bool {
	id := auth.IdentityOf(c)
	if id.Kind == auth.KindAPIKey { return true }
	return kb.OwnerID != nil && *kb.OwnerID == id.UserID
}
```
注意：**没有 SQL 层的强制隔离**（无 RLS、无 `WHERE owner_id` 的统一查询入口），`store.GetKB` 按 id 直查并「返回 OwnerID，权限判断由 API 层负责」（`store/kb.go:97-101`）——隔离正确性完全依赖 handler 自觉调用 `canAccessKB`。目前所有相关 handler 都调用了（`handler_doc.go:57,208,257,296`；`handler_task.go:35,68`；`handler_chunk.go:68`；`handler_video.go:47`；`proxy_eval.go:105`）。

### 5.3 系统级 Key 与用户 Key 的差异

| 维度 | 系统级 Key（`api_keys.owner_id IS NULL`） | 用户 MCP 凭据（`owner_id = users.id`） |
|---|---|---|
| 创建入口 | `POST /api/v1/api-keys`（需系统 Key） | `POST /api/v1/mcp/my/key`（需会话 JWT） |
| 数量约束 | 不限 | 每用户至多一个（部分唯一索引 `idx_api_keys_owner`，`schema.go:93-96`） |
| 知识库可见性（REST） | 全部 | **代码中未区分 → 也是全部**（见下） |
| 知识库可见性（MCP） | 由 `mcp_tools`/`mcp_kb_scope` 授权，`owner_id=""` 不再收敛 | gateway 按 owner 收敛（`mcp/auth.go:23,77`） |
| 管理 API Key | 允许（`requireSystemKey` 通过） | **代码中未区分 → 也允许** |
| 改配置 / 授 MCP 权限 | 仅 bootstrap 可 | 不可（需 `is_bootstrap`） |

> ⚠️ **本次分析的关键发现**：REST 认证层**从不读取 `APIKey.OwnerID`**。`middleware.go:79-80` 对任何非 JWT 形态、能查到 hash 且 `enabled` 的 Key 一律写 `Kind = KindAPIKey`；`handler_kb.go:67` 见 `Kind==KindAPIKey` 直接 `return true`；`handler_key.go:42` 的 `requireSystemKey` 也只判 `Kind`。全仓 `grep OwnerID internal/api/*.go`（非测试）仅出现在 KB 归属语境，无一处用于 Key 权限收敛。结论：**用户在浏览器里自助生成的 MCP 凭据，在 REST API 上等价于系统级 Key**：可读/改/删所有租户的知识库、文档、chunk、历史，并可通过 `/api/v1/api-keys` 创建/删除/启停任意 Key（`is_bootstrap` 仍阻止改配置与授 MCP 权限）。现有测试未覆盖该组合（`handler_mcp_my_test.go` 全程用 JWT 访问自身接口）。

### 5.4 越权返回码：404 vs 403 的取舍与真实处理

- **404（不泄露资源存在性）**：所有「资源存在但当前身份无权」的情况一律 404，且错误文案与「不存在」完全一致。
  - KB：`handler_kb.go:183-186`、`222-225`、`270-273`（`"知识库不存在"`）
  - 文档：`handler_doc.go:57-60`、`208-211`、`296-299`；视频 `handler_video.go:47-50`
  - 任务：`handler_task.go:35-38`、`68-71`
  - Chunk：`handler_chunk.go:57-71`（chunk→document→KB 三级，任一断裂即 404，含「文档已删除但向量残留」）
  - 评测提交：`proxy_eval.go:105-108`（注释「与 GetKB 同款语义」）
- **403（身份类型不满足，而非资源越权）**：
  - 非系统 Key 调 API Key 管理：`handler_key.go:41-48`（`"仅系统级 API Key 可管理 API Key"`）
  - 非 bootstrap 授 MCP 权限：`handler_key.go:161-165`（注释「防普通/MCP Key 自我提权，安全审查 HIGH」）
  - 非 bootstrap 改配置：`handler_config.go:194-196`
  - 非会话身份访问 `/mcp/my/*`：`handler_mcp_my.go:39-46`
- 设计理由（可由注释与测试印证）：`handler_kb.go:64`「不匹配一律返回 false（调用方转 404，不泄露存在性）」；而 403 用于「你的凭据类型就不被允许」这种不涉及资源存在性的场景，语义更清晰且便于前端提示「请用账号登录」。

### 5.5 `api/kb_isolation_test.go` 断言了什么

| 测试 | 断言 |
|---|---|
| `TestKBOwnerIsolation` (:11-60) | A 建库 200；创建响应 `data` **不含 `owner_id`** 键（`containsJSONKey`）；A 列表含该库；B 列表不含；B 的 GET/PUT/DELETE 均 **404**；系统级 Key 列表**可见**该库 |
| `TestKBOwnerPersisted` (:63-88) | fakeStore 中所有 KB 的 `OwnerID` 非 nil 且等于 `user-a`；B 登录后自己建库，B 的列表长度**恰好 1**（`respCount`） |
| `TestKBSystemLevelInvisibleToUser` (:91-120) | 系统 Key 建库（OwnerID nil）→ OIDC 用户列表**不可见**、GET **404**；系统 Key 可见 |
| `TestKeyManagementSystemOnly` (:123-143) | JWT 调 api-keys 的 create/list/delete/toggle **全部 403**；系统级 Key create **200** |
| `TestChatUserRequiresKBID` (:192-205) | 登录用户**完全无库**时 chat 不指定 kb_id → **400**；指定他人的 `kb-other` → **404** |
| `TestChunkAccessControl` (:208-240) | chunk 无 `document_id` → 404；他人文档的 chunk → 404；**文档已删除但向量残留** → 404；自己的文档 → 200 |

---

## 6. 数据表结构

**建表方式**：**代码内建表 + 幂等 ALTER**，仓库内**无 `.sql` 文件、无 migrations 目录、无迁移库**（`find . -name "*.sql"` 无结果；`go.mod` 无 golang-migrate/goose）。全部集中在 `internal/store/schema.go`，由 `pgStore.Migrate`（`schema.go:121-145`）在启动期顺序执行（`app.go:70-74`）。**没有任何 JSONB 列**（`grep -rn JSONB internal/` 无命中）；等价物是 `TEXT[]` 与存 JSON 字符串的 `TEXT`。

### 6.1 基础 DDL（`schema.go:6-68`）

**knowledge_bases**
| 列 | 类型/约束 |
|---|---|
| `id` | `TEXT PRIMARY KEY` |
| `name` | `TEXT NOT NULL` |
| `description` | `TEXT NOT NULL DEFAULT ''` |
| `created_at` / `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` |
| `strategy` | 追加迁移：`TEXT NOT NULL DEFAULT ''`（存 `StrategyConfig` 的 JSON 字符串，`schema.go:71-73`） |
| `owner_id` | 追加迁移：`TEXT`（NULL=系统级，`schema.go:81-83`） |
索引：仅主键；**`owner_id` 无索引**（`ListKBsByOwner` 为全表/主键扫描后过滤）。

**documents**
`id TEXT PK`、`kb_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE`、`filename TEXT NOT NULL`、`format TEXT NOT NULL DEFAULT ''`、`size BIGINT NOT NULL DEFAULT 0`、`status TEXT NOT NULL DEFAULT 'pending'`、`chunk_ids TEXT[] NOT NULL DEFAULT '{}'`、`file_path TEXT NOT NULL DEFAULT ''`、`task_id TEXT NOT NULL DEFAULT ''`、`created_at TIMESTAMPTZ`。
索引：`idx_documents_kb_id ON documents(kb_id)`（`schema.go:27`）。

**ingest_tasks**（任务表）
`id TEXT PK`、`kb_id TEXT NOT NULL`（**无外键**，删除 KB 时由 `store/kb.go:113-119` 手动清理）、`document_id TEXT NOT NULL`、`status TEXT NOT NULL DEFAULT 'pending'`、`retry_count INT NOT NULL DEFAULT 0`、`error_message TEXT NOT NULL DEFAULT ''`、`created_at`/`updated_at`、追加 `warning_message TEXT NOT NULL DEFAULT ''`（`schema.go:99-101`）。
索引：`idx_ingest_tasks_status ON ingest_tasks(status)`（`schema.go:39`）。
状态枚举：`pending/processing/completed/failed`（`store.go:22-27`）。

**api_keys**
`id TEXT PK`、`name TEXT NOT NULL`、`key_hash TEXT NOT NULL UNIQUE`、`enabled BOOLEAN NOT NULL DEFAULT TRUE`、`last_used_at TIMESTAMPTZ`（可空）、`created_at TIMESTAMPTZ`、追加 `mcp_tools TEXT[] NOT NULL DEFAULT '{}'`、`mcp_kb_scope TEXT NOT NULL DEFAULT ''`、`mcp_kb_ids TEXT[] NOT NULL DEFAULT '{}'`（`schema.go:86-90`）、追加 `owner_id TEXT`（`schema.go:93-94`）。
唯一约束：`key_hash` 唯一 + **部分唯一索引** `CREATE UNIQUE INDEX idx_api_keys_owner ON api_keys(owner_id) WHERE owner_id IS NOT NULL`（`schema.go:95`）→ 每用户至多一个凭据；`owner_id` 可空故系统级 Key 不受限制。

**chat_history**
`id BIGSERIAL PK`、`session_id TEXT NOT NULL`、`role TEXT NOT NULL`、`content TEXT NOT NULL`、`created_at TIMESTAMPTZ`、追加 `sources TEXT NOT NULL DEFAULT ''`（**存引用来源 JSON 字符串，非 JSONB**，`schema.go:76-78`）。
索引：`idx_chat_history_session ON chat_history(session_id, created_at)`（`schema.go:57`）。

**users**（`schema.go:59-67`）
`id TEXT PK`、`provider TEXT NOT NULL`、`subject TEXT NOT NULL`、`name TEXT NOT NULL DEFAULT ''`、`email TEXT NOT NULL DEFAULT ''`、`created_at TIMESTAMPTZ`，**`UNIQUE (provider, subject)`**。

**mcp_audit_logs**（`schema.go:104-118`）
`id BIGSERIAL PK`、`api_key_id TEXT NOT NULL`（**仅引用，无外键**）、`tool_name TEXT NOT NULL`、`params TEXT NOT NULL DEFAULT ''`（截断后 JSON 字符串）、`params_len INT NOT NULL DEFAULT 0`（截断前长度）、`status TEXT NOT NULL DEFAULT ''`（success/error）、`error_message TEXT NOT NULL DEFAULT ''`、`duration_ms BIGINT NOT NULL DEFAULT 0`、`created_at TIMESTAMPTZ`。
索引：`idx_mcp_audit_logs_created_at(created_at)`、`idx_mcp_audit_logs_api_key(api_key_id, created_at)`。
注释自陈「仅记录 api_key_id 引用与截断参数，绝不存 Secret」（`schema.go:103`），表结构确实无 Secret/Token 列。

**索引/约束缺口**：`ingest_tasks.kb_id`、`documents.task_id`、`api_keys.owner_id`（除唯一索引外）无查询索引；`knowledge_bases.owner_id` 无索引；无租户维度的复合索引。

### 6.2 迁移清单（`Migrate` 顺序）

`schemaDDL` → `kbStrategyMigration` → `chatHistorySourcesMigration` → `kbOwnerMigration` → `apiKeyMCPPermissionsMigration` → `apiKeyOwnerMigration` → `ingestTasksWarningMigration` → `mcpAuditLogsDDL`（`schema.go:121-145`）。全部使用 `IF NOT EXISTS`，仅追加不修改历史列，**无版本号表、无 down 迁移、不可回滚**。测试：`schema_test.go:11-42`（pgxmock 按序匹配 8 条 Exec）、`migrate_pg_test.go:13-69`（真实 PG，`BINRAG_TEST_PG_DSN` 未设则 skip；断言两次 Migrate 均成功、权限列可查、同 owner 插第二个凭据被唯一索引拒绝、审计表可查）。

---

## 7. 配置系统

### 7.1 配置结构体全部字段

| 结构体 | 字段（yaml tag） | 默认值（`applyDefaults` 行号） |
|---|---|---|
| `Config` (:16-31) | embedder/vectorstore/chunker/retriever/reranker/llm/rag/web_search/postgres/server/loader/multimedia/oidc/eval | — |
| `EmbedderConfig` (:133-142) | provider, base_url, api_key, model, dimension, batch_size, max_retries, qps | batch_size 100(:381), max_retries 3(:384), qps 10(:387), dimension 1536(:390) |
| `VectorStoreConfig` (:145-150) | host, collection_name, dimension, distance | distance `cosine`(:393), dimension=embedder.dimension(:396) |
| `ChunkerConfig` (:153-158) | strategy, chunk_size, chunk_overlap, heading_level | 512(:399), overlap<0→50(:402), heading 2(:405) |
| `RetrieverConfig` (:105-113) | top_k, rrf_k, vector_weight, bm25_weight, enable_bm25, enable_reranker, multi_query_concurrency | top_k 10(:409), rrf_k 60(:412), vector 0.7(:415), bm25 0.3(:418), mq_concurrency 3(:421) |
| `RerankerConfig` (:116-130) | base_url, api_key, model, top_n, max_retries, qps, mode, llm_prompt_template, llm_temperature | top_n 5(:425), max_retries 3(:428), qps 10(:431) |
| `LLMConfig` (:161-170) | base_url, api_key, model, temperature, max_tokens, max_retries, qps, timeout | max_retries 3(:435), qps 10(:438), timeout 60(:441), temperature 0.7(:444), max_tokens 2048(:447) |
| `WebSearchConfig` (:46-53) | provider, base_url, api_key, count, timeout, qps | provider `bocha`(:451), count 5(:454), timeout 30(:457), qps 1(:460) |
| `RAGConfig` (:198-225) | top_k, max_context_tokens, max_chunks, enable_rewrite, multi_query_enabled, multi_query_count, multi_query_concurrency, multi_query_template_path, decomposition_enabled, decomposition_mode, decomposition_max_sub, step_back_enabled, decomposition/step_back/routing/hyde 模板路径, routing_enabled, routing_fallback, hyde_enabled, hyde_skip_simple, strategy, history_capacity, history_limit, system_prompt_path, context_template_path, rewrite_template_path | top_k 5(:464), max_context_tokens 2048(:467), max_chunks 5(:470), enable_rewrite true(:473), history_capacity 50(:477), history_limit 10(:480), mq_count 3(:484), mq_concurrency 3(:487), decomposition_mode `parallel`(:491), decomposition_max_sub 5(:494), routing_fallback `multi_query`(:498) |
| `StrategyConfig` (:185-195) | query, fusion, decomposition, step_back, hyde, routing, thinking, data_sources | query `multi`(:502), fusion `rrf`(:505), decomposition/step_back/hyde/routing `off`(:508-519) |
| `LoaderConfig` (:58-60) | min_readable_chars | 20(:521) |
| `PostgresConfig` (:263-265) | dsn | 无 |
| `ServerConfig` (:268-278) | port, file_storage_dir, upload_max_size_mb, worker_count, task_max_retries, auth_enabled, bootstrap_api_key, rate_limit_qps, mcp | port 8080(:525), dir `./data/uploads`(:528), upload 50(:531), worker 2(:534), task_max_retries 3(:537)；**auth_enabled 无默认（bool 零值 false）**；**rate_limit_qps 无默认（0）** |
| `MCPConfig` (:281-288) | enabled, path, audit_param_limit | enabled false（零值，安全默认），path `/mcp`(:541), audit_param_limit 2000(:544) |
| `OIDCConfig` (:298-306) | enabled, public_url, jwt_secret, jwt_expire_minutes, providers | jwt_expire_minutes 120(:548) |
| `ProviderConfig` (:313-323) | name, type, display_name, client_id, client_secret, issuer, scope, redirect_url, permissive_sub | type `oidc`(:553)，display_name=name(:556)，scope：github→`[read:user]`、其他→`[openid,profile,email]`(:559-565) |
| `MultimediaConfig` (:65-70) + `VideoConfig` (:73-78) + `SceneConfig` (:81-85) + `MultimediaServiceConfig` (:91-97) | vision/speech(frame_interval_sec,video) ；frame_strategy, frame_interval_sec, scene, vision_embedding；sample_fps, similarity_threshold, min_scene_duration_ms；provider/base_url/api_key/model/timeout | frame_interval 10(:568), provider `openai_compat`(:571,577,599), timeout 30(:574,580,602), frame_strategy `fixed`(:584), sample_fps 2(:590), similarity 0.85(:593), min_scene 3000(:596) |
| `EvalConfig` (:35-43) | service_url, internal_token；`Available()` 要求二者均非空 | 无默认（未配置→503） |

### 7.2 YAML 与环境变量优先级

**仅有 YAML + 一个环境变量**：
- 路径解析优先级（`config.go:329-335`）：`-c/--config` 参数（`app.ParseConfigFlag`，`app.go:295-305`）→ `os.Getenv("BINRAG_CONFIG")` → `./configs/config.yaml`。
- **字段级环境变量覆盖不存在**（`grep os.Getenv internal/config/` 仅 `config.go:331`；`grep '\${' configs/*.yaml` 无命中）。
- 合并顺序（后者覆盖前者）：主 YAML → `.local.yaml` → `applyDefaults()` 补齐零值 → `Validate()`。

### 7.3 `config.local.yaml` 自动合并的实现位置

`internal/config/config.go:347-358`（在 `LoadConfig` 内）：
```go
// 自动合并本地覆盖文件（<主文件名>.local.yaml）
if lp := localOverridePath(path); lp != "" {
	localData, err := os.ReadFile(lp)
	if err != nil {
		slog.Warn("读取本地配置覆盖文件失败，忽略 local 覆盖", "path", lp, "err", err)
	} else if err := yaml.Unmarshal(localData, &cfg); err != nil {   // 同一 &cfg 二次 Unmarshal
		slog.Warn("解析本地配置覆盖文件失败，忽略 local 覆盖", "path", lp, "err", err)
	} else {
		slog.Info("已合并本地配置覆盖", "local", lp, "main", path)
	}
}
```
路径推导：`config.go:367-378` `localOverridePath` — 把扩展名替换为 `.local` + 原扩展名（`config.yaml` → `config.local.yaml`）；文件不存在或与主文件同名（用户显式传 local 文件）则返回空串，**避免 `local.local` 二次合并**。
覆盖语义依赖 yaml.v3 对已填充结构的行为：local 出现的字段覆盖，未出现的保留（注释 `config.go:353`）。测试：`config_test.go:11-58`（local 覆盖 model/api_key，未出现字段保留）、`config_test.go:61-74`（显式传 local 文件时不重复合并）。

### 7.4 是否有热更新 / manager.go 做什么

**有**（仅限白名单的可变更字段）。`internal/config/manager.go`：
- 职责：持有 `atomic.Pointer[Config]` 当前快照 + 上一份快照 + `path`；提供请求级不可变快照读取（`Get`/`Current`，`manager.go:32-40`）。
- `Update(newCfg, rebuild)`（`manager.go:45-73`）串行化四步：
  1. `ValidateConfig(newCfg)`（`manager.go:53`，规则见 :107-139：temperature∈[0,2]、top_k∈[1,50]、权重非负且 **vector+bm25≈1（容差 0.001）**、strategy 枚举、min_readable_chars≥0、mcp.path 必须以 `/` 开头）；
  2. `rebuild(newCfg)` **试构建**新运行时组件（失败即返回错误、不替换，`:57-61`）；
  3. `atomicWriteYAML(path, newCfg)` — 临时文件 + `Sync` + `os.Rename`（`:79-104`）；
  4. `old := m.cur.Load(); m.prev = old; m.cur.Store(newCfg)` 原子替换（`:69-71`）。
- HTTP 入口：`PUT /api/v1/config`（`handler_config.go:192-292`）→ 白名单字段**指针式部分更新**（`ConfigUpdateRequest`，`handler_config.go:11-19`，nil 字段不动）→ `h.cfgMgr.Update(&newCfg, h.rebuild)`（`handler_config.go:272`）。
- 生效范围：**仅影响新请求**；已在执行的请求持旧快照（`manager.go:15-17` 注释；使用点 `handler_chat.go:92-95`、`handler_chat.go:158`、`proxy_eval.go:47`）。
- 重启才生效的字段：`GET /config` 的 `read_only` 组显式标注 `needs_restart: true`（`handler_config.go:165-172`：postgres.dsn、vectorstore.host、server.port、upload_dir、worker_count、chunker.strategy）；MCP 的 enabled/path 亦为重启生效（`handler_config.go:258` 注释）。
- 测试：`manager_test.go:25-51`（Update 成功 + rebuild 被调用 + 文件已写）、`:53-69`（rebuild 失败不替换）、`:71-83`（非法配置不替换）、`:85-105`（50 goroutine 并发 Get 快照完整）、`:107-123`（atomicWriteYAML，父目录不存在时报错）、`:126-148`（MCP path 校验）。

### 7.5 敏感字段如何避免泄漏到日志/接口

**做了**：
- `GET /config` 的视图**结构性剔除所有 API Key 明文**：`LLMView`（`handler_config.go:68-73`）、`EmbedderView`(:76-79)、`RerankerView`(:82-85) 均只含 model/参数，注释三处强调「不含 APIKey 明文」。
- DSN 掩码：`handler_config.go:166` + `maskDSN`(:294-328) 把 `postgres://user:pass@host` 的密码段替换为 `****`。
- 日志：`middleware.go:92-97` 只记录 method/path/status/耗时，**不含 query、body、Authorization**；认证失败日志 `middleware.go:50,67` 只记 err。
- 审计表结构无 Secret 列（`schema.go:104-118`），`mcp/audit.go:22` 注释同旨。

**没做/有缺口**：
- `maskDSN` 仅识别 `postgres://` 前缀，**其他 DSN 形式（如 `postgresql://`、keyword/value）原样回传**（`handler_config.go:300-303` 直接 `return dsn`）→ 密码泄漏。
- `GET /config` **任意已认证身份均可访问**（含普通 Key 与任何登录用户），暴露 vectorstore.host、端口、上传目录、worker 数、chunker 策略等基础设施信息（`handler_config.go:143-175` 无身份分支）。
- `PUT /config` 会把**整份配置（含 llm.api_key、jwt_secret、postgres.dsn、bootstrap_api_key）yaml.Marshal 落盘**（`manager.go:80`）；临时文件由 `os.CreateTemp` 创建（0600），rename 后权限保持 0600——但仓库内 `configs/config.yaml` 本身是 `-rw-r--r--`。
- `config.local.yaml` 在仓库目录下即含 `api_key`/`bootstrap` 类明文，属运维约定而非代码控制。

---

## 8. 统一响应与错误码

`internal/api/response.go` 全文（33 行）：
```go
const (
	CodeOK = 0; CodeBadRequest = 400; CodeUnauthorized = 401; CodeForbidden = 403
	CodeNotFound = 404; CodeConflict = 409; CodeInternal = 500
	CodeBadGateway = 502; CodeServiceUnavailable = 503
)                                                       // :6-16
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}                                                       // :19-23
func OK(c *gin.Context, data any)   { c.JSON(200, Response{Code: CodeOK, Message: "ok", Data: data}) }   // :26-28
func Fail(c *gin.Context, code int, message string) { c.JSON(code, Response{Code: code, Message: message}) } // :31-33
```

**约定**：`{code, message, data}`；成功 `code=0, message="ok"`，HTTP 200；失败 **HTTP 状态码 = 业务码**（`Fail` 用同一个 `code` 同时作为 HTTP status，`response.go:30` 注释「HTTP 状态码与业务码一致，便于前端处理」）；失败响应**不显式设置 data**，`Data any` 无 `omitempty` → 输出 `"data":null`。

**错误码 ↔ HTTP 映射（实际使用点）**：

| 码 | 常量 | 典型触发点 |
|---|---|---|
| 0 | CodeOK | 全部成功 |
| 400 | CodeBadRequest | 请求体绑定失败、缺 kb_id/session_id、非法 kb_id（非 UUID）、非法 mcp_kb_scope、文件超限、格式不支持、无可读文本、非 failed 任务重试、用户无任何可访问知识库（chat） |
| 401 | CodeUnauthorized | 缺/无效 Authorization（`middleware.go:41,51,73`）、ticket 无效/过期（`handler_auth.go:161`）、未认证 `/auth/me` |
| 403 | CodeForbidden | 非系统 Key 管 Key、非 bootstrap 改配置/授 MCP 权限、非会话访问 `/mcp/my/*` |
| 404 | CodeNotFound | 资源不存在**或越权**（统一文案）、provider 不存在（`handler_auth.go:46`）、GitHub 未配置（:71） |
| 409 | CodeConflict | 已有 MCP 凭据（`handler_mcp_my.go:110`） |
| 429 | `http.StatusTooManyRequests`（未定义为常量） | 限流（`middleware.go:123`） |
| 500 | CodeInternal | DB 错误、引擎未初始化、生成 Key 失败、配置管理器未初始化 |
| 502 | CodeBadGateway | 评测上游不可用（`proxy_eval.go:79-80`） |
| 503 | CodeServiceUnavailable | 评测服务未配置（`proxy_eval.go:49`） |

补充约定：`NoRoute` 对 `/api/*` 返回 `{"code":404,"message":"接口不存在"}`（`internal/webui/router.go:29-34`）；Swagger 注解声明全局约定（`router.go:5`）。测试断言：`api_test.go:1089-1116`（成功与错误响应均含 code/message；成功含 data）。

---

## 9. 测试覆盖证据（函数名 + 断言行为）

### internal/api
| 测试 | 断言 |
|---|---|
| `TestAuthDispatch` (:39) | 有效 API Key→200；**非 `binrag_` 前缀但已登记的 Key→200**（前缀非判据）；有效 JWT→200；**伪造三段式 JWT→401**；非 JWT 无效串→401；无凭据→401 |
| `TestAuth` (:959) / `TestAuthDisabledKey` (:982) | 无/错 Key 401；正确 Key 200；`enabled=false` 后 401 |
| `TestMeAPIKey` (:73) / `TestMeOIDC` (:94) / `TestMeUnauthorized` (:124) | apikey 身份 `kind=apikey`+`is_bootstrap`；oidc 返回 user_id/provider/name（按 user_id 查 store）；未认证 401 |
| `TestProvidersPublic` (:134) | 公开 200；未配置时 data 为空数组 |
| `TestLoginUnknownProvider` (:152) | 未知 provider login→404；callback 无效 state→**302 跳 `/login?error=`**（不建会话） |
| `TestExchangeReplay` (:168) | ticket 首次 200；重放 401；伪造 ticket 401 |
| `TestOIDCFullHTTPFlow` (:259) | providers→login(302 且 URL 含 state+nonce)→callback(302 且跳 `/login?ticket=`，**URL 无 token**)→exchange(200 出 JWT)→`/auth/me` 200 且 kind=oidc、name 来自 ID Token（自动注册） |
| `TestOIDCCallbackNonceMismatch` (:348) | nonce 被篡改→302 error 且 **`env.store.users` 为空**（不创建用户/会话） |
| `TestKBOwnerIsolation` / `TestKBOwnerPersisted` / `TestKBSystemLevelInvisibleToUser` / `TestKeyManagementSystemOnly` / `TestChatUserRequiresKBID` / `TestChunkAccessControl` | 见 §5.5 |
| `TestAPIKeyLifecycle` (:1044) | 创建返回 `binrag_` 前缀明文；新 Key 立即可用；**列表响应不含 `key_hash`/明文**；toggle 停用后 401 |
| `TestAPIKeyPermissions` (:10 key_permissions_test.go) | 历史 Key 的 `mcp_tools` 为空数组、`mcp_kb_scope` 为空串；bootstrap 更新 allowlist 200 且查询可见；非法 scope→400；**普通 Key→403；会话 JWT→403** |
| `TestMyMCPLifecycle` (:11) | 初始 key=null；创建返回 id+明文；重复创建 409；套用 allowlist 自己的库 200；**他人 kb_id→400**；停用后 enabled=false；吊销后 key=null 可重建；**API Key 访问→403** |
| `TestConfigGet` (:1260) | `GET /config` 含 `mutable` 与 `read_only` 分组 |
| `TestConfigPutBootstrap` (:1272) / `TestConfigPutForbidden` (:1285) / `TestConfigPutInvalid` (:1295) | bootstrap PUT 200 且响应含新温度；普通 Key 403；temperature=5 → 400 |
| `TestConfigMCPUpdate` (:9 config_mcp_test.go) | 默认 `mcp.enabled=false`；bootstrap 改 enabled/path/audit_param_limit 生效；非 bootstrap 403；非法 path（不以 / 开头）400 |
| `TestUploadTooLarge` (:714) | 11MB > 10MB →400 且**未创建文档/任务记录** |
| `TestUploadUnsupportedFormat` (:626) / `TestUploadNoReadableContent` (:1119) | 不支持格式 400；扫描件/空内容预检 400 且不落库 |
| `TestUploadReturnsTaskID` (:569) | 上传返回 task_id/document_id，落文档+任务记录 |
| `TestDeleteDocument` (:689) | 向量按 chunk_ids 删除、BM25 移除对应数量、文档记录删除 |
| `TestChat` (:748) / `TestChatUserNoKBSpecExpandsToAccessibleKBs` (:798) / `TestChatUserNoAccessibleKBRejected` (:829) / `TestChatAPIKeyNoKBSpecUnlimited` (:841) | 回答与 sources 结构；登录用户空 kb_id → 展开为**自己**的库（不含系统库/他人库）；无库 400；API Key 空 kb_id → 不加过滤 |
| `TestChatSSE`(:884) / `TestChatSSEError`(:910) / `TestChatSSEEmptyStream`(:936) / `TestChatSSEThinking`(:1336) | SSE 事件序列 sources→chunk×N→done / error；thinking 事件在开启时出现 |
| `TestChatIncludeContexts*` (:10,31,52 handler_chat_test.go) | `include_contexts=true` 透传到 AskOptions；默认不填充 content；开启后响应含 content |
| `TestChatRequestStrategyOverride` (:1181) / `TestChatThinkingJSON` (:1305) | 单次请求策略覆盖；非流式响应 `data.thinking` 结构完整 |
| `TestGetHistory` (:993) / `TestRetryTask` (:1012) / `TestRetryTaskNotFailed` (:1032) | 历史按 session 返回 2 条；failed→pending 且 retry_count 归零；非 failed 重试 400 |
| `TestUnifiedResponseShape` (:1089) | 成功/错误响应均含 code、message；成功含 data |
| `TestGetChunkReturnsTimestamp` (:1561) / `TestGetChunkReturnsLocation` (:1687) | chunk payload 的 start_ms/end_ms/page/heading/anchor 正确回传 |
| `TestStreamVideo` (:1593) / `TestGetRawDocument` (:1638) | 视频与原文访问（Content-Type/Range） |
| `TestEvalProxyNotConfigured` (:116) | 无 cfgMgr → 503「评测服务未配置」 |
| `TestEvalProxyPassThrough` (:129) | 路径/查询串/Body **原样透传**，注入 `X-Eval-Internal-Token`，响应原样回传，method/path 不改写 |
| `TestEvalProxyTaskKBForbidden` (:163) | kb 属他人 →404 且**上游 hits==0**（不转发） |
| `TestEvalProxyTaskKBOwned` (:184) | 自有库 →200 且上游 hits==1 |
| `TestEvalProxyHealthNoAuth` (:204) | `GET /eval/health` 无 Authorization 头→200 且到达上游 |
| `TestEvalProxyAuthRequired` (:220) | 其他 eval 端点无凭据→401 且上游 hits==0 |
| `TestEvalProxyUpstreamDown` (:236) | 上游关闭→502「评测服务不可用」 |
| `TestEvalProxyTaskNonJSONBodyPassThrough` (:251) | 非 JSON body 不在代理层拦截，原样透传 |

### internal/auth
| 测试 | 断言 |
|---|---|
| `TestNewManagerDisabled` (:12) | `NewManager(nil)` 成功且 providers 为空 |
| `TestNewManagerMissingPublicURL` (:23) | enabled 但缺 public_url →装配失败 |
| `TestNewManagerDuplicateName` (:33) | Provider name 重复→装配失败（不后覆盖前） |
| `TestNewManagerOAuth2NonGithub` (:48) | oauth2 非 github→装配失败 |
| `TestNewManagerGithub` (:61) | ProviderView type/name 正确；BeginLogin 返回非空 URL 与 state |
| `TestManagerOIDCFlow` (:86) | 全链路 BeginLogin→CompleteLogin→IssueTicket→ExchangeTicket→VerifyJWT；**同 state 二次回调失败**；**ticket 重放失败**；未知 provider / 无效 state 失败 |
| `TestSignVerifyRoundtrip` (:13) | claims（uid/provider/iss）正确 |
| `TestVerifyTamperedPayload` (:35) | 篡改 payload→验签失败 |
| `TestVerifyExpired` (:47) | `ttl=-time.Minute` →失败 |
| `TestVerifyWrongIssuer` (:56) | `iss=evil`→失败 |
| `TestVerifyAlgNone` (:77) | 手工构造 `alg=none` 三段式→失败 |
| `TestVerifyWrongAlgorithm` (:89) | 用 HS256 密钥走 RS256 签名→失败 |
| `TestVerifyWrongSecret` (:107) | 换密钥→失败 |
| `TestNewSignerAutoSecret` (:117) | 空 secret→自动生成并可签发/验证 |
| `TestStateStore` (:10) | state 一次性 + 过期失败 |
| `TestTicketStore` (:39) | ticket 一次性 + 不保存 nonce + 过期失败 |
| `TestConcurrentConsume` (:64) | 8 goroutine 并发：state/ticket **各仅 1 次成功** |
| `TestCleanupOnNew` (:96) | `New` 时顺带清理过期项 |
| `TestOIDCExchangeOK` (:131) / `TestOIDCAuthCodeURL` (:149) / `TestOIDCExchangeFailures` (:159) | 正常换取与 sub 提取；授权 URL 含 nonce；各种失败场景 |
| `TestOIDCDiscoveryFail` (:200) | discovery 不可达→装配失败 |
| `TestOIDCPermissiveSub` (:209) | 数字 sub：严格模式失败、permissive 成功且 `subject="12345"` |
| `TestOIDCPermissiveStillStrict` (:238) | permissive 下 nonce 不匹配/错误 iss/过期 exp/aud 不含 client_id/sub 为布尔 **仍全部失败** |
| `TestGithubExchangeOK`(:53) / `TestGithubAuthCodeURL`(:68) / `TestGithubAPIError`(:80) / `TestGithubMissingID`(:88) / `TestGithubNonNumericID`(:96) / `TestGithubTypeIsOAuth2`(:104) | /user 换取、授权 URL 仅 state、非 2xx 失败、缺 id 失败、非数字 id 失败、Type=oauth2 |

### internal/store
`TestMigrate`(:11) 迁移 SQL 序列 8 条按序匹配；`TestMigrateIdempotentOnRealPG`(:13) 真实 PG 两次 Migrate 幂等 + 权限列/owner 唯一索引/审计表可用；`TestUpdateAPIKeyPermissions`(:14) 全量更新与 nil→空数组清空；`TestCreateAPIKeyNoMCPPermissions`(:48) 创建不指定权限列；`TestGetAPIKeyByOwner`(:68) owner 过滤命中/未命中返回 nil；`TestListKBsByIDs`(:11 audit_test) `ANY($1)` 绑定 + 空白名单不发查询；`TestAppendAuditLog`(:48 audit_test) INSERT 参数绑定（无 Secret 列）；`TestGetOrCreateUser`(:11) upsert 参数；`TestGetOrCreateUserEmpty`(:42) 空 provider/subject 报错；`TestGetUser`(:52) 命中与 `(nil,nil)`；`TestCreateKB`(:16)、`TestListKBs`(:52)、`TestCreateKB_Error`(:258)、`TestClaimPendingTasks`(:116)、`TestResetProcessingTasks`(:149)、`TestGetAPIKeyByHash_Hit/Miss`(:168/:202)、`TestPostgresHistory_Get`(:224)。

### internal/config
`TestLoadConfigLocalOverride`(:11)、`TestLoadConfigLocalExplicitNoDoubleMerge`(:61)、`TestMCPConfigDefaults`(:78)、`TestMCPConfigExplicit`(:94)、`TestOIDCValidate`(:113)、`TestOIDCDisabledNoValidation`(:168)、`TestMultimediaConfigDefaults`(:176)、`TestMultimediaConfigExplicitAndValidate`(:195)、`TestVideoConfigDefaultsAndValidate`(:219)；`manager_test.go` 7 个见 §7.4。

---

## 10. 面试官最可能深挖的 12 个点

### Q1 双通道认证怎么区分？为什么不用前缀判断？
`internal/api/middleware.go:18-21,46-58`
```go
var jwtShapeRe = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
// 符合该形态的 token 才进入 JWT 本地验签；验签失败直接 401，不再尝试 API Key。
if jwtShapeRe.MatchString(token) {
	claims, err := authMgr.VerifyJWT(token)
	if err != nil { slog.Warn("会话 JWT 校验失败", "err", err); Fail(c, CodeUnauthorized, "无效或过期的会话"); c.Abort(); return }
	auth.SetIdentity(c, auth.Identity{Kind: auth.KindUser, UserID: claims.UserID, Provider: claims.Provider})
	c.Next(); return
}
```
**要点**：① 不用 `binrag_` 前缀作判据 → 兼容历史/自定义 Key（`auth_flow_test.go:46-50` 明确断言非 `binrag_` 前缀的合法 Key 仍 200）；② 形态匹配走纯本地验签（**认证路径零网络调用**，`middleware.go:27` 注释 N5）；③ 形态匹配但验签失败 **不 fallback 查库**，既避免「伪造 JWT 变相等价于一次全表 hash 查询」的放大攻击，也让错误语义唯一（`auth_flow_test.go:57-59`）。

### Q2 API Key 存什么？明文只给一次怎么保证？
`internal/api/handler_key.go:75-96`
```go
raw := make([]byte, 32)
if _, err := rand.Read(raw); err != nil { Fail(c, CodeInternal, "生成 Key 失败"); return }
token := "binrag_" + base64.RawURLEncoding.EncodeToString(raw)
sum := sha256.Sum256([]byte(token))
key := store.APIKey{ID: uuid.New().String(), Name: req.Name, KeyHash: hex.EncodeToString(sum[:]), Enabled: true, CreatedAt: time.Now()}
if err := h.store.CreateAPIKey(c.Request.Context(), key); err != nil { Fail(c, CodeInternal, "创建 API Key 失败"); return }
OK(c, gin.H{"id": key.ID, "name": key.Name, "key": token})   // 明文仅此一次
```
**要点**：256 bit 随机 → 无需盐（无字典空间）；只存 SHA-256 hex（`api_keys.key_hash UNIQUE`）；明文是函数局部变量，落库后不可再取；列表视图 `keyView` 不含 hash（`handler_key.go:27-37`）且有测试断言（`api_test.go:1065-1070`）。**可被追问**：bootstrap key 是人选口令 → 无盐 SHA-256 在库泄漏后可能被爆破（§11）。

### Q3 bootstrap 身份怎么判定？有什么坑？
`internal/api/middleware.go:78-80`
```go
_ = s.TouchAPIKey(ctx, key.ID)
isBootstrap := bootstrapKey != "" && token == bootstrapKey
auth.SetIdentity(c, auth.Identity{Kind: auth.KindAPIKey, APIKeyID: key.ID, IsBootstrap: isBootstrap})
```
配合 `internal/app/app.go:316-330`（表空才种子 + 打印「请立即从配置中移除 bootstrap_api_key」）与 `handler_config.go:194`、`handler_key.go:162` 的 bootstrap 闸门。
**要点（加分回答）**：bootstrap 是「配置明文比对」而非数据库标记 → ① 非恒定时间比较；② **配置删明文后全系统再无 bootstrap 身份** → `PUT /config`、`PUT /api-keys/:id/permissions` 永久 403。种子幂等以「表是否为空」为判据 → 删光 Key 重启会复活 bootstrap Key；改了配置里的 bootstrap 明文但表非空则不会同步。正确做法应是库内 `is_bootstrap` 标志位 + 凭据来源与权限解耦。

### Q4 越权为什么返回 404 而不是 403？
`internal/api/handler_kb.go:62-71` 与 `:183-186`
```go
// 不匹配一律返回 false（调用方转 404，不泄露存在性）。
func (h *handler) canAccessKB(c *gin.Context, kb *store.KnowledgeBase) bool {
	id := auth.IdentityOf(c)
	if id.Kind == auth.KindAPIKey { return true }
	return kb.OwnerID != nil && *kb.OwnerID == id.UserID
}
...
if !h.canAccessKB(c, kb) { Fail(c, CodeNotFound, "知识库不存在"); return }
```
**要点**：404 让攻击者无法通过状态码枚举他人资源 id（存在性 + 归属双重遮蔽）；错误文案与真不存在完全一致。**403 保留给「凭据类型不被允许」**（`handler_key.go:41-48`、`handler_config.go:194`、`handler_mcp_my.go:39-46`），因为这类判断不涉及资源存在性。**注意**：这里的 `if id.Kind == auth.KindAPIKey { return true }` 正是 §11 首条漏洞的根因（不区分 owner_id 非空的用户 Key）。

### Q5 多租户隔离落在哪一层？SQL 还是 handler？
`internal/store/kb.go:35-37,55-57`（两种查询）+ `internal/api/handler_kb.go:143-157`（选择器）
```go
// store/kb.go
`SELECT ` + kbColumns + ` FROM knowledge_bases ORDER BY created_at DESC`                        // ListAllKBs：无 where
`SELECT ` + kbColumns + ` FROM knowledge_bases WHERE owner_id = $1 ORDER BY created_at DESC`     // ListKBsByOwner
// handler_kb.go
if id.Kind == auth.KindUser { kbs, err = h.store.ListKBsByOwner(ctx, id.UserID) } else { kbs, err = h.store.ListAllKBs(ctx) }
```
**要点**：**列表用 SQL where 过滤，单资源用 handler 内存判断**（`store.GetKB` 直查并「权限判断由 API 层负责」，`store/kb.go:97`）。这是「无 RLS、无统一查询入口」的代价：隔离正确性依赖每个 handler 记得调 `canAccessKB/ensureKBAccess`（目前 11 处调用点全部到位：`handler_doc.go:57,208,257,296`、`handler_task.go:35,68`、`handler_chunk.go:68`、`handler_video.go:47`、`proxy_eval.go:105`）。RAG 侧另有一层：空 kb_id 的登录用户被展开为其名下 KB ID 白名单（`handler_chat.go:42-56`），并在名下无库时 400。

### Q6 登录用户不带 kb_id 问问题怎么办？（跨租户检索防护）
`internal/api/handler_chat.go:37-51`
```go
id := auth.IdentityOf(c)
if id.Kind != auth.KindUser { return "", nil, true }         // API Key / 匿名：不限定
kbs, err := h.store.ListKBsByOwner(c.Request.Context(), id.UserID)
if err != nil { Fail(c, CodeInternal, "查询可访问知识库失败"); return "", nil, false }
if len(kbs) == 0 { Fail(c, CodeBadRequest, "当前账号没有可访问的知识库，请先创建或指定知识库"); return "", nil, false }
ids := make([]string, 0, len(kbs))
for _, kb := range kbs { ids = append(ids, kb.ID) }
return "", ids, true
```
**要点**：默认拒绝而非默认放开（fail-closed）；展开结果通过 `rag.WithKBIDs` 下推检索层过滤（`handler_chat.go:104-106`），不含系统级库与他人私有库；接口返回 400 而非空答案，便于前端引导。

### Q7 一次性票据是如何保证「一次性 + 过期」的？
`internal/auth/ticket.go:61-73,118-130`
```go
func (s *stateStore) Consume(state string) (provider, nonce string, ok bool) {
	s.mu.Lock(); defer s.mu.Unlock()
	e, exists := s.m[state]
	if !exists { return "", "", false }
	delete(s.m, state)                       // 先删：并发/重放必失败
	if time.Now().After(e.ExpiresAt) { return "", "", false }
	return e.Provider, e.Nonce, true
}
// ticketStore.Consume 同构（:118-130）
```
**要点**：互斥锁保护的「读+删」原子临界区（不是 check-then-act）；TTL state 10min（`ticket.go:13`）、ticket 2min（`ticket.go:15`）；`New` 顺带 GC 过期项（`ticket.go:55,112`）；并发唯一性由 `ticket_test.go:64-93` 用 8 goroutine 断言「各仅 1 次成功」；state 还绑定 provider 与 nonce，回调端 provider 取自 state 而非 URL（`auth.go:139-146`）。

### Q8 JWT 密钥留空随机生成的影响？
`internal/auth/jwt.go:28-39`
```go
func NewSigner(secret string) (*Signer, error) {
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil { return nil, fmt.Errorf("生成 JWT 密钥失败: %w", err) }
		return &Signer{secret: b}, nil     // 仅进程内持有，重启后旧会话失效
	}
	return &Signer{secret: []byte(secret)}, nil
}
```
**要点**：① 单副本：重启即全员掉线（`config.go:301` 与 `auth.go:33` 注释均自陈）；② **多副本：各副本密钥不同 → JWT 只能被签发它的副本验证**，必须显式共享 `jwt_secret`；③ 无刷新令牌、无吊销（§11）；④ HS256 对称密钥意味着任何持有 `jwt_secret` 的一方能任意伪造会话（含 `uid`），故该配置项属高敏感值，而它会被写回 YAML 文件。

### Q9 OIDC 的 nonce 校验与 permissive_sub 降级是否削弱安全？
`internal/auth/oidc.go:120-122,160-171` + `:183-238`
```go
func (p *oidcProvider) AuthCodeURL(state, nonce string) string {
	return p.oauthCfg.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce))
}
...
if nonce == "" || claims.Nonce != nonce { return nil, fmt.Errorf("nonce 校验失败") }
if claims.Nbf != 0 && time.Now().Unix() < claims.Nbf { return nil, fmt.Errorf("id_token 尚未生效（nbf）") }
if idToken.Subject == "" { return nil, fmt.Errorf("id_token 缺少 subject") }
```
降级路径 `verifyPermissive`：同一 `oidc.NewRemoteKeySet` 验签（:188），手动校验 `iss`（:210 仅信任配置 issuer）、`aud` 含 client_id（:214，兼容 string/数组 :241-255）、`exp` 必填未过期（:218）、`nbf`（:222）、`nonce`（:226），仅 `sub` 允许数字并确定性转字符串（:230,258-267）。
**要点**：nonce 是 ID Token 重放/混淆的防线，绑定在 state 里，只能来自登录期生成值；降级路径安全强度与严格路径等价（测试 `oidc_test.go:238-267` 用 5 个反例断言 nonce/iss/exp/aud/sub 类型仍全部收敛失败）；`go-oidc` 的 `Verifier` 已校验签名/iss/aud/exp；**不调用 userinfo 端点**减少一次外部信任（`oidc.go:126`）。欠缺：**无 PKCE**（§11）。

### Q10 配置热更新怎么做到「失败不脏」？
`internal/config/manager.go:52-72`
```go
if err := ValidateConfig(newCfg); err != nil { return err }             // 1 校验
if rebuild != nil {
	if err := rebuild(newCfg); err != nil { return fmt.Errorf("新配置组件构建失败，已回滚: %w", err) }  // 2 试构建
}
if m.path != "" {
	if err := atomicWriteYAML(m.path, newCfg); err != nil { return fmt.Errorf("写入配置文件失败: %w", err) } // 3 原子写
}
old := m.cur.Load()                                                     // 4 原子替换
m.prev = old
m.cur.Store(newCfg)
```
**要点**：`sync.Mutex` 串行化整个「校验→试构建→写文件→替换」；**先试构建后替换**保证坏配置不会让线上组件失效（`manager_test.go:53-69`）；写文件用 临时文件+Sync+rename（`manager.go:79-104`）防半截文件；`atomic.Pointer[Config]` 提供**请求级不可变快照**，旧快照对新请求不可见、进行中的请求不受影响（`manager.go:15-17` + 使用点 `handler_chat.go:92-95,158`、`proxy_eval.go:47`）；更新是白名单指针式部分合并（`handler_config.go:214-269`），请求体无法触达启动级字段（dsn/port/oidc 等）。

### Q11 评测反向代理为什么要单独做 kb_id 越权校验？
`internal/api/proxy_eval.go:59-65,88-109`
```go
if c.Request.Method == http.MethodPost && c.Param("path") == evalTaskCreatePath {
	if !h.checkEvalTaskKB(c) { return }
}
...
body, err := io.ReadAll(c.Request.Body)
if err != nil { Fail(c, CodeBadRequest, "读取请求体失败"); return false }
c.Request.Body = io.NopCloser(bytes.NewReader(body))     // 必须复原，供原样透传
var payload evalTaskCreateBody
if err := json.Unmarshal(body, &payload); err != nil { return true }   // 非 JSON 不拦截，交 Python 侧
if payload.KBID == "" { return true }                                  // 未指定由 Python 校验必填
if !h.ensureKBAccess(c, payload.KBID) { Fail(c, CodeNotFound, "知识库不存在"); return false }
```
**要点**：微服务边界处「收口」——Python 侧信任「已由 Go 校验过」的请求；越权与不存在同码 404；**body 读取后必须复原**否则透传空 body（`proxy_eval_test.go:154-156` 断言 body 原样）；非 JSON 不拦截，坚持纯透传原则（`TestEvalProxyTaskNonJSONBodyPassThrough`）；上游调用次数被断言为 0（越权时绝不触达 Python，`:178-180`）；未配置服务整组 503（`:116-126`）；上游挂掉 502 统一包装（`:236-248`）。`/eval/health` 的豁免通过**包装中间件**而非独立路由实现（`router.go:97-105` 注释：Gin 不允许静态路由与 `/eval/*path` 通配共存）。

### Q12 `last_used_at` 的写入时机有什么工程代价？
`internal/api/middleware.go:78` + `internal/store/apikey.go:108-111`
```go
_ = s.TouchAPIKey(ctx, key.ID)        // 认证路径内同步执行，错误被丢弃
...
`UPDATE api_keys SET last_used_at = now() WHERE id = $1`
```
**要点**：这是「可观测性 vs 写放大」的经典权衡题。当前实现：**每个认证成功的 API Key 请求同步写一次库**，无节流/合并/异步 → 热点行更新、认证链路从 1 次读变 1 次读 + 1 次写、无法用只读副本分担、把可用性耦合到写路径。改进方向：内存计数 + 定时批量 flush、或按 key 做 N 分钟节流、或异步队列（可参考本项目 `mcp/audit.go:52-69` 的 buffered channel 异步审计模式）。同一处还埋着 `is_bootstrap` 明文比对（Q3）。

---

## 11. 可被挑刺的安全缺陷（诚实清单）

> 按严重度排序，均附代码依据。

1. **【高】用户自助 MCP 凭据在 REST 层等价于系统级 Key（跨租户越权 + 权限提升）**
   `middleware.go:79-80` 只写 `Kind=KindAPIKey`，**从不读 `key.OwnerID`**；`handler_kb.go:67` `if id.Kind == auth.KindAPIKey { return true }`；`handler_key.go:42` `requireSystemKey` 也只判 Kind。→ 用户通过 `/api/v1/mcp/my/key` 拿到的明文凭据可读改删**任意租户**的 KB/文档/chunk/历史，并可对 `/api/v1/api-keys` 做 create/list/delete/toggle（仅 bootstrap 闸门仍拦配置与 MCP 授权）。MCP 侧有 owner 收敛（`mcp/auth.go:23,77`），REST 侧无。**无任何测试覆盖此组合**（`handler_mcp_my_test.go` 只用 JWT 调自身接口）。
2. **【高】`/api/v1/chat/history` 无归属校验 → 跨租户会话内容泄露**
   `handler_history.go:19-25` 仅按 `session_id` 查询；`session_id` 由客户端自选（`handler_chat.go:16` `binding:"required"`，**无格式/UUID 校验、无 owner 列**）；`chat_history` 表无用户维度（`schema.go:50-56`）。任何已认证身份（含任意 Key/用户）只要猜到或撞到 session_id 即可读取他人问答原文（含 `sources`）。
3. **【中高】无 Recovery 中间件**
   `router.go:66` `gin.New()`（非 `gin.Default()`），`r.Use(Logger(), CORS(), RateLimit(...))` 无 Recovery；全仓无自定义 panic 恢复（仅 `internal/eval/evaluator.go:220` 局部 recover）。panic 时由 `net/http` 每连接兜底：连接中断、**无统一 `{code,message}` 响应**、无结构化日志/告警。
4. **【中高】HTTP Server 无超时**
   `cmd/server/main.go:39-42`（桌面版 `cmd/desktop/main.go:42`）仅设 `Addr`/`Handler`，无 `ReadTimeout`/`ReadHeaderTimeout`/`WriteTimeout`/`IdleTimeout`/`MaxHeaderBytes` → 慢速攻击（slowloris）可长期占用连接；SSE 流（`handler_chat.go:172-194`）无上限，客户端不读也不断。业务层 LLM 有 60s 超时（`config.go:441`）但 HTTP 层无。
5. **【中高】限流形同虚设且可反向滥用**
   ① 默认关闭：`applyDefaults` 未设 `RateLimitQPS`（`config.go:524-539`），仓库配置 `rate_limit_qps: 0`（`configs/config.yaml:249`）→ `middleware.go:117-119` 直接不限流；② 即使开启也是**进程级单桶**（不区分 IP/Key/租户），一个客户端可打满全局配额导致他人 429（`middleware.go:120-126`）；③ 限流在认证之前，**未认证请求同样消耗配额**，构成无凭据 DoS 放大面；④ 多副本各自独立配额，实际阈值 × 副本数。
6. **【中高】JWT 无刷新、无吊销、无 `jti`/`aud`，且密钥可能每副本不同**
   `jwt.go:44-53` claims 仅 uid/provider/iss/iat/exp；全仓无 `refresh_token`/`revoke`/黑名单（grep 无命中）→ 令牌在 TTL 内被盗无法作废（仓库配置 TTL 20 小时，`configs/config.yaml` oidc 段 `jwt_expire_minutes: 1200`）；`jwt_secret` 留空时每进程随机（`jwt.go:30-36`）→ 多副本下会话随机失效；无 `aud` 使令牌无法绑定受众。
7. **【中高】state/ticket 为进程内 map，无容量上限**
   `ticket.go:35-42,92-99` 无最大条目限制，GC 只在 `New`（`:55,112`）时触发；攻击者可反复 `BeginLogin`（公开无认证接口，`router.go:89`）制造无界内存增长；多副本/滚动发布时回调必须落回同一实例（否则 state 找不到 → 登录失败），ticket 同理（`auth.go:160-166`）→ 需粘性会话或改用 Redis/DB，**代码中未找到**任何共享存储方案。
8. **【中】OIDC 未实现 PKCE**：`oauth2.Config` 无 `code_challenge`/`code_verifier`（`oidc.go:68-74`，全仓 grep 无命中）。在移动端/公共客户端或被重定向劫持场景下，授权码可能被截获兑换。
9. **【中】`is_bootstrap` 明文比对带来的运维死锁与弱权限模型**
   `middleware.go:79`（非恒定时间 `==`）+ `app.go:330` 主动建议「从配置中移除 bootstrap_api_key」→ 移除后 `PUT /config`（`handler_config.go:194`）与 `PUT /api-keys/:id/permissions`（`handler_key.go:162`）**永久 403**。同时缺少细粒度授权：REST 侧 Key **无 scope 概念**（任何系统 Key 都是全权管理员，能创建新的全权 Key），只有 MCP 侧有 `mcp_tools/mcp_kb_scope`。
10. **【中】REST 侧关键操作无审计**
    `AppendAuditLog` 唯一生产调用点是 MCP 工具（`internal/mcp/tools.go:124`）；`internal/api` 无任何审计写入。→ Key 创建/删除/启停、MCP 权限授予、`PUT /config`、知识库/文档删除均无痕。且审计本身是**异步可丢弃**（`mcp/audit.go:63-68` 队列满 `default:` 丢弃并 warn；`:81` Shutdown 超时提示「可能丢失部分审计」）→ 审计完整性无保证。
11. **【中】`GET /api/v1/config` 对所有已认证身份开放 + DSN 掩码不全**
    `handler_config.go:143-175` 无身份分支（普通 Key、任何登录用户均可读），暴露 vectorstore.host、端口、上传目录、worker 数、chunker 策略等信息；`maskDSN`（`:294-328`）**只识别 `postgres://` 前缀**，`postgresql://` 或 keyword/value DSN 会**原样返回含密码字符串**。另：`PUT /config` 会把整份配置（含各 `api_key`、`jwt_secret`、`dsn`）落盘（`manager.go:80`）。
12. **【中】CORS 全放开**
    `middleware.go:104` `Allow-Origin: *` + `Allow-Headers: Authorization`。因凭据走 `Authorization` 头（非 Cookie）且无 `Allow-Credentials`，跨站自动带凭据的经典 CSRF 不成立；但任意站点都能在用户浏览器中调用该 API（配合浏览器插件/本地恶意页面泄露 token 的场景风险上升），且与 `POST /auth/exchange`、`/auth/*` 公开端点的组合缺乏 Origin 校验。**代码中未找到**任何 Origin 白名单配置项。
13. **【中】评测代理的路径白名单缺失 + 凭据外泄上游**
    `router.go:141` `v1.Any("/eval/*path")` 任意方法任意子路径；`proxy_eval.go:69-75` Director 只改 scheme/host，**路径与 query 原样带走**（`TestEvalProxyPassThrough` 断言「路径/查询串不改写」是设计目标）→ 客户端可控路径（含编码的穿越片段）会直达评测微服务，安全性依赖 Python 侧路由；同时**不剥离客户端的 `Authorization` 头**，把用户的 API Key/JWT 一并转发给上游服务。
14. **【中】body 大小限制仅覆盖上传接口**
    `handler_doc.go:42-43` 的 `MaxBytesReader` 是唯一限制；`/chat`、`/config`、`/api-keys`、`/eval/*` 等 JSON 端点**无上限**（代码中未找到全局限制中间件）→ 大 body 内存放大（配合无超时更危险）。
15. **【低-中】无邮箱归并与邮箱验证**
    `store/user.go:31-34` 唯一键 `(provider, subject)`，**同邮箱不同 provider → 两个账号、两套数据**；且每次登录无条件覆盖 name/email（`:33`）而不过滤 `email_verified`（`oidc.go:151-159` 未取该字段）→ email 仅作展示，但也意味着无法做「同邮箱合并」，企业 SSO 迁移场景会产生数据孤岛。
16. **【低】写接口不校验资源存在**
    `SetAPIKeyEnabled`/`DeleteAPIKey`（`store/apikey.go:96-105`）不检查 `RowsAffected` → 对不存在 id 的 toggle/delete 返回 **200**（与「404 = 不存在」的统一约定不一致）；`handler_key.go:194-203`、`:220-235` 未做存在性预检。
17. **【低】Key 权限授予校验不对称**
    系统级路径 `PUT /api-keys/:id/permissions` 只校验 scope 枚举（`handler_key.go:171-174`），**不校验 `mcp_kb_ids` 是否存在**（可写悬空 KB id）；用户自助路径却做了归属校验（`handler_mcp_my.go:263-280` → 越权 400）。二者语义不一致。
18. **【低】认证关闭模式功能退化（正确性缺陷）**
    `auth_enabled=false` 时 `middleware.go:30-33` 直接放行且**不写 Identity**，`IdentityOf` 返回零值 → `canAccessKB`（`handler_kb.go:65-71`）对零值 Kind 走 owner 分支、系统级 KB 的 `OwnerID==nil` → **一律 404**；`requireSystemKey`/`requireUser` 则一律 403。即「关认证」并不能得到可用的本地开发体验（KB 详情/文档/任务/视频全部 404）。**无测试覆盖** `AuthEnabled=false`（全仓仅 true）。
19. **【低】无密钥轮换与吊销审计**
    代码中未找到密钥轮换/宽限期/过期时间字段（`api_keys` 无 `expires_at`）→ Key 永久有效直到被删；删除为物理删除（`store/apikey.go:102-105`）且不记录操作者。
20. **【低】`mcp_audit_logs.api_key_id` 无外键、删除 Key 后审计成孤儿**
    `schema.go:104-118` 无 FK 与级联；`DeleteAPIKey` 不清理审计 → 审计记录无法回溯到已删凭据（同时这也是「审计不可篡改」的意外收益，两面可辩）。

---

### 附：本次分析中「代码中未找到」的清单（避免误判为已实现）
- `gin.Recovery` 或等价 panic 恢复中间件
- 全局请求体大小限制中间件 / MaxBytesReader 之外的 body 上限
- HTTP Server 超时（Read/Write/Idle/ReadHeader）与 `MaxHeaderBytes`
- PKCE（`code_challenge`/`code_verifier`/`S256`）
- JWT 刷新令牌、吊销/黑名单、`jti`、`aud`、`sub`
- 密钥轮换 / Key 过期字段（`expires_at`）/ 软吊销
- 会话登出接口、会话固定防护之外的会话失效机制
- state/ticket 的分布式存储（Redis/DB）与条目数上限
- 任何 `.sql` 迁移文件、migrations 目录或迁移库（建表全在 `internal/store/schema.go`）
- 任何 JSONB 列（权限/策略/来源均以 `TEXT[]` 或 JSON 字符串 `TEXT` 存储）
- REST API 层的审计写入（审计仅存在于 MCP 工具调用路径）
- API Key 的 scope/角色模型（REST 侧只有「是/不是系统 Key」与「是/不是 bootstrap」两级）
- email 归并、账号绑定、`email_verified` 校验
- 知识库/文档/任务表的 `owner_id` 索引与数据库级租户隔离（RLS）
- `AuthEnabled=false` 路径的测试覆盖
