package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func testDealerFieldwork(t *testing.T, db *gorm.DB, cache *redis.Client, request func(string, string, string, string, int, any),
	projectID string, dealerID uint64, dealer, otherDealer, customer, manufacturer, password string) {
	t.Helper()
	ctx := context.Background()
	var account data.User
	if err := db.Where("organization_id = ?", dealerID).Take(&account).Error; err != nil {
		t.Fatal(err)
	}
	colleague := data.User{OrganizationID: dealerID, Username: account.Username + "_peer", PasswordHash: account.PasswordHash}
	if err := db.Create(&colleague).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Delete(&colleague).Error; err != nil {
			t.Error(err)
		}
	})
	var peer struct {
		Token string `json:"access_token"`
	}
	request(http.MethodPost, "/api/auth/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, colleague.Username, password), 200, &peer)
	t.Cleanup(func() { _ = auth.New(db, cache).Logout(ctx, peer.Token) })
	t.Cleanup(func() {
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, query := range []string{
				"DELETE FROM pl_channel_visit_events WHERE visit_id IN (SELECT id FROM pl_channel_visits WHERE organization_id = ?)",
				"DELETE FROM pl_channel_visits WHERE organization_id = ?",
				"DELETE FROM pl_channel_partner_cases WHERE partner_id IN (SELECT id FROM pl_channel_partners WHERE organization_id = ?)",
				"DELETE FROM pl_channel_opportunity_reads WHERE opportunity_id IN (SELECT id FROM pl_channel_opportunities WHERE organization_id = ?)",
				"DELETE FROM pl_channel_opportunity_events WHERE opportunity_id IN (SELECT id FROM pl_channel_opportunities WHERE organization_id = ?)",
				"DELETE FROM pl_channel_opportunities WHERE organization_id = ?",
				"DELETE FROM pl_channel_partners WHERE organization_id = ?",
				"DELETE FROM pl_channel_requests WHERE organization_id = ? AND (operation LIKE 'opportunity:%' OR operation LIKE 'visit:%')",
			} {
				if err := tx.Exec(query, dealerID).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Errorf("clean up fieldwork fixtures: %v", err)
		}
	})
	const base = "/api/channel/v1/opportunities"
	const visits = "/api/channel/v1/visits"
	key := func(n int) string { return fmt.Sprintf("cccccccc-0000-4000-8000-%012d", n) }
	capture := fmt.Sprintf(`{"requestId":%q,"projectId":%q,"payload":{"note":"IDE 接口，联系人测试，电话13800000000，预算999","source":"ide"}}`, key(1), projectID)
	request(http.MethodGet, base+"/desk", customer, "", 403, nil)
	request(http.MethodGet, base+"/desk", manufacturer, "", 403, nil)
	request(http.MethodPost, base, otherDealer, capture, 404, nil)
	var item, replay channel.OpportunityView
	request(http.MethodPost, base, dealer, capture, 201, &item)
	request(http.MethodPost, base, dealer, capture, 201, &replay)
	if item.ID == "" || replay.ID != item.ID || len(item.Questions) == 0 || len(item.Matches) != 0 ||
		strings.Contains(item.Brief, "13800000000") || strings.Contains(item.Brief, "联系人测试") ||
		strings.Contains(item.Brief, "999") {
		t.Fatalf("incorrect capture, projection, or replay: %+v", item)
	}
	request(http.MethodPost, base, dealer, strings.Replace(capture, key(1), key(2), 1), 409, nil)
	action := func(token, name string, n int, revision uint32, payload string, status int, result any) {
		t.Helper()
		body := fmt.Sprintf(`{"requestId":%q,"revision":%d,"payload":%s}`, key(n), revision, payload)
		request(http.MethodPost, base+"/"+item.ID+"/actions/"+name, token, body, status, result)
	}
	const capability = `{"category":"integration","source":"erp","target":"mes"}`
	action(otherDealer, "clarify", 3, 1, capability, 404, nil)
	action(dealer, "clarify", 3, 1, capability, 200, &item)
	type partnerResult struct {
		ID string
	}
	partner := func(n int, name, evidence string) partnerResult {
		t.Helper()
		var p, replayPartner partnerResult
		body := fmt.Sprintf(`{"requestId":%q,"payload":{"name":%q,"owner":"测试引荐人","evidence":%q,"category":"integration","source":"erp","target":"mes"}}`, key(n), name, evidence)
		request(http.MethodPost, base+"/partners", dealer, body, 201, &p)
		request(http.MethodPost, base+"/partners", dealer, body, 201, &replayPartner)
		if replayPartner.ID != p.ID {
			t.Fatal("partner retry duplicated the team record")
		}
		return p
	}
	registered := partner(4, "登记团队", "")
	proven := partner(5, "有记录的团队", "测试合作案例")
	desk := func(token string) channel.OpportunityDesk {
		t.Helper()
		var result channel.OpportunityDesk
		request(http.MethodGet, base+"/desk", token, "", 200, &result)
		return result
	}
	own := desk(dealer)
	if len(own.Items) != 1 || len(own.Items[0].Matches) != 2 || own.Items[0].Matches[0].PartnerID != proven.ID ||
		len(own.Tasks) != 1 || own.Tasks[0].Action != "查看新伙伴依据" || len(own.ReverseMatches) != 2 {
		t.Fatalf("unexpected matches or tasks: %+v", own)
	}
	if isolated := desk(otherDealer); len(isolated.Items) != 0 || len(isolated.Partners) != 0 {
		t.Fatal("dealer desk exposed a different organization")
	}
	action(dealer, "acknowledge", 6, item.Revision, "{}", 200, &item)
	if len(desk(dealer).Tasks) != 0 || len(desk(peer.Token).Tasks) != 1 {
		t.Fatal("candidate read status was not isolated per member")
	}
	action(dealer, "follow", 14, item.Revision, `{"followUp":"2020-01-01","followReason":"customer"}`, 200, &item)
	own = desk(dealer)
	if len(own.Tasks) != 1 || !strings.Contains(own.Tasks[0].Reason, "客户约定答复") {
		t.Fatal("open opportunity did not generate the customer follow-up")
	}
	action(dealer, "rejectPartner", 7, item.Revision, fmt.Sprintf(`{"partnerId":%q,"reason":"测试冲突"}`, registered.ID), 200, &item)
	if len(item.Matches) != 1 || len(item.Rejections) != 1 {
		t.Fatal("rejected partner remained in the candidate list")
	}
	action(dealer, "contactReview", 8, item.Revision, fmt.Sprintf(`{"partnerId":%q,"checks":{"region":"blocked"}}`, proven.ID), 200, &item)
	intro := fmt.Sprintf(`{"partnerId":%q,"consent":true,"followUp":"2020-01-01","feeAmount":"1500.5","feeTerms":"验收后"}`, proven.ID)
	action(dealer, "introduce", 9, item.Revision, intro, 400, nil)
	action(dealer, "contactReview", 10, item.Revision, fmt.Sprintf(`{"partnerId":%q,"checks":{"region":"confirmed"}}`, proven.ID), 200, &item)
	action(dealer, "introduce", 9, item.Revision, strings.Replace(intro, "true", "false", 1), 400, nil)
	introRevision := item.Revision
	action(dealer, "introduce", 9, introRevision, intro, 200, &item)
	action(dealer, "introduce", 9, introRevision, intro, 200, &replay)
	if item.FeeAmount != "1500.50" || item.Stage != "introduced" || replay.Revision != item.Revision ||
		len(replay.History) != len(item.History) || len(desk(dealer).Tasks) != 1 {
		t.Fatal("invalid introduced state, precision, or idempotent replay")
	}
	action(dealer, "follow", 11, introRevision, `{"followUp":"2020-02-01"}`, 409, nil)
	action(dealer, "win", 12, item.Revision, `{"outcome":"测试成交","remember":true}`, 200, &item)
	own = desk(dealer)
	if len(own.Tasks) != 1 || own.Tasks[0].Action != "核对结清条件" ||
		!strings.Contains(own.Partners[0].Evidence, "非交付验收") {
		t.Fatal("win did not add authorized evidence or fee task")
	}
	action(dealer, "settle", 13, item.Revision, "{}", 200, &item)
	if len(desk(dealer).Tasks) != 0 {
		t.Fatal("settled fee still appears in task list")
	}

	var before, after channel.View
	const projectPath = "/api/channel/v1/projects/"
	request(http.MethodGet, projectPath+projectID, dealer, "", 200, &before)
	var visit channel.VisitView
	request(http.MethodPost, visits, dealer, fmt.Sprintf(`{"requestId":%q,"revision":0,"payload":{"text":"初记未关联拜访"}}`, key(20)), 201, &visit)
	path := visits + "/" + visit.ID
	request(http.MethodGet, path, peer.Token, "", 404, nil)
	request(http.MethodGet, path, otherDealer, "", 404, nil)
	request(http.MethodGet, visits, customer, "", 403, nil)
	var peerDrafts struct{ Items []channel.VisitView }
	request(http.MethodGet, visits, peer.Token, "", 200, &peerDrafts)
	if len(peerDrafts.Items) != 0 {
		t.Fatal("a team member's list exposed a private visit draft")
	}
	save := fmt.Sprintf(`{"requestId":%q,"revision":1,"payload":{"projectId":%q,"text":"内部记录","main":"不自动覆盖主单","additional":"另外需要培训","reviewed":true}}`, key(21), projectID)
	request(http.MethodPut, path, dealer, save, 200, &visit)
	confirmation := fmt.Sprintf(`{"requestId":%q,"revision":%d,"confirmed":true,"createOpportunity":true}`, key(22), visit.Revision)
	request(http.MethodPost, path+"/confirm", dealer, strings.Replace(confirmation, "true", "false", 1), 400, nil)
	request(http.MethodPost, path+"/confirm", dealer, confirmation, 200, &visit)
	var visitReplay channel.VisitView
	request(http.MethodPost, path+"/confirm", dealer, confirmation, 200, &visitReplay)
	if visit.OpportunityID == "" || visitReplay.OpportunityID != visit.OpportunityID || len(desk(dealer).Items) != 2 {
		t.Fatal("visit confirmation did not return one durable, linked opportunity")
	}
	request(http.MethodGet, projectPath+projectID, dealer, "", 200, &after)
	if before.Revision != after.Revision || *before.Requirement != *after.Requirement {
		t.Fatal("confirming a visit changed the main procurement project")
	}
	deletion := fmt.Sprintf(`{"requestId":%q,"revision":%d}`, key(23), visit.Revision)
	request(http.MethodDelete, path, dealer, deletion, 200, nil)
	request(http.MethodDelete, path, dealer, deletion, 200, nil)
	request(http.MethodGet, path, dealer, "", 404, nil)
	if len(desk(dealer).Items) != 2 {
		t.Fatal("removing a visit also removed a confirmed opportunity")
	}
}
