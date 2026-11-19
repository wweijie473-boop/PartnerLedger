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
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/tables"
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
		fmt.Println("usage: partnerledger [serve | check | migrate | tables-migrate | bootstrap-org | link-customer]")
		return nil
	}
	if command != "serve" && command != "check" && command != "migrate" && command != "tables-migrate" && command != "bootstrap-org" && command != "link-customer" {
		return errors.New("expected serve, check, migrate, tables-migrate, bootstrap-org or link-customer")
	}
	if len(args) > 0 && command != "bootstrap-org" && command != "link-customer" {
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
	sqlDB, err := data.OpenSQL(cfg.MySQLDSN, command == "migrate" || command == "tables-migrate")
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if command == "migrate" || command == "tables-migrate" {
		if err := sqlDB.PingContext(ctx); err != nil {
			return fmt.Errorf("MySQL connection failed: %w", err)
		}
		migrate := data.Migrate
		if command == "tables-migrate" {
			migrate = tables.Migrate
		}
		if err := migrate(sqlDB); err != nil {
			return fmt.Errorf("migrate MySQL: %w", err)
		}
		fmt.Println("MySQL migrations are up to date")
		return nil
	}
	if command == "bootstrap-org" {
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		kind := flags.String("kind", "", "MANUFACTURER, RESELLER or CUSTOMER")
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
		id, err := auth.CreateOrganization(ctx, db, *name, *kind, *username, password)
		if err != nil {
			return fmt.Errorf("create organization and account: %w", err)
		}
		fmt.Printf("Organization %d and first account created\n", id)
		return nil
	}
	if command == "link-customer" {
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		customer := flags.Uint64("customer", 0, "customer organization ID")
		reseller := flags.Uint64("reseller", 0, "reseller organization ID")
		if err := flags.Parse(args); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("link-customer accepts flags only")
		}
		db, err := data.OpenGORM(sqlDB)
		if err != nil {
			return err
		}
		if err := auth.LinkCustomer(ctx, db, *customer, *reseller); err != nil {
			return fmt.Errorf("link organizations: %w", err)
		}
		fmt.Println("Customer and reseller linked")
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
