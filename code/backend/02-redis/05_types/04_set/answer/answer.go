package setx

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Like 参考答案。
func Like(ctx context.Context, c *redis.Client, postID, userID int64) (int64, error) {
	key := fmt.Sprintf("like:post:%d", postID)
	if err := c.SAdd(ctx, key, userID).Err(); err != nil {
		return 0, err
	}
	return c.SCard(ctx, key).Result()
}

// HasLiked 参考答案。
func HasLiked(ctx context.Context, c *redis.Client, postID, userID int64) (bool, error) {
	key := fmt.Sprintf("like:post:%d", postID)
	return c.SIsMember(ctx, key, userID).Result()
}

// CommonFollow 参考答案。
func CommonFollow(ctx context.Context, c *redis.Client, uidA, uidB int64) ([]string, error) {
	return c.SInter(ctx, fmt.Sprintf("follow:%d", uidA), fmt.Sprintf("follow:%d", uidB)).Result()
}
