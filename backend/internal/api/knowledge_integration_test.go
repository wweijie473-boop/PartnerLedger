package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/catalog"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

func testProductKnowledge(t *testing.T, request func(string, string, string, string, int, any),
	project string, product uint64, vendor, otherVendor, dealer, otherDealer, customer string) {
	t.Helper()
	path := fmt.Sprintf("/api/products/%d/knowledge", product)
	var profile catalog.Knowledge
	request(http.MethodGet, path, dealer, "", 200, &profile)
	if profile.Modules == nil || len(profile.Sources) != 0 {
		t.Fatalf("legacy product knowledge is not empty: %+v", profile)
	}
	const knowledge = `{"category":"ERP","modules":["财务管理","库存"],"deployment_modes":["private"],"license":{"minimum":5,"maximum":50},"sources":[{"title":"演示产品手册","reference":"测试版第 4 页","excerpt":"支持财务、库存及私有化部署。"}]}`
	request(http.MethodPut, path, otherVendor, knowledge, 404, nil)
	request(http.MethodPut, path, dealer, knowledge, 403, nil)
	request(http.MethodPut, path, vendor, strings.Replace(knowledge, `"maximum":50`, `"maximum":4`, 1), 400, nil)
	request(http.MethodPut, path, vendor, knowledge, 200, nil)
	request(http.MethodPut, path, vendor, knowledge, 200, nil)
	request(http.MethodGet, path, dealer, "", 200, &profile)
	if len(profile.Sources) != 1 || profile.License.Maximum == nil || *profile.License.Maximum != 50 {
		t.Fatalf("unexpected stored knowledge: %+v", profile)
	}
	request(http.MethodGet, path, customer, "", 404, nil)

	var hidden data.Product
	request(http.MethodPost, "/api/products", otherVendor, `{"name":"不可见产品","version":"v2","published":false}`, 201, &hidden)
	hiddenPath := fmt.Sprintf("/api/products/%d/knowledge", hidden.ID)
	request(http.MethodPut, hiddenPath, otherVendor, knowledge, 200, nil)
	request(http.MethodGet, hiddenPath, dealer, "", 404, nil)

	const matchPath = "/api/ai/match-products"
	const demand = `{"employee_count":120,"required_modules":["财务"],"deployment_mode":"私有化部署","license_count":null}`
	type matches struct{ Items []catalog.Match }
	var results matches
	request(http.MethodPost, matchPath, customer, demand, 403, nil)
	request(http.MethodPost, matchPath, dealer, demand, 200, &results)
	var own catalog.Match
	for _, m := range results.Items {
		if m.Product.ID == hidden.ID {
			t.Fatal("unpublished candidate leaked")
		}
		if m.Product.ID == product {
			own = m
		}
	}
	if own.Status != "needs_confirmation" || len(own.Sources) != 1 || len(own.Checks) != 3 ||
		own.Checks[0].Status != "met" || own.Checks[1].Status != "met" || own.Checks[2].Status != "unknown" {
		t.Fatalf("unverified seat count must remain unknown: %+v", own)
	}
	request(http.MethodPost, matchPath, dealer, strings.Replace(demand, `"license_count":null`, `"license_count":60`, 1), 200, &results)
	own = catalog.Match{}
	for _, item := range results.Items {
		if item.Product.ID == product {
			own = item
		}
	}
	if own.Status != "mismatch" || len(own.Checks) != 3 || own.Checks[2].Status != "missing" {
		t.Fatalf("license outside the recorded range: %+v", own)
	}

	const requestID = "eeeeeeee-0000-4000-8000-000000000001"
	makeDraft := func(id uint64, revision int, confirmed bool) string {
		return fmt.Sprintf(`{"requestId":%q,"revision":%d,"confirmed":%t,"productId":"%d","requirement":%s}`, requestID, revision, confirmed, id, demand)
	}
	draftPath := project + "/inquiry-draft"
	request(http.MethodPost, draftPath, otherDealer, makeDraft(product, 1, true), 404, nil)
	request(http.MethodPost, draftPath, dealer, makeDraft(product, 1, false), 400, nil)
	request(http.MethodPost, draftPath, dealer, makeDraft(product, 2, true), 409, nil)
	request(http.MethodPost, draftPath, dealer, makeDraft(hidden.ID, 1, true), 404, nil)
	type draftView struct{ ID, Summary string }
	var first, replay draftView
	request(http.MethodPost, draftPath, dealer, makeDraft(product, 1, true), 201, &first)
	request(http.MethodPost, draftPath, dealer, makeDraft(product, 1, true), 201, &replay)
	if first != replay || first.ID == "" || !strings.Contains(first.Summary, "员工人数：120") ||
		!strings.Contains(first.Summary, "授权席位：待确认") {
		t.Fatalf("invalid draft or replay: %+v", first)
	}
	request(http.MethodGet, draftPath, dealer, "", 200, &replay)
	request(http.MethodGet, draftPath, otherDealer, "", 404, nil)
	var stillDraft channel.View
	request(http.MethodGet, project, dealer, "", 200, &stillDraft)
	if stillDraft.Revision != 1 || stillDraft.Inquiry != nil {
		t.Fatal("preparing draft submitted an inquiry or changed the project revision")
	}
}
