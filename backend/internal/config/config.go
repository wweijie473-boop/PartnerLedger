package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr         string
	MySQLDSN         string
	RedisAddr        string
	RedisPassword    string
	DeepSeekEndpoint string
	DeepSeekAPIKey   string
	DeepSeekModel    string
}

func Load() Config {
	return Config{
		HTTPAddr:         envOr("PARTNERLEDGER_HTTP_ADDR", "127.0.0.1:8080"),
		MySQLDSN:         os.Getenv("PARTNERLEDGER_MYSQL_DSN"),
		RedisAddr:        os.Getenv("PARTNERLEDGER_REDIS_ADDR"),
		RedisPassword:    os.Getenv("PARTNERLEDGER_REDIS_PASSWORD"),
		DeepSeekEndpoint: os.Getenv("PARTNERLEDGER_DEEPSEEK_ENDPOINT"),
		DeepSeekAPIKey:   os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekModel:    envOr("PARTNERLEDGER_DEEPSEEK_MODEL", "deepseek-flash"),
	}
}

func (c Config) ValidateMySQL() error {
	if c.MySQLDSN == "" {
		return errors.New("set PARTNERLEDGER_MYSQL_DSN")
	}
	return nil
}

func (c Config) Validate() error {
	if err := c.ValidateMySQL(); err != nil {
		return err
	}
	if c.RedisAddr == "" {
		return errors.New("set PARTNERLEDGER_REDIS_ADDR")
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
