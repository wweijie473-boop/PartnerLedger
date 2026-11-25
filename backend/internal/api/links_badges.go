package api

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const (
	linkInviteTTL       = 7 * 24 * time.Hour
	linkInviteKeyPrefix = "pl:link-invite:code:"
	linkInviteOrgPrefix = "pl:link-invite:org:"
	inviteAlphabet      = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

func generateInviteCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("PL-")
	for _, v := range buf {
		b.WriteByte(inviteAlphabet[int(v)%len(inviteAlphabet)])
	}
	return b.String(), nil
}

func normalizeInviteCode(raw string) string {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if len(code) == 8 && !strings.HasPrefix(code, "PL-") {
		code = "PL-" + code
	}
	return code
}

func validInviteCodeFormat(code string) bool {
	if len(code) != 11 || !strings.HasPrefix(code, "PL-") {
		return false
	}
	for i := 3; i < len(code); i++ {
		if !strings.ContainsRune(inviteAlphabet, rune(code[i])) {
			return false
		}
	}
	return true
}

func (s server) createLinkInvite(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	if s.cache == nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	ctx := c.Request.Context()
	orgKey := linkInviteOrgPrefix + strconv.FormatUint(actor.OrganizationID, 10)
	refresh := c.Query("refresh") == "1"

	if !refresh {
		if existing, err := s.cache.Get(ctx, orgKey).Result(); err == nil && validInviteCodeFormat(existing) {
			if storedOrg, err2 := s.cache.Get(ctx, linkInviteKeyPrefix+existing).Result(); err2 == nil && storedOrg == strconv.FormatUint(actor.OrganizationID, 10) {
				_ = s.cache.Expire(ctx, orgKey, linkInviteTTL).Err()
				_ = s.cache.Expire(ctx, linkInviteKeyPrefix+existing, linkInviteTTL).Err()
				respond(c, http.StatusOK, "success", gin.H{
					"code":            existing,
					"reseller_id":     strconv.FormatUint(actor.OrganizationID, 10),
					"reseller_name":   actor.Organization.Name,
					"expires_in_days": 7,
				})
				return
			}
		}
	} else {
		if oldCode, err := s.cache.Get(ctx, orgKey).Result(); err == nil && oldCode != "" {
			_ = s.cache.Del(ctx, linkInviteKeyPrefix+oldCode).Err()
		}
	}

	code, err := generateInviteCode()
	if err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	orgIDStr := strconv.FormatUint(actor.OrganizationID, 10)
	pipe := s.cache.Pipeline()
	pipe.Set(ctx, linkInviteKeyPrefix+code, orgIDStr, linkInviteTTL)
	pipe.Set(ctx, orgKey, code, linkInviteTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	respond(c, http.StatusOK, "success", gin.H{
		"code":            code,
		"reseller_id":     orgIDStr,
		"reseller_name":   actor.Organization.Name,
		"expires_in_days": 7,
	})
}

func (s server) previewLinkInvite(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Customer {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	code := normalizeInviteCode(c.Query("code"))
	if !validInviteCodeFormat(code) {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	if s.cache == nil || s.db == nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	ctx := c.Request.Context()
	orgIDStr, err := s.cache.Get(ctx, linkInviteKeyPrefix+code).Result()
	if errors.Is(err, redis.Nil) {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	if err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	resellerID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil || resellerID == 0 {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	var reseller data.Organization
	if err := s.db.WithContext(ctx).Where("id = ? AND kind = ?", resellerID, data.Reseller).Take(&reseller).Error; err != nil {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	var count int64
	_ = s.db.WithContext(ctx).Table("pl_channel_links").
		Where("customer_id = ? AND reseller_id = ?", actor.OrganizationID, resellerID).Count(&count).Error

	respond(c, http.StatusOK, "success", gin.H{
		"code":           code,
		"reseller":       reseller,
		"already_linked": count > 0,
	})
}

func (s server) bindChannelLink(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Customer {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if !decode(c, &input) {
		return
	}
	code := normalizeInviteCode(input.Code)
	if !validInviteCodeFormat(code) {
		respond(c, http.StatusBadRequest, "invalid_invite_code", nil)
		return
	}
	if s.cache == nil || s.db == nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	ctx := c.Request.Context()
	orgIDStr, err := s.cache.Get(ctx, linkInviteKeyPrefix+code).Result()
	if errors.Is(err, redis.Nil) {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	if err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	resellerID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil || resellerID == 0 {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	var reseller data.Organization
	if err := s.db.WithContext(ctx).Where("id = ? AND kind = ?", resellerID, data.Reseller).Take(&reseller).Error; err != nil {
		respond(c, http.StatusNotFound, "invalid_invite_code", nil)
		return
	}
	if err := s.db.WithContext(ctx).Exec(
		"INSERT IGNORE INTO pl_channel_links (customer_id, reseller_id) VALUES (?, ?)",
		actor.OrganizationID, reseller.ID,
	).Error; err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	respond(c, http.StatusOK, "success", gin.H{
		"linked":   true,
		"reseller": reseller,
	})
}

type badgeAlert struct {
	View        string `json:"view"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
	Description string `json:"description"`
}

func (s server) workspaceBadges(c *gin.Context) {
	actor := principal(c)
	ctx := c.Request.Context()
	pages := make(map[string]int)
	alerts := make([]badgeAlert, 0, 5)

	addAlert := func(view, label string, count int, desc string) {
		if count <= 0 {
			return
		}
		pages[view] = count
		alerts = append(alerts, badgeAlert{
			View:        view,
			Label:       label,
			Count:       count,
			Description: desc,
		})
	}

	todoProjects := 0
	if list, err := s.channel.List(ctx, actor, 0, 100); err == nil {
		for _, item := range list.Items {
			if item.IsTodo {
				todoProjects++
			}
		}
	}

	switch actor.Organization.Kind {
	case data.Manufacturer:
		addAlert("projects", "渠道询价", todoProjects, fmt.Sprintf("%d 个渠道项目待回复报价或登记供货", todoProjects))
		if s.db != nil {
			var pendingReceipts, pendingSettlements, pendingFinance, openTickets int64
			_ = s.db.WithContext(ctx).Table("pl_retail_orders").
				Where("manufacturer_id = ? AND status = 'payment'", actor.OrganizationID).Count(&pendingReceipts).Error
			_ = s.db.WithContext(ctx).Table("pl_retail_orders").
				Where("manufacturer_id = ? AND status = 'settlement'", actor.OrganizationID).Count(&pendingSettlements).Error
			_ = s.db.WithContext(ctx).Table("pl_channel_finance_requests").
				Where("manufacturer_id = ? AND status IN ('pending', 'requested')", actor.OrganizationID).Count(&pendingFinance).Error
			_ = s.db.WithContext(ctx).Table("pl_channel_service_tickets").
				Where("manufacturer_id = ? AND status = 'open'", actor.OrganizationID).Count(&openTickets).Error

			addAlert("manufacturer-receipts", "直收订单与收款", int(pendingReceipts), fmt.Sprintf("%d 笔直购订单待确认收款", pendingReceipts))
			addAlert("manufacturer-settlements", "代理商佣金结算", int(pendingSettlements), fmt.Sprintf("%d 笔代理商佣金待登记结算", pendingSettlements))
			addAlert("vendor-finance", "财务办理", int(pendingFinance), fmt.Sprintf("%d 笔预存/开票/报销申请待办理", pendingFinance))
			addAlert("vendor-tickets", "渠道工单", int(openTickets), fmt.Sprintf("%d 张渠道售后工单待回复", openTickets))
		}

	case data.Customer:
		addAlert("projects", "我的采购项目", todoProjects, fmt.Sprintf("%d 个采购项目待确认报价或验收交付", todoProjects))
		if s.db != nil {
			var pendingConfirm int64
			_ = s.db.WithContext(ctx).Table("pl_retail_orders").
				Where("customer_id = ? AND status = 'confirmation'", actor.OrganizationID).Count(&pendingConfirm).Error
			addAlert("retail-orders", "厂商订购订单", int(pendingConfirm), fmt.Sprintf("%d 笔代客直购订单待您确认", pendingConfirm))
		}

	case data.Reseller:
		addAlert("projects", "客户项目", todoProjects, fmt.Sprintf("%d 个客户协作项目待跟进办理", todoProjects))
		if s.db != nil {
			today := time.Now().In(chinaTime).Format(time.DateOnly)
			var pendingComm, dueRenewals, resolvedTickets int64
			_ = s.db.WithContext(ctx).Table("pl_retail_orders").
				Where("reseller_id = ? AND status = 'settlement'", actor.OrganizationID).Count(&pendingComm).Error
			_ = s.db.WithContext(ctx).Table("pl_retail_orders").
				Where("reseller_id = ? AND renewal_due_on BETWEEN ? AND DATE_ADD(?, INTERVAL 30 DAY)", actor.OrganizationID, today, today).
				Count(&dueRenewals).Error
			_ = s.db.WithContext(ctx).Table("pl_channel_service_tickets").
				Where("reseller_id = ? AND status = 'resolved'", actor.OrganizationID).Count(&resolvedTickets).Error

			addAlert("commission", "佣金结算", int(pendingComm), fmt.Sprintf("%d 笔已收款订单佣金待厂商结算", pendingComm))
			addAlert("renewals", "续费跟进", int(dueRenewals), fmt.Sprintf("%d 笔年度授权将在 30 天内到期", dueRenewals))
			addAlert("service-tickets", "服务工单", int(resolvedTickets), fmt.Sprintf("%d 张工单厂商已回复待确认关闭", resolvedTickets))
		}
	}

	total := 0
	for _, count := range pages {
		total += count
	}
	respond(c, http.StatusOK, "success", gin.H{
		"total": total,
		"pages": pages,
		"items": alerts,
	})
}
