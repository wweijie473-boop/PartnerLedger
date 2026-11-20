package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
)

func testProjectWorkspace(t *testing.T, request func(string, string, string, string, int, any), completedID, dealer, customer, vendor, outsider string) {
	t.Helper()
	const base = "/api/channel/v1/projects"
	late, early, noDate := channel.View{}, channel.View{}, channel.View{}
	for i, item := range []struct {
		title, date string
		view        *channel.View
	}{
		{"ERP_批次% 晚", "2027-09-01", &late},
		{"ERP_批次% 早", "2027-05-01", &early},
		{"ERPx批次 无日期", "", &noDate},
	} {
		body := fmt.Sprintf(`{"requestId":"eeeeeeee-0000-4000-8000-%012d","title":%q,"requirement":"private-only-fixture","dueDate":%q}`, i, item.title, item.date)
		request(http.MethodPost, base, customer, body, 201, item.view)
	}
	read := func(token, query string) channel.ProjectWorkspacePage {
		t.Helper()
		var page channel.ProjectWorkspacePage
		request(http.MethodGet, base+"/workspace?"+query, token, "", 200, &page)
		return page
	}
	first := read(dealer, "scope=mine&limit=1")
	if first.Counts != (channel.ProjectCounts{All: 4, Mine: 3}) || len(first.Items) != 1 || first.Items[0].ID != early.ID || first.NextCursor == nil {
		t.Fatalf("expected the earliest dealer task and unfiltered counts: %+v", first)
	}
	second := read(dealer, "scope=mine&limit=2&after="+url.QueryEscape(*first.NextCursor))
	if second.NextCursor != nil || len(second.Items) != 2 || second.Items[0].ID != late.ID || second.Items[1].ID != noDate.ID {
		t.Fatalf("unexpected task page or due date ordering: %+v", second)
	}
	filtered := read(dealer, "scope=all&stage=requested&q="+url.QueryEscape("ERP_批次%"))
	if len(filtered.Items) != 2 || filtered.Counts != first.Counts || len(filtered.Stages) != 2 {
		t.Fatalf("literal search or stage filtering returned unexpected records: %+v", filtered)
	}
	customerPage := read(customer, "scope=partners")
	if len(customerPage.Items) != 3 || customerPage.Counts != (channel.ProjectCounts{All: 4, Partners: 3}) {
		t.Fatalf("customer partner queue incorrectly included a completed project: %+v", customerPage)
	}
	var vendorPage struct{ Items []map[string]any }
	request(http.MethodGet, base+"/workspace", vendor, "", 200, &vendorPage)
	if len(vendorPage.Items) != 1 || vendorPage.Items[0]["id"] != completedID {
		t.Fatalf("manufacturer list includes rows outside its inquiries: %+v", vendorPage)
	}
	for _, field := range []string{"customerName", "dueDate", "requirement", "inquiry", "channelQuotes", "salesQuotes", "timeline"} {
		if _, ok := vendorPage.Items[0][field]; ok {
			t.Fatalf("manufacturer list sent an unnecessary or restricted field: %s", field)
		}
	}
	if page := read(outsider, "scope=all"); len(page.Items) != 0 || len(page.Stages) != 0 || page.Counts.All != 0 {
		t.Fatalf("workspace totals crossed organizations: %+v", page)
	}
	for _, query := range []string{"scope=unknown", "stage=unknown", "scope=mine&after=bad", "limit=101", "q=" + strings.Repeat("a", 121)} {
		request(http.MethodGet, base+"/workspace?"+query, dealer, "", 400, nil)
	}
}
