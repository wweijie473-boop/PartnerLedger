package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/config"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func TestOrganizationIsolation(t *testing.T) {
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
	send := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	request := func(method, path, token, body string, wantStatus int, target any) {
		t.Helper()
		w := send(method, path, token, body)
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
	testPublicRegistration(t, db, cache, router)

	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	const password = "test-account-only-32"
	orgIDs := make(map[string]uint64)
	createSession := func(label, kind string) string {
		t.Helper()
		username := "integration_" + label + "_" + suffix
		id, err := auth.CreateOrganization(ctx, db, username, kind, username, password)
		if err != nil {
			t.Fatal(err)
		}
		orgIDs[label] = id
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

	customer := createSession("customer", data.Customer)
	otherCustomer := createSession("other_customer", data.Customer)
	dealer := createSession("dealer", data.Reseller)
	otherDealer := createSession("other_dealer", data.Reseller)
	customerID, dealerID := orgIDs["customer"], orgIDs["dealer"]
	t.Cleanup(func() {
		err := db.Transaction(func(tx *gorm.DB) error {
			for _, statement := range []string{
				"DELETE FROM pl_channel_requests WHERE project_id IN (SELECT id FROM pl_channel_projects WHERE customer_id = ?)",
				"DELETE FROM pl_channel_events WHERE project_id IN (SELECT id FROM pl_channel_projects WHERE customer_id = ?)",
				"DELETE FROM pl_channel_projects WHERE customer_id = ?",
				"DELETE FROM pl_channel_links WHERE customer_id = ?",
			} {
				if err := tx.Exec(statement, customerID).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Errorf("clean up projects: %v", err)
		}
	})

	for i := 0; i < 2; i++ {
		if err := auth.LinkCustomer(ctx, db, customerID, dealerID); err != nil {
			t.Fatal(err)
		}
	}
	const base = "/api/channel/v1/projects"
	var links struct{ Items []data.Organization }
	request(http.MethodGet, "/api/channel/v1/organization-links", customer, "", 200, &links)
	if len(links.Items) != 1 || links.Items[0].ID != dealerID {
		t.Fatalf("wrong linked reseller: %+v", links)
	}
	request(http.MethodGet, "/api/channel/v1/organization-links", first, "", 403, nil)

	create := `{"requestId":"aaaaaaaa-0000-4000-8000-000000000001","title":"Private project","requirement":"private customer text","dueDate":"2027-06-01"}`
	request(http.MethodPost, base, otherCustomer, create, 400, nil)
	request(http.MethodPost, base, otherDealer, fmt.Sprintf(`{"requestId":"aaaaaaaa-0000-4000-8000-000000000001","customerId":"%d","title":"Private project","requirement":"Notes"}`, customerID), 404, nil)
	var project, replay channel.View
	created := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() { created <- send(http.MethodPost, base, customer, create) }()
	}
	for _, target := range []*channel.View{&project, &replay} {
		w := <-created
		var response struct{ Data json.RawMessage }
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &response) != nil || json.Unmarshal(response.Data, target) != nil {
			t.Fatalf("concurrent creation failed: %d %s", w.Code, w.Body)
		}
	}
	if project.ID == "" || project.ID != replay.ID || replay.Revision != 1 || len(replay.Timeline) != 1 {
		t.Fatalf("incorrect creation replay: %+v", replay)
	}
	request(http.MethodPost, base, customer, strings.ReplaceAll(create, "Private project", "Changed"), 409, nil)
	path := base + "/" + project.ID
	for _, token := range []string{first, second, otherCustomer, otherDealer} {
		request(http.MethodGet, path, token, "", 404, nil)
	}
	act := func(token, action string, key int, revision uint32, payload string, status int, result any) {
		t.Helper()
		body := fmt.Sprintf(`{"requestId":"bbbbbbbb-0000-4000-8000-%012d","revision":%d,"payload":%s}`, key, revision, payload)
		request(http.MethodPost, path+"/actions/"+action, token, body, status, result)
	}
	submit := fmt.Sprintf(`{"product":"%d","summary":"upstream inquiry"}`, product.ID)
	act(dealer, "submitInquiry", 1, 1, submit, 200, &project)
	act(dealer, "submitInquiry", 1, 1, submit, 200, &replay)
	if replay.Revision != 2 || len(replay.Timeline) != 2 {
		t.Fatalf("inquiry replay changed state: %+v", replay)
	}
	request(http.MethodGet, path, second, "", 404, nil)
	var view map[string]any
	request(http.MethodGet, path, first, "", 200, &view)
	for _, forbidden := range []string{"customerName", "requirement", "dueDate", "salesQuotes"} {
		if _, ok := view[forbidden]; ok {
			t.Fatalf("manufacturer was sent %s", forbidden)
		}
	}
	if strings.Contains(fmt.Sprint(view), "private customer text") {
		t.Fatal("manufacturer timeline exposed original requirement")
	}
	act(dealer, "withdrawInquiry", 2, 1, "{}", 409, nil)
	act(dealer, "withdrawInquiry", 2, 2, "{}", 200, nil)
	request(http.MethodGet, path, first, "", 404, nil)
	act(dealer, "submitInquiry", 3, 3, submit, 200, nil)
	act(first, "flagGap", 4, 4, `{"note":"vendor-only gap"}`, 200, nil)
	act(dealer, "withdrawInquiry", 5, 5, "{}", 409, nil)
	act(first, "requestInfo", 6, 5, `{"note":"vendor-only question"}`, 200, nil)
	act(dealer, "requestInfo", 7, 6, `{"note":"customer-only question"}`, 200, nil)
	act(customer, "replyInfo", 8, 7, `{"note":"customer-only reply"}`, 200, nil)
	act(dealer, "replyInfo", 9, 8, `{"note":"cleaned reply for manufacturer"}`, 200, nil)
	var customerPage channel.Page
	request(http.MethodGet, base+"?limit=1", customer, "", 200, &customerPage)
	if len(customerPage.Items) != 1 || customerPage.Items[0].Inquiry != nil ||
		customerPage.Items[0].Channel != nil || customerPage.Items[0].Stage != "processing" ||
		strings.Contains(fmt.Sprint(customerPage.Items[0].Timeline), "vendor-only") {
		t.Fatalf("incorrect customer projection: %+v", customerPage)
	}
	var vendorPage channel.Page
	request(http.MethodGet, base, first, "", 200, &vendorPage)
	if len(vendorPage.Items) != 1 || len(vendorPage.Items[0].Timeline) != 4 ||
		strings.Contains(fmt.Sprint(vendorPage.Items[0].Timeline), "customer-only") {
		t.Fatalf("incorrect manufacturer projection: %+v", vendorPage)
	}
	var outsiderPage channel.Page
	request(http.MethodGet, base, otherDealer, "", 200, &outsiderPage)
	if len(outsiderPage.Items) != 0 {
		t.Fatalf("cross-organization list leaked: %+v", outsiderPage)
	}
	testDealerFieldwork(t, db, cache, request, project.ID, dealerID, dealer, otherDealer, customer, first, password)
}
