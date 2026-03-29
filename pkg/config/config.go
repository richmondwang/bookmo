package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	AppEnv string
	Port   string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// JWT
	JWTSecret string

	// PayMongo
	PayMongoSecretKey      string
	PayMongoWebhookSecret  string

	// Firebase / APNs
	FCMServerKey  string
	APNSKeyID     string
	APNSTeamID    string
	APNSBundleID  string

	// Storage
	S3Bucket string
	S3Region string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env is optional; env vars take precedence

	cfg := &Config{
		AppEnv:                os.Getenv("APP_ENV"),
		Port:                  getEnvOrDefault("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		PayMongoSecretKey:     os.Getenv("PAYMONGO_SECRET_KEY"),
		PayMongoWebhookSecret: os.Getenv("PAYMONGO_WEBHOOK_SECRET"),
		FCMServerKey:          os.Getenv("FCM_SERVER_KEY"),
		APNSKeyID:             os.Getenv("APNS_KEY_ID"),
		APNSTeamID:            os.Getenv("APNS_TEAM_ID"),
		APNSBundleID:          os.Getenv("APNS_BUNDLE_ID"),
		S3Bucket:              os.Getenv("S3_BUCKET"),
		S3Region:              getEnvOrDefault("S3_REGION", "ap-southeast-1"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
