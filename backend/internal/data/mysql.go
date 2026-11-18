package data

import (
	"database/sql"
	"errors"
	"time"

	driver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func OpenSQL(dsn string, multiStatements bool) (*sql.DB, error) {
	cfg, err := driver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		return nil, errors.New("PARTNERLEDGER_MYSQL_DSN must be a valid DSN with an explicit database name")
	}
	cfg.MultiStatements = multiStatements
	cfg.ClientFoundRows = true
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 10 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	connector, err := driver.NewConnector(cfg)
	if err != nil {
		return nil, errors.New("invalid MySQL connection settings")
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)
	return db, nil
}

func OpenGORM(db *sql.DB) (*gorm.DB, error) {
	return gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing: true,
		NamingStrategy:       schema.NamingStrategy{TablePrefix: "pl_"},
		Logger:               logger.Default.LogMode(logger.Silent),
		TranslateError:       true,
	})
}
