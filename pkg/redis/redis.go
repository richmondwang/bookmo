package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis.Connect parse: %w", err)
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis.Connect ping: %w", err)
	}
	return client, nil
}
