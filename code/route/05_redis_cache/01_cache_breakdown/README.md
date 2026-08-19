# 缓存防击穿：带互斥锁的缓存重建

## 难度：⭐⭐⭐ 中等偏难

## 考点
- 缓存穿透 / 击穿 / 雪崩的区别
- SETNX 原子加锁 + TTL 防死锁
- 未抢到锁的请求自旋等待（限次）
- go-redis 对应实现（文档 5.6、`code/backend/02-redis`）

## 题目描述

实现 `GetWithMutex`：并发请求同一热点 key 时，**只有一个请求**负责重建缓存（调 `loader`），其余请求等待缓存出现——这就是防击穿。

流程：
1. 先查缓存，命中直接返回
2. miss：`SetNX("lock:"+key)` 抢锁
3. 抢到锁：调 `loader` 重建 → `Set` 写缓存 → `defer Del` 释放锁；返回新值
4. 没抢到：最多自旋 5 次（间隔 20ms）等缓存出现；仍没有返回 `ErrRebuildTimeout`

## 函数签名

```go
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, val string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
}

type Loader func(ctx context.Context, key string) (string, error)

var ErrRebuildTimeout = errors.New("缓存重建超时")

func GetWithMutex(ctx context.Context, c Cache, loader Loader, key string) (string, error)
```

## 提示
1. `Cache` 接口抽象了 Redis 操作（生产用 go-redis 的 `SetNX`/`Del`，见文档 5.6），这里用内存实现即可离线自测
2. 释放锁用 `defer`，保证任何路径都会执行
3. 思考：锁 TTL 的意义（防持有者崩溃死锁）与风险（业务没跑完锁先过期 → 双重建，可用 watchdog 续期）
4. 进阶：释放锁前先比较 value（Lua 脚本"先比较再删除"），防误删他人锁

## 验收
- [ ] 20 个并发请求下 loader 只被调用 1 次
- [ ] 缓存命中时 loader 0 次调用
- [ ] loader 出错后锁被释放，后续请求能重新抢锁
