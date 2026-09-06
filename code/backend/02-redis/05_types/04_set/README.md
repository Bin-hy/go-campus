# 04 Set：点赞去重 + 共同关注

## 难度：⭐ 入门

## 考点
- `SADD` 自动去重
- `SISMEMBER` / `SCARD`：成员查询与计数
- `SINTER`：集合交集（共同好友经典题）

## 题目描述

1. `Like`：给帖子 `like:post:{postID}` 的集合加入用户，返回当前点赞总数；重复点赞不增加。
2. `HasLiked`：查询某用户是否已点赞。
3. `CommonFollow`：返回 `follow:{uidA}` 和 `follow:{uidB}` 两个关注集合的交集。

## 函数签名

```go
func Like(ctx context.Context, c *redis.Client, postID, userID int64) (int64, error)
func HasLiked(ctx context.Context, c *redis.Client, postID, userID int64) (bool, error)
func CommonFollow(ctx context.Context, c *redis.Client, uidA, uidB int64) ([]string, error)
```

## 提示

1. `SAdd` 返回**实际新增**的成员数（重复加返回 0）
2. `SCard` 拿集合大小
3. `c.SInter(ctx, key1, key2)` 直接算交集，Redis 服务端完成

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/04_set && go test -v
```
