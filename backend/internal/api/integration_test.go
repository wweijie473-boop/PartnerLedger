package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/config"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func TestCatalogOrganizationIsolation(t *testing.T) {
	if os.Getenv("PARTNERLEDGER_INTEGRATION_TEST") != "1" {
		t.Skip("set PARTNERLEDGER_INTEGRATION_TEST=1 to test an initialized development database")
	}
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := data.OpenSQL(cfg.MySQLDSN, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := data.OpenGORM(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	cache := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
	t.Cleanup(func() { _ = cache.Close() })
	router := New(db, sqlDB, cache, ai.NewDeepSeek("", ""))
	request := func(method, path, token, body string, wantStatus int, target any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != wantStatus {
			t.Fatalf("%s %s: want %d, got %d: %s", method, path, wantStatus, w.Code, w.Body.String())
		}
		if target != nil {
			var envelope struct {
				Data json.RawMessage
			}
			if json.Unmarshal(w.Body.Bytes(), &envelope) != nil || json.Unmarshal(envelope.Data, target) != nil {
				t.Fatalf("invalid response envelope for %s", path)
			}
		}
	}
	request(http.MethodGet, "/readyz", "", "", 200, nil)
	request(http.MethodGet, "/api/products", "", "", 401, nil)

	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	const password = "test-account-only-32"
	createSession := func(label, kind string) string {
		t.Helper()
		username := "integration_" + label + "_" + suffix
		if err := auth.CreateOrganization(ctx, db, username, kind, username, password); err != nil {
			t.Fatal(err)
		}
		var user data.User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("manufacturer_id = ?", user.OrganizationID).Delete(&data.Product{}).Error; err != nil {
					return err
				}
				if err := tx.Delete(&data.User{}, user.ID).Error; err != nil {
					return err
				}
				return tx.Delete(&data.Organization{}, user.OrganizationID).Error
			}); err != nil {
				t.Errorf("clean up integration fixture: %v", err)
			}
		})
		input, _ := json.Marshal(map[string]string{"username": username, "password": password})
		var result struct {
			Token string `json:"access_token"`
		}
		request(http.MethodPost, "/api/auth/login", "", string(input), 200, &result)
		t.Cleanup(func() {
			if err := auth.New(db, cache).Logout(ctx, result.Token); err != nil {
				t.Errorf("clean up integration session: %v", err)
			}
		})
		return result.Token
	}
	first := createSession("first", data.Manufacturer)
	second := createSession("second", data.Manufacturer)
	reseller := createSession("reseller", data.Reseller)

	const draft = `{"name":"ERP integration","version":"v1","published":false}`
	const published = `{"name":"ERP integration","version":"v1","published":true}`
	var product data.Product
	request(http.MethodPost, "/api/products", first, draft, 201, &product)
	productPath := "/api/products/" + strconv.FormatUint(product.ID, 10)
	request(http.MethodGet, productPath, reseller, "", 404, nil)
	request(http.MethodGet, productPath, second, "", 404, nil)
	request(http.MethodPut, productPath, second, published, 404, nil)
	request(http.MethodPut, productPath, first, published, 200, nil)
	request(http.MethodPut, productPath, first, published, 200, nil)
	request(http.MethodGet, productPath, reseller, "", 200, nil)
	request(http.MethodPut, productPath, reseller, draft, 403, nil)
	request(http.MethodPost, "/api/ai/parse-requirement", reseller, `{"text":"我们需要ERP"}`, 503, nil)
	if os.Getenv("PARTNERLEDGER_LIVE_MODEL_TEST") == "1" {
		if cfg.DeepSeekAPIKey == "" {
			t.Fatal("DEEPSEEK_API_KEY is required for the live model test")
		}
		router = New(db, sqlDB, cache, ai.NewDeepSeek(cfg.DeepSeekAPIKey, cfg.DeepSeekModel))
		var extraction ai.Requirement
		request(http.MethodPost, "/api/ai/parse-requirement", reseller,
			`{"text":"我们是一家120人的制造公司，需要ERP财务和库存模块，希望私有化部署，授权数量待确认。"}`,
			200, &extraction)
		if extraction.EmployeeCount == nil || *extraction.EmployeeCount != 120 || extraction.LicenseCount != nil ||
			len(extraction.RequiredModules) < 2 || !extraction.NeedUserConfirmation {
			t.Fatal("the live model did not extract the sample requirement correctly")
		}
		t.Log("DeepSeek accepted the authenticated API flow; employee count preserved and license count marked unknown")
	}
	request(http.MethodPost, "/api/auth/logout", reseller, "", 200, nil)
	request(http.MethodGet, "/api/org/current", reseller, "", 401, nil)
}
