package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/api"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/config"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	if command == "help" || command == "-h" || command == "--help" {
		fmt.Println("usage: partnerledger [serve | check | migrate | bootstrap-org]")
		return nil
	}
	if command != "serve" && command != "check" && command != "migrate" && command != "bootstrap-org" {
		return errors.New("expected serve, check, migrate or bootstrap-org")
	}
	if len(args) > 0 && command != "bootstrap-org" {
		return fmt.Errorf("%s does not accept positional arguments", command)
	}

	cfg := config.Load()
	if err := cfg.ValidateMySQL(); err != nil {
		return err
	}
	if command == "serve" || command == "check" {
		if err := cfg.Validate(); err != nil {
			return err
		}
	}
	sqlDB, err := data.OpenSQL(cfg.MySQLDSN, command == "migrate")
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if command == "migrate" {
		if err := sqlDB.PingContext(ctx); err != nil {
			return fmt.Errorf("MySQL connection failed: %w", err)
		}
		if err := data.Migrate(sqlDB); err != nil {
			return fmt.Errorf("migrate MySQL: %w", err)
		}
		fmt.Println("MySQL migrations are up to date")
		return nil
	}
	if command == "bootstrap-org" {
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		kind := flags.String("kind", "", "MANUFACTURER or RESELLER")
		name := flags.String("name", "", "organization display name")
		username := flags.String("username", "", "first account username")
		if err := flags.Parse(args); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("bootstrap-org accepts flags only")
		}
		db, err := data.OpenGORM(sqlDB)
		if err != nil {
			return err
		}
		password := os.Getenv("PARTNERLEDGER_BOOTSTRAP_PASSWORD")
		if err := auth.CreateOrganization(ctx, db, *name, *kind, *username, password); err != nil {
			return fmt.Errorf("create organization and account: %w", err)
		}
		fmt.Println("Organization and first account created")
		return nil
	}

	cache := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DialTimeout:  4 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	defer cache.Close()
	if command == "check" {
		var version string
		if err := sqlDB.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
			return fmt.Errorf("MySQL connection failed: %w", err)
		}
		fmt.Printf("MySQL authenticated; server version %s\n", version)
		if err := cache.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("Redis connection failed: %w", err)
		}
		fmt.Println("Redis PING succeeded")
		return nil
	}

	db, err := data.OpenGORM(sqlDB)
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	parser := ai.NewDeepSeek(cfg.DeepSeekAPIKey, cfg.DeepSeekModel)
	return serve(cfg.HTTPAddr, api.New(db, sqlDB, cache, parser))
}

func serve(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Printf("PartnerLedger API address %s", addr)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		timeout, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return server.Shutdown(timeout)
	}
}
