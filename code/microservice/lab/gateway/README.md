# Lab：gateway 客户端网关

> 教程章节：07 负载均衡、08 超时重试、09 熔断、13 Trace、15 部署

## 它演示什么

1. **客户端负载均衡**：`round_robin` 把请求分到多个实例；
2. **Deadline**：每次调用派生 800ms 超时，慢实例不会拖死网关；
3. **重试**：service config 的 `retryPolicy`，只对 `UNAVAILABLE` 重试，指数退避 + 抖动；
4. **熔断**：`/fail` 端点连续失败 5 次后进入 3 秒冷却，期间快速返回 503；
5. **两种寻址**：逗号分隔 → manual resolver；单个名字 → `dns:///`（K8s Headless Service）；
6. **幂等键粘性路由**：`/charge` 按 key 哈希到固定实例。这是 lab 为了让"每实例内存幂等表"可演示的折中；
   生产必须把幂等表放 Redis/MySQL，否则实例故障导致 key 漂移仍会重复扣款。

## 运行

```bash
# 先起三个 uservice 实例
go run ./lab/gateway -targets 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003

# 轮询：name 后缀在 user-1/2/3 之间切换
for i in $(seq 1 12); do
  curl -s "http://127.0.0.1:8080/user?id=1" | grep name
done

# 幂等扣款：第二次 deduplicated=true
curl -s "http://127.0.0.1:8080/charge?key=k1&amount=100"; echo
curl -s "http://127.0.0.1:8080/charge?key=k1&amount=100"; echo

# 故障注入 + 熔断
for i in $(seq 1 8); do curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:8080/fail"; done

# 网关统计
curl -s http://127.0.0.1:8080/stats
```

## 关键代码

```go
serviceConfig := `{
  "loadBalancingConfig": [{"round_robin": {}}],
  "methodConfig": [{
    "name": [{}],
    "timeout": "0.8s",
    "retryPolicy": {
      "maxAttempts": 3,
      "initialBackoff": "0.05s",
      "maxBackoff": "0.5s",
      "backoffMultiplier": 2,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`
```

注意：**只有幂等接口才能重试**。`Charge` 之所以能安全重试，是因为服务端用 `idempotency_key`
做了去重；`GetUser` 天然幂等；如果接口是"创建订单"且没有幂等键，就不应放进重试策略。
