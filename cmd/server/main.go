package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	httpAdapter "github.com/jcrlabs/chat-back/internal/adapter/inbound/http"
	"github.com/jcrlabs/chat-back/internal/adapter/inbound/ws"
	"github.com/jcrlabs/chat-back/internal/adapter/outbound/postgres"
	redisAdapter "github.com/jcrlabs/chat-back/internal/adapter/outbound/redis"
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
		Addrs:        cfg.RedisAddrs,
		Password:     cfg.RedisPassword,
		ClusterSlots: clusterSlotsByHostname(cfg.RedisAddrs, cfg.RedisPassword),
	})
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}

	// ── JWT keys ───────────────────────────────────────────────────────────
	privKey := mustLoadPrivateKey(cfg.JWTPrivateKeyPath)
	pubKey := mustLoadPublicKey(cfg.JWTPublicKeyPath)

	// ── Adapters ───────────────────────────────────────────────────────────
	roomRepo := postgres.NewRoomRepo(pool)
	inviteRepo := postgres.NewInviteRepo(pool)
	messageRepo := postgres.NewMessageRepo(pool)
	userRepo := postgres.NewUserRepo(pool)
	friendRepo := postgres.NewFriendRepo(pool)
	presence := redisAdapter.NewPresence(rdb)
	pubsub := redisAdapter.NewPubSub(rdb)

	// ── Services ───────────────────────────────────────────────────────────
	roomSvc := app.NewRoomService(roomRepo, inviteRepo)
	msgSvc := app.NewMessageService(messageRepo)
	userSvc := app.NewUserService(userRepo)
	friendSvc := app.NewFriendService(friendRepo)
	presenceSvc := app.NewPresenceService(presence)
	chatSvc := app.NewChatService(msgSvc, pubsub)

	// ── WebSocket hub ──────────────────────────────────────────────────────
	hub := ws.NewHub(chatSvc, presenceSvc, roomSvc, pubsub)
	go hub.Run()

	// ── HTTP server ────────────────────────────────────────────────────────
	authMW := middleware.NewJWTMiddleware(pubKey)
	srv := httpAdapter.NewServer(cfg, pool, hub, roomSvc, msgSvc, presenceSvc, userSvc, friendSvc, authMW, privKey)

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

// clusterSlotsByHostname returns a ClusterSlots function that discovers the
// Redis cluster topology using hostnames instead of the pod IPs returned by
// CLUSTER SLOTS. This avoids stale-IP failures when pods are rescheduled.
func clusterSlotsByHostname(addrs []string, password string) func(context.Context) ([]redis.ClusterSlot, error) {
	return func(ctx context.Context) ([]redis.ClusterSlot, error) {
		var slots []redis.ClusterSlot
		for _, addr := range addrs {
			c := redis.NewClient(&redis.Options{Addr: addr, Password: password})
			nodes, err := c.ClusterNodes(ctx).Result()
			c.Close()
			if err != nil {
				continue
			}
			for _, line := range strings.Split(nodes, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || !strings.Contains(line, "myself") {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) < 9 {
					continue
				}
				for _, sr := range parts[8:] {
					if strings.HasPrefix(sr, "[") {
						continue // skip migrating/importing slots
					}
					var start, end int
					if idx := strings.Index(sr, "-"); idx != -1 {
						start, _ = strconv.Atoi(sr[:idx])
						end, _ = strconv.Atoi(sr[idx+1:])
					} else {
						start, _ = strconv.Atoi(sr)
						end = start
					}
					slots = append(slots, redis.ClusterSlot{
						Start: start,
						End:   end,
						Nodes: []redis.ClusterNode{{Addr: addr}},
					})
				}
				break
			}
		}
		if len(slots) == 0 {
			return nil, fmt.Errorf("redis: no cluster slots discovered from seeds %v", addrs)
		}
		return slots, nil
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
