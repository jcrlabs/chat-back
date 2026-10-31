package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

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

	// ── Demo user seed ─────────────────────────────────────────────────────
	seedDemoUser(context.Background(), pool, cfg.DemoUserEmail, cfg.DemoUserPassword)
	seedDemoData(context.Background(), pool, cfg.DemoUserEmail)

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

func seedDemoUser(ctx context.Context, pool *pgxpool.Pool, email, password string) {
	if password == "" {
		log.Println("DEMO_USER_PASSWORD not set — demo user not seeded")
		return
	}
	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count)
	if count > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Printf("seed demo user: hash error: %v", err)
		return
	}
	tag := "0000"
	for range 20 {
		t := fmt.Sprintf("%04d", rand.IntN(9999)+1)
		var exists bool
		_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE tag = $1)`, t).Scan(&exists)
		if !exists {
			tag = t
			break
		}
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password, tag) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (email) DO NOTHING`,
		uuid.New(), "demo", email, string(hash), tag,
	)
	if err != nil {
		log.Printf("seed demo user: %v", err)
		return
	}
	log.Printf("demo user seeded: %s", email)
}

func seedDemoData(ctx context.Context, pool *pgxpool.Pool, demoEmail string) {
	// Resolve demo user ID
	var demoID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, demoEmail).Scan(&demoID); err != nil {
		log.Printf("seedDemoData: demo user not found, skipping: %v", err)
		return
	}

	// Seed demofriend user
	friendEmail := "demofriend@jcrlabs.net"
	var friendID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, friendEmail).Scan(&friendID); err != nil {
		// Create demofriend
		friendID = uuid.New()
		tag := "demo"
		_, err = pool.Exec(ctx,
			`INSERT INTO users (id, username, email, password, tag) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (email) DO NOTHING`,
			friendID, "demofriend", friendEmail, "$2a$12$disabled", tag,
		)
		if err != nil {
			log.Printf("seedDemoData: create demofriend: %v", err)
			return
		}
		log.Println("seedDemoData: demofriend user seeded")
	}

	// Seed accepted friendship
	_, err := pool.Exec(ctx,
		`INSERT INTO friendships (requester_id, addressee_id, status)
		 VALUES ($1, $2, 'accepted')
		 ON CONFLICT (requester_id, addressee_id) DO NOTHING`,
		demoID, friendID,
	)
	if err != nil {
		log.Printf("seedDemoData: friendship: %v", err)
	}

	// Seed public text room "demotexto"
	var exists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rooms WHERE name = $1 AND type = 'public')`, "demotexto").Scan(&exists)
	if !exists {
		_, err = pool.Exec(ctx,
			`INSERT INTO rooms (id, name, type, owner_id) VALUES ($1, $2, 'public', $3)`,
			uuid.New(), "demotexto", demoID,
		)
		if err != nil {
			log.Printf("seedDemoData: create demotexto room: %v", err)
		} else {
			log.Println("seedDemoData: demotexto room seeded")
		}
	}

	// Seed voice room "demo voice chat"
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rooms WHERE name = $1 AND type = 'voice')`, "demo voice chat").Scan(&exists)
	if !exists {
		_, err = pool.Exec(ctx,
			`INSERT INTO rooms (id, name, type, owner_id) VALUES ($1, $2, 'voice', $3)`,
			uuid.New(), "demo voice chat", demoID,
		)
		if err != nil {
			log.Printf("seedDemoData: create voice room: %v", err)
		} else {
			log.Println("seedDemoData: demo voice chat room seeded")
		}
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
