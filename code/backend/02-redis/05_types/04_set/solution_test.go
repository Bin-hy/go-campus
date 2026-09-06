package setx

import (
	"context"
	"sort"
	"testing"

	"github.com/redis/go-redis/v9"
)

func connect(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB 失败: %v", err)
	}
	return c
}

func TestLike(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	n, _ := Like(ctx, c, 1001, 1)
	if n != 1 {
		t.Fatalf("首次点赞总数应为 1，实际 %d", n)
	}
	n, _ = Like(ctx, c, 1001, 1) // 重复点赞
	if n != 1 {
		t.Fatalf("重复点赞总数应仍为 1，实际 %d", n)
	}
	n, _ = Like(ctx, c, 1001, 2)
	if n != 2 {
		t.Fatalf("另一用户点赞后应为 2，实际 %d", n)
	}

	ok, err := HasLiked(ctx, c, 1001, 1)
	if err != nil || !ok {
		t.Fatalf("用户 1 应已点赞，实际 %v, err=%v", ok, err)
	}
	ok, err = HasLiked(ctx, c, 1001, 99)
	if err != nil || ok {
		t.Fatalf("用户 99 不应点赞，实际 %v, err=%v", ok, err)
	}
}

func TestCommonFollow(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	c.SAdd(ctx, "follow:1", "a", "b", "c")
	c.SAdd(ctx, "follow:2", "b", "c", "d")

	common, err := CommonFollow(ctx, c, 1, 2)
	if err != nil {
		t.Fatalf("CommonFollow 失败: %v", err)
	}
	sort.Strings(common)
	if len(common) != 2 || common[0] != "b" || common[1] != "c" {
		t.Fatalf("共同关注应为 [b c]，实际 %v", common)
	}
}
