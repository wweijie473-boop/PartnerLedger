package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func testPublicRegistration(t *testing.T, db *gorm.DB, cache *redis.Client, router http.Handler) {
	t.Helper()
	ctx := context.Background()
	stamp := uint64(time.Now().UnixNano())
	ip := fmt.Sprintf("2001:db8::%x:%x:%x:%x", stamp>>48, stamp>>32&65535, stamp>>16&65535, stamp&65535)
	t.Cleanup(func() { _ = cache.Del(ctx, "partnerledger:registration:"+ip).Err() })
	call := func(route, body string, status int, result any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/"+route, strings.NewReader(body))
		req.RemoteAddr = "[" + ip + "]:4321"
		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)
		if r.Code != status {
			t.Fatalf("%s: expected %d, got %d: %s", route, status, r.Code, r.Body.String())
		}
		if result != nil {
			var response struct{ Data json.RawMessage }
			if json.Unmarshal(r.Body.Bytes(), &response) != nil || json.Unmarshal(response.Data, result) != nil {
				t.Fatal("invalid registration response")
			}
		}
	}
	suffix := strconv.FormatUint(stamp, 36)
	const password = "register-fixture-32"
	for _, kind := range []string{data.Manufacturer, data.Reseller, data.Customer} {
		name := "integration_signup_" + strings.ToLower(kind) + "_" + suffix
		var org struct {
			ID uint64 `json:"organization_id,string"`
		}
		call("register", fmt.Sprintf(`{"username":%q,"password":%q,"organization_name":%q,"organization_kind":%q}`, name, password, name, kind), 201, &org)
		t.Cleanup(func() {
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("organization_id = ?", org.ID).Delete(&data.User{}).Error; err != nil {
					return err
				}
				return tx.Delete(&data.Organization{}, org.ID).Error
			}); err != nil {
				t.Errorf("clean up signup fixture: %v", err)
			}
		})
		var login struct {
			User  data.User
			Token string `json:"access_token"`
		}
		call("login", fmt.Sprintf(`{"username":%q,"password":%q}`, name, password), 200, &login)
		t.Cleanup(func() { _ = auth.New(db, cache).Logout(ctx, login.Token) })
		if org.ID == 0 || login.User.Organization.ID != org.ID || login.User.Organization.Kind != kind {
			t.Fatal("signup did not create the expected isolated organization")
		}
		duplicate := name + "_duplicate"
		call("register", fmt.Sprintf(`{"username":%q,"password":%q,"organization_name":%q,"organization_kind":%q}`, name, password, duplicate, kind), 409, nil)
		var count int64
		if err := db.Model(&data.Organization{}).Where("name = ?", duplicate).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("duplicate signup left an orphan organization: %v", err)
		}
	}
	call("register", `{"organization_id":"1","organization_kind":"RESELLER"}`, 400, nil)
	for i := 0; i < registrationAttempts-6; i++ {
		call("register", `{"username":"example","password":"register-fixture-32","organization_name":"Fixture","organization_kind":"ADMIN"}`, 400, nil)
	}
	call("register", `{"organization_kind":"CUSTOMER"}`, 429, nil)
}
