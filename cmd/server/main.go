package main

import (
	"context"
	"crypto/rsa"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	httpAdapter "github.com/jcrlabs/chat-back/internal/adapter/inbound/http"
	redisAdapter "github.com/jcrlabs/chat-back/internal/adapter/outbound/redis"
	"github.com/jcrlabs/chat-back/internal/adapter/outbound/postgres"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/config"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

func main() {
	cfg := config.Load()

	// ── PostgreSQL ──────────────────────────────────────────────────────────
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	// ── Redis Cluster (RedKey Operator) ────────────────────────────────────
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    cfg.RedisAddrs,
		Password: cfg.RedisPassword,
	})
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}

	// ── JWT keys ───────────────────────────────────────────────────────────
	privKey := mustLoadPrivateKey(cfg.JWTPrivateKeyPath)
	pubKey := mustLoadPublicKey(cfg.JWTPublicKeyPath)

	// ── Adapters ───────────────────────────────────────────────────────────
	roomRepo    := postgres.NewRoomRepo(pool)
	messageRepo := postgres.NewMessageRepo(pool)
	presence    := redisAdapter.NewPresence(rdb)
	pubsub      := redisAdapter.NewPubSub(rdb)

	// ── Services ───────────────────────────────────────────────────────────
	roomSvc    := app.NewRoomService(roomRepo)
	msgSvc     := app.NewMessageService(messageRepo)
	presenceSvc := app.NewPresenceService(presence)
	chatSvc    := app.NewChatService(msgSvc, pubsub)

	// ── WebSocket hub ──────────────────────────────────────────────────────
	hub := ws.NewHub(chatSvc, presenceSvc, pubsub)
	go hub.Run()

	// ── HTTP server ────────────────────────────────────────────────────────
	authMW := middleware.NewJWTMiddleware(pubKey)
	srv := httpAdapter.NewServer(cfg, pool, hub, roomSvc, msgSvc, presenceSvc, authMW, privKey)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // 0 = no timeout (needed for WebSocket upgrades)
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func mustLoadPrivateKey(path string) *rsa.PrivateKey {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("private key: %v", err)
	}
	key, err := middleware.ParseRSAPrivateKey(data)
	if err != nil {
		log.Fatalf("private key parse: %v", err)
	}
	return key
}

func mustLoadPublicKey(path string) *rsa.PublicKey {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("public key: %v", err)
	}
	key, err := middleware.ParseRSAPublicKey(data)
	if err != nil {
		log.Fatalf("public key parse: %v", err)
	}
	return key
}
