package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/richmondwang/bookmo/pkg/config"
	"github.com/richmondwang/bookmo/pkg/db"
	redispkg "github.com/richmondwang/bookmo/pkg/redis"
)

func Run(cfg *config.Config) error {
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("server: db: %w", err)
	}
	defer pool.Close()

	rdb, err := redispkg.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("server: redis: %w", err)
	}
	defer rdb.Close()

	_ = pool
	_ = rdb

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	// TODO: register module route groups here
	// v1 := r.Group("/v1")
	// authHandler.RegisterRoutes(v1)
	// ...

	return r.Run(":" + cfg.Port)
}
