package config

import (
	"os"
	"strings"
)

type Config struct {
	Port string

	DatabaseURL string

	RedisAddrs    []string
	RedisPassword string

	JWTPublicKeyPath  string
	JWTPrivateKeyPath string

	AllowedOrigins []string
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       mustEnv("DATABASE_URL"),
		RedisAddrs:        splitEnv("REDIS_ADDRS", "chat-redis-0.chat-redis.chat.svc.cluster.local:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		JWTPublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", ".keys/public.pem"),
		JWTPrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", ".keys/private.pem"),
		AllowedOrigins:    splitEnv("ALLOWED_ORIGINS", "https://chat.jcrlabs.net"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env var not set: " + key)
	}
	return v
}

func splitEnv(key, fallback string) []string {
	v := getEnv(key, fallback)
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}
