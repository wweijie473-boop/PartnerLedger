package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func bootstrapTestWorkspaces(ctx context.Context, sqlDB *sql.DB, args []string) error {
	flags := flag.NewFlagSet("bootstrap-test-workspaces", flag.ContinueOnError)
	prefix := flags.String("prefix", "test", "username prefix shared by the three test accounts")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("bootstrap-test-workspaces accepts flags only")
	}
	db, err := data.OpenGORM(sqlDB)
	if err != nil {
		return err
	}
	password := os.Getenv("PARTNERLEDGER_BOOTSTRAP_PASSWORD")
	type account struct {
		role, kind, organization string
		id                       uint64
	}
	dealer := account{role: "dealer", kind: data.Reseller, organization: "测试代理商"}
	vendor := account{role: "vendor", kind: data.Manufacturer, organization: "测试软件厂商"}
	customer := account{role: "customer", kind: data.Customer, organization: "测试企业客户"}
	accounts := []*account{&dealer, &vendor, &customer}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, account := range accounts {
			username := *prefix + "_" + account.role
			id, err := auth.CreateOrganization(ctx, tx, account.organization, account.kind, username, password)
			if err != nil {
				return fmt.Errorf("create %s account: %w", account.role, err)
			}
			account.id = id
		}
		return auth.LinkCustomer(ctx, tx, customer.id, dealer.id)
	})
	if err != nil {
		return err
	}
	for _, account := range accounts {
		fmt.Printf("%s -> /login/%s (organization %d)\n", *prefix+"_"+account.role, account.role, account.id)
	}
	return nil
}
