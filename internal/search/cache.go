package search

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ttlNormal = 300 * time.Second
	ttlDev    = 10 * time.Second
)

// Cache wraps a Redis client for feed caching operations.
type Cache struct {
	rdb     *redis.Client
	devMode bool
}

// NewCache constructs a Cache.
func NewCache(rdb *redis.Client, devMode bool) *Cache {
	return &Cache{rdb: rdb, devMode: devMode}
}

// feedCacheKey builds a Redis key for the given lat/lng and category slug.
// Coordinates are rounded to 2 decimal places (~1.1 km grid).
func feedCacheKey(lat, lng float64, categorySlug string) string {
	if categorySlug == "" {
		categorySlug = "all"
	}
	return fmt.Sprintf("feed:%.2f:%.2f:%s",
		math.Round(lat*100)/100,
		math.Round(lng*100)/100,
		categorySlug,
	)
}

// ttl returns the appropriate cache TTL based on mode.
func (c *Cache) ttl() time.Duration {
	if c.devMode {
		return ttlDev
	}
	return ttlNormal
}

// GetFeed retrieves cached feed results. Returns (nil, false, nil) on a cache miss.
func (c *Cache) GetFeed(ctx context.Context, lat, lng float64, categorySlug string) ([]ServiceResult, bool, error) {
	key := feedCacheKey(lat, lng, categorySlug)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("search.Cache.GetFeed: %w", err)
	}

	var results []ServiceResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, false, fmt.Errorf("search.Cache.GetFeed unmarshal: %w", err)
	}
	return results, true, nil
}

// SetFeed stores feed results in Redis with the configured TTL.
func (c *Cache) SetFeed(ctx context.Context, lat, lng float64, categorySlug string, results []ServiceResult) error {
	key := feedCacheKey(lat, lng, categorySlug)
	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("search.Cache.SetFeed marshal: %w", err)
	}
	if err := c.rdb.Set(ctx, key, data, c.ttl()).Err(); err != nil {
		return fmt.Errorf("search.Cache.SetFeed: %w", err)
	}
	return nil
}

// InvalidateCellsNearBranch deletes all feed keys in the 3×3 grid of cells
// centred on the given branch location (lat ± 0.01, lng ± 0.01).
func (c *Cache) InvalidateCellsNearBranch(ctx context.Context, lat, lng float64) error {
	offsets := []float64{-0.01, 0, 0.01}
	for _, dlat := range offsets {
		for _, dlng := range offsets {
			cellLat := math.Round((lat+dlat)*100) / 100
			cellLng := math.Round((lng+dlng)*100) / 100
			pattern := fmt.Sprintf("feed:%.2f:%.2f:*", cellLat, cellLng)
			if err := c.deleteScan(ctx, pattern); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteScan uses SCAN to find all keys matching pattern and deletes them.
func (c *Cache) deleteScan(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("search.Cache.deleteScan SCAN: %w", err)
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("search.Cache.deleteScan DEL: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
