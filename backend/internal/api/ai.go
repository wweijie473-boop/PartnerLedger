package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const maxRawAudioBytes = 3 << 20

func (s server) parseRequirement(c *gin.Context) {
	text, ok := readAIWorkspaceText(c)
	if !ok {
		return
	}
	requirement, err := s.parser.ParseRequirement(c.Request.Context(), text)
	aiWorkspaceResult(c, requirement, err)
}

func (s server) routeWorkspace(c *gin.Context) {
	text, ok := readAIWorkspaceText(c)
	if !ok {
		return
	}
	router, ok := s.parser.(interface {
		RouteWorkspace(context.Context, string) (ai.WorkspaceRoute, error)
	})
	if !ok {
		aiWorkspaceResult(c, nil, ai.ErrUnavailable)
		return
	}
	route, err := router.RouteWorkspace(c.Request.Context(), text)
	aiWorkspaceResult(c, route, err)
}

func (s server) chatWorkspace(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		Messages []ai.ChatMessage `json:"messages"`
	}
	if !decodeWithLimit(c, &input, 64<<10) {
		return
	}
	if len(input.Messages) == 0 || len(input.Messages) > 16 {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	for i := range input.Messages {
		role := strings.TrimSpace(input.Messages[i].Role)
		text := strings.TrimSpace(input.Messages[i].Content)
		if (role != "user" && role != "assistant") || text == "" || utf8.RuneCountInString(text) > 4000 {
			respond(c, http.StatusBadRequest, "invalid_request", nil)
			return
		}
		input.Messages[i].Role = role
		input.Messages[i].Content = text
	}
	if input.Messages[len(input.Messages)-1].Role != "user" {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	chatter, ok := s.parser.(interface {
		ChatWorkspace(context.Context, string, []ai.ChatMessage) (ai.ChatResponse, error)
	})
	if !ok {
		aiWorkspaceResult(c, nil, ai.ErrUnavailable)
		return
	}
	snapshot := s.buildDealerSnapshot(c.Request.Context(), actor)
	result, err := chatter.ChatWorkspace(c.Request.Context(), snapshot, input.Messages)
	aiWorkspaceResult(c, result, err)
}

func (s server) streamWorkspace(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		Messages []ai.ChatMessage `json:"messages"`
	}
	if !decodeWithLimit(c, &input, 64<<10) {
		return
	}
	if len(input.Messages) == 0 || len(input.Messages) > 16 {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	for i := range input.Messages {
		role := strings.TrimSpace(input.Messages[i].Role)
		text := strings.TrimSpace(input.Messages[i].Content)
		if (role != "user" && role != "assistant") || text == "" || utf8.RuneCountInString(text) > 4000 {
			respond(c, http.StatusBadRequest, "invalid_request", nil)
			return
		}
		input.Messages[i].Role = role
		input.Messages[i].Content = text
	}
	if input.Messages[len(input.Messages)-1].Role != "user" {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	streamer, ok := s.parser.(interface {
		StreamWorkspace(context.Context, string, []ai.ChatMessage, ai.ToolExecutor, ai.StreamCallbacks) error
	})
	if !ok {
		aiWorkspaceResult(c, nil, ai.ErrUnavailable)
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(event string, payload any) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, raw)
		c.Writer.Flush()
	}

	today := time.Now().In(chinaTime).Format(time.DateOnly)
	actorHeader := fmt.Sprintf("日期：%s | 代理商组织：%s | 当前经办账号：%s", today, actor.Organization.Name, actor.Username)

	err := streamer.StreamWorkspace(c.Request.Context(), actorHeader, input.Messages,
		func(toolCtx context.Context, name string, args json.RawMessage) (string, string, string, error) {
			return s.executeDealerReadTool(toolCtx, actor, name, args)
		},
		ai.StreamCallbacks{
			OnStatus: func(label string) {
				writeEvent("status", gin.H{"label": label})
			},
			OnRoute: func(route ai.WorkspaceRoute) {
				writeEvent("route", route)
			},
			OnDelta: func(delta string) {
				writeEvent("delta", gin.H{"text": delta})
			},
			OnDone: func(links []string, model string) {
				writeEvent("done", gin.H{"links": links, "model": model})
			},
		},
	)
	if err != nil {
		code := "model_response_error"
		if errors.Is(err, ai.ErrUnavailable) {
			code = "model_unavailable"
		}
		writeEvent("error", gin.H{"code": code})
	}
}

func (s server) buildDealerSnapshot(ctx context.Context, actor data.User) string {
	var b strings.Builder
	today := time.Now().In(chinaTime).Format(time.DateOnly)
	fmt.Fprintf(&b, "当前日期：%s | 代理商组织：%s（经办人：%s）\n", today, actor.Organization.Name, actor.Username)

	if s.db != nil {
		dealer := actor.OrganizationID
		customers := make([]operationsCustomer, 0)
		_ = s.db.WithContext(ctx).Raw(`
			SELECT * FROM (
				SELECT CONCAT('o', org.id) AS id, org.name, COALESCE(profile.contact, '') AS contact,
					COALESCE(profile.note, '') AS note, link.customer_id IS NOT NULL AS linked, FALSE AS prospect
				FROM (`+dealerCustomerOrganizations+`) known
				JOIN pl_organizations org ON org.id = known.customer_id
				LEFT JOIN pl_channel_links link ON link.customer_id = org.id AND link.reseller_id = ?
				LEFT JOIN pl_dealer_customer_profiles profile ON profile.customer_id = org.id AND profile.reseller_id = ?
				UNION ALL
				SELECT CONCAT('p', id), prospect_name, contact, note, FALSE, TRUE
				FROM pl_dealer_customer_profiles WHERE reseller_id = ? AND customer_id IS NULL
			) customers ORDER BY id LIMIT 40`,
			dealer, dealer, dealer, dealer, dealer).Scan(&customers).Error

		fmt.Fprintf(&b, "\n【1. 客户档案明细（共 %d 家）】\n", len(customers))
		for _, cust := range customers {
			tag := "直购客户"
			if cust.Linked {
				tag = "已关联签约客户"
			} else if cust.Prospect {
				tag = "潜在跟进客户"
			}
			fmt.Fprintf(&b, "- %s [%s] | 联系人：%s | 服务与需求备注：%s\n",
				cust.Name, tag, fallbackDash(cust.Contact), fallbackDash(cust.Note))
		}

		renewals := make([]renewalEntry, 0)
		if s.db.WithContext(ctx).Raw(`
			SELECT orders.id, orders.id AS order_id, client.name AS customer, orders.product_name, orders.product_version,
				orders.source, orders.quantity, orders.paid_at, DATE_FORMAT(orders.renewal_due_on, '%Y-%m-%d') AS due_at,
				COALESCE(followup.note, '') AS note, followup.contacted_at
			FROM pl_retail_orders orders JOIN pl_organizations client ON client.id = orders.customer_id
			LEFT JOIN pl_dealer_renewal_notes followup ON orders.id = followup.order_id
			WHERE orders.reseller_id = ? AND orders.renewal_due_on IS NOT NULL
			ORDER BY orders.renewal_due_on ASC LIMIT 25`, dealer).Scan(&renewals).Error == nil && len(renewals) > 0 {
			fmt.Fprintf(&b, "\n【2. 已购年度授权与续费台账（共 %d 笔）】\n", len(renewals))
			for _, r := range renewals {
				fmt.Fprintf(&b, "- 订单#%d | 客户：%s | 产品：%s %s × %d套 | 购买日：%s | 到期日：%s | 续费回访备注：%s\n",
					r.OrderID, r.Customer, r.ProductName, r.ProductVersion, r.Quantity,
					r.PaidAt.In(chinaTime).Format(time.DateOnly), r.DueAt, fallbackDash(r.Note))
			}
		}

		var totals operationsTotals
		if s.db.WithContext(ctx).Raw(`
			SELECT (SELECT COUNT(*) FROM (`+dealerCustomerOrganizations+`) known)
				+ (SELECT COUNT(*) FROM pl_dealer_customer_profiles WHERE reseller_id = ? AND customer_id IS NULL) AS customers,
				COUNT(*) AS orders, COUNT(CASE WHEN source = 'promotion' THEN 1 END) AS promotion_orders,
				COUNT(CASE WHEN renewal_due_on BETWEEN ? AND DATE_ADD(?, INTERVAL 30 DAY) THEN 1 END) AS renewals_30_days,
				COALESCE(SUM(CASE WHEN status = 'settlement' THEN `+retailCommissionCents+` ELSE 0 END), 0) AS pending_commission_cents,
				(SELECT COUNT(*) FROM pl_channel_service_tickets WHERE reseller_id = ? AND status <> 'closed') AS open_tickets
			FROM pl_retail_orders WHERE reseller_id = ?`,
			dealer, dealer, dealer, today, today, dealer, dealer).Scan(&totals).Error == nil {
			fmt.Fprintf(&b, "\n【3. 经营统计汇总】直购订单 %d 笔，待结佣金 %.2f 元，30天内到期续费 %d 笔，未关闭服务工单 %d 单。\n",
				totals.Orders, float64(totals.PendingCommissionCents)/100.0, totals.Renewals30Days, totals.OpenTickets)
		}
	}

	if page, err := s.channel.List(ctx, actor, 0, 20); err == nil {
		fmt.Fprintf(&b, "\n【4. 协作项目列表（共 %d 个）】\n", len(page.Items))
		for _, p := range page.Items {
			custName := ""
			if p.CustomerName != nil {
				custName = *p.CustomerName
			}
			todoTag := ""
			if p.IsTodo {
				todoTag = "【待我处理】"
			}
			fmt.Fprintf(&b, "- 项目#%s %s（客户：%s）| 阶段：%s %s | 下一步：%s\n",
				p.ID, p.Title, fallbackDash(custName), p.StageLabel, todoTag, p.NextStep)
		}
	}
	return b.String()
}

func fallbackDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未填写"
	}
	return strings.TrimSpace(value)
}

func (s server) transcribeAudio(c *gin.Context) {
	if principal(c).Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		Audio  string `json:"audio"`
		Format string `json:"format"`
	}
	if !decodeWithLimit(c, &input, 6<<20) {
		return
	}
	format := strings.ToLower(strings.TrimSpace(input.Format))
	cleaned := strings.TrimSpace(input.Audio)
	if format != "wav" || cleaned == "" {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil || len(raw) < 44 || len(raw) > maxRawAudioBytes ||
		!bytes.Equal(raw[:4], []byte("RIFF")) || !bytes.Equal(raw[8:12], []byte("WAVE")) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	transcriber, ok := s.parser.(interface {
		TranscribeAudio(context.Context, string, string) (string, error)
	})
	if !ok {
		aiWorkspaceResult(c, nil, ai.ErrUnavailable)
		return
	}
	text, err := transcriber.TranscribeAudio(c.Request.Context(), cleaned, format)
	aiWorkspaceResult(c, gin.H{"text": text}, err)
}

func readAIWorkspaceText(c *gin.Context) (string, bool) {
	if principal(c).Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return "", false
	}
	var input struct {
		Text string `json:"text"`
	}
	if !decode(c, &input) {
		return "", false
	}
	text := strings.TrimSpace(input.Text)
	if text == "" || utf8.RuneCountInString(text) > 2000 {
		respond(c, http.StatusBadRequest, "invalid_requirement", nil)
		return "", false
	}
	return text, true
}

func aiWorkspaceResult(c *gin.Context, result any, err error) {
	switch {
	case errors.Is(err, ai.ErrUnavailable):
		respond(c, http.StatusServiceUnavailable, "model_unavailable", nil)
	case err != nil:
		respond(c, http.StatusBadGateway, "model_response_error", nil)
	default:
		respond(c, http.StatusOK, "success", result)
	}
}
