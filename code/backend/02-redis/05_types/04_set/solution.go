package setx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Like 用户 userID 给帖子 postID 点赞（集合自动去重），返回当前点赞总数。
func Like(ctx context.Context, c *redis.Client, postID, userID int64) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// HasLiked 查询用户 userID 是否已给帖子 postID 点赞。
func HasLiked(ctx context.Context, c *redis.Client, postID, userID int64) (bool, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// CommonFollow 返回用户 uidA 和 uidB 的共同关注列表（集合交集）。
// 关注集合的 key 为 follow:{uid}。
func CommonFollow(ctx context.Context, c *redis.Client, uidA, uidB int64) ([]string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
