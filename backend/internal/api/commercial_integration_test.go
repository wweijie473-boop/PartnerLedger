package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
	"gorm.io/gorm"
)

func testProjectCommercial(t *testing.T, db *gorm.DB, request func(string, string, string, string, int, any),
	send func(string, string, string, string) *httptest.ResponseRecorder, path, dealer, vendor, customer,
	otherDealer, otherCustomer, otherVendor string) {
	t.Helper()
	var project channel.View
	request(http.MethodGet, path, dealer, "", 200, &project)
	key := func(n int) string { return fmt.Sprintf("dddddddd-0000-4000-8000-%012d", n) }
	body := func(n int, revision uint32, payload string) string {
		return fmt.Sprintf(`{"requestId":%q,"revision":%d,"payload":%s}`, key(n), revision, payload)
	}
	act := func(token, name string, n int, revision uint32, payload string, status int) {
		t.Helper()
		var out any
		if status == 200 {
			// Decode a fresh view because missing fields enforce the role's projection.
			project = channel.View{}
			out = &project
		}
		request(http.MethodPost, path+"/actions/"+name, token, body(n, revision, payload), status, out)
	}
	china := time.FixedZone("UTC+8", 8*3600)
	today := time.Now().In(china).Format(time.DateOnly)
	future := time.Now().In(china).AddDate(0, 0, 10).Format(time.DateOnly)
	draft := func(price, validUntil, note string) string {
		return fmt.Sprintf(`{"lines":[{"label":"ERP","quantity":3,"unitPrice":%q}],"validUntil":%q,"deliveryDays":10,"note":%q}`,
			price, validUntil, note)
	}

	start := project.Revision
	act(otherVendor, "channelQuote", 1, start, draft("125.05", future, "渠道内部说明"), 404)
	act(dealer, "channelQuote", 1, start, draft("125.05", future, "渠道内部说明"), 403)
	act(vendor, "channelQuote", 1, start, draft("125.051", future, "渠道内部说明"), 400)
	act(vendor, "channelQuote", 1, start, draft("125.05", "2020-01-01", "渠道内部说明"), 400)
	act(vendor, "channelQuote", 1, start, draft("125.05", today, "渠道内部说明"), 200)
	if project.Stage != "channel_wait" || len(*project.Channel) != 1 || (*project.Channel)[0].Total != "375.15" ||
		project.Sales != nil {
		t.Fatalf("unexpected channel quote: %+v", project)
	}
	firstChannel := (*project.Channel)[0].ID
	act(dealer, "salesQuote", 2, project.Revision, draft("150.01", future, "方案首版"), 200)
	firstSales := (*project.Sales)[0].ID
	quoteID := func(id string) string { return fmt.Sprintf(`{"quoteId":%q}`, id) }
	act(customer, "confirmSales", 3, project.Revision, quoteID(firstSales), 200)
	if project.Order != nil || project.Channel != nil || project.Inquiry != nil {
		t.Fatal("confirming a sale created an order or exposed the channel price")
	}
	if err := db.Exec("UPDATE pl_channel_quotes SET valid_until = '2020-01-01' WHERE id = ? AND project_id = ?", firstChannel, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	var renewal channel.ProjectWorkspacePage
	request(http.MethodGet, "/api/channel/v1/projects/workspace?scope=mine&stage=channel_wait", vendor, "", 200, &renewal)
	if renewal.Counts.Mine != 1 || len(renewal.Items) != 1 || renewal.Items[0].ID != project.ID ||
		!renewal.Items[0].IsTodo || renewal.Items[0].NextStep != "发送渠道报价" {
		t.Fatalf("expired channel quote missing from the manufacturer's projected work queue: %+v", renewal)
	}
	expired := send(http.MethodPost, path+"/actions/placeOrder", dealer, body(4, project.Revision, quoteID(firstSales)))
	if expired.Code != 409 || !strings.Contains(expired.Body.String(), "quote_expired") {
		t.Fatalf("expired channel quote could be ordered: %s", expired.Body.String())
	}
	row := fmt.Sprintf(`{"label":%q,"quantity":3,"unitPrice":"130.01"}`, strings.Repeat("项", 120))
	largeQuote := fmt.Sprintf(`{"lines":[%s],"validUntil":%q,"deliveryDays":10,"note":"渠道改版条款"}`,
		strings.Repeat(row+",", 99)+row, future)
	act(vendor, "channelQuote", 5, project.Revision, largeQuote, 200)
	currentChannel := (*project.Channel)[1].ID
	if len((*project.Channel)[1].Lines) != 100 {
		t.Fatal("large quote did not preserve every line")
	}
	var customerView channel.View
	request(http.MethodGet, path, customer, "", 200, &customerView)
	if customerView.Stage != "processing" || customerView.Channel != nil || (*customerView.Sales)[0].Status != "superseded" ||
		strings.Contains(fmt.Sprint(customerView.Timeline), "渠道改版条款") {
		t.Fatalf("upstream replacement failed to invalidate private sales confirmation: %+v", customerView)
	}
	act(dealer, "salesQuote", 6, project.Revision, draft("151.03", future, "对客保密条款"), 200)
	act(otherCustomer, "confirmSales", 7, project.Revision, quoteID((*project.Sales)[1].ID), 404)
	act(customer, "confirmSales", 7, project.Revision, quoteID(firstSales), 409)
	act(customer, "reviseSales", 8, project.Revision, `{"note":"客户私有改价意见"}`, 200)
	act(dealer, "salesQuote", 9, project.Revision, draft("149.99", future, "仅对客交付条款"), 200)
	finalSales := (*project.Sales)[2].ID
	act(customer, "confirmSales", 10, project.Revision, quoteID(finalSales), 200)
	confirmedRevision := project.Revision

	// Independent retries can arrive from different tabs before either response.
	act(otherDealer, "placeOrder", 11, confirmedRevision, quoteID(finalSales), 404)
	results := make(chan *httptest.ResponseRecorder, 2)
	for n := range 2 {
		go func() {
			results <- send(http.MethodPost, path+"/actions/placeOrder", dealer, body(11+n, confirmedRevision, quoteID(finalSales)))
		}()
	}
	for range 2 {
		w := <-results
		var envelope struct{ Data channel.View }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &envelope) != nil || envelope.Data.Order == nil {
			t.Fatalf("simultaneous order submission failed: %d %s", w.Code, w.Body.String())
		}
		if project.Order != nil && envelope.Data.Order.ID != project.Order.ID {
			t.Fatal("one confirmed version produced multiple orders")
		}
		project = envelope.Data
	}
	if project.Revision != confirmedRevision+1 || project.Order.ChannelQuoteID != currentChannel ||
		project.Order.SalesQuoteID != finalSales {
		t.Fatal("order failed to lock the expected quote versions")
	}
	act(dealer, "placeOrder", 11, confirmedRevision, quoteID(finalSales), 200)
	act(dealer, "placeOrder", 11, confirmedRevision, quoteID(firstSales), 409)
	var vendorView channel.View
	request(http.MethodGet, path, vendor, "", 200, &vendorView)
	if vendorView.Sales != nil || vendorView.Order.SalesQuoteID != "" ||
		strings.Contains(fmt.Sprint(vendorView.Timeline), "客户私有改价意见") || !slices.Contains(vendorView.Actions, "supply") {
		t.Fatalf("vendor order view exposes sales data or cannot supply: %+v", vendorView)
	}
	act(vendor, "supply", 13, project.Revision, `{"note":"已交接授权"}`, 200)
	act(dealer, "implement", 14, project.Revision, `{"note":"仅对下游的实施说明"}`, 200)
	act(customer, "rectify", 15, project.Revision, `{"note":"仅对下游的整改要求"}`, 200)
	act(dealer, "implement", 16, project.Revision, `{"note":"仅对下游的整改结果"}`, 200)
	act(customer, "accept", 17, project.Revision, `{"note":"仅对下游的验收结论"}`, 200)
	if project.Stage != "completed" || project.Order.ChannelQuoteID != "" || project.SupplyNote == "" || len(project.Actions) != 0 {
		t.Fatal("customer acceptance did not complete the expected workflow")
	}
	var vendorPage channel.Page
	request(http.MethodGet, "/api/channel/v1/projects?limit=100", vendor, "", 200, &vendorPage)
	if len(vendorPage.Items) != 1 {
		t.Fatalf("vendor result length = %d", len(vendorPage.Items))
	}
	vendorView = vendorPage.Items[0]
	history := fmt.Sprint(vendorView.Timeline)
	if vendorView.Stage != "completed" || vendorView.ImplementationNote != "" || vendorView.Sales != nil ||
		vendorView.Order.SalesQuoteID != "" || strings.Contains(history, "仅对下游") || !strings.Contains(history, "项目已验收") {
		t.Fatalf("unexpected vendor completion projection: %+v", vendorView)
	}
}
