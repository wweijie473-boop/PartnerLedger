package channel

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type actionFields struct {
	Product string
	Summary string
	Note    string
}

func (s Service) Act(ctx context.Context, actor data.User, id uint64, action string, input ActionInput) (View, error) {
	if !requestIDPattern.MatchString(input.RequestID) || input.Revision == 0 {
		return View{}, ErrInvalid
	}
	role := actor.Organization.Kind
	var result View
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p channelProject
		if err := visibleProjects(tx, actor).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Customer").First(&p, id).Error; err != nil {
			return err
		}
		allowed, known := canAct(role, action)
		if !known {
			return ErrNotFound
		}
		if !allowed {
			return ErrForbidden
		}
		fields, err := parseAction(action, input.Payload)
		if err != nil {
			return err
		}
		operation := "project:" + strconv.FormatUint(id, 10) + ":" + action
		entry, replay, err := claim(tx, actor.OrganizationID, operation, input.RequestID, struct {
			Revision uint32
			Fields   actionFields
		}{input.Revision, fields})
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, &result)
		}
		if p.Revision != input.Revision {
			return ErrRevision
		}
		if !slices.Contains(actionsFor(p, role), action) {
			return ErrTransition
		}
		event, err := applyAction(tx, actor, &p, action, fields)
		if err != nil {
			return err
		}
		p.Revision++
		if err := tx.Model(&channelProject{}).Where("id = ?", p.ID).Updates(map[string]any{
			"stage": p.Stage, "revision": p.Revision, "manufacturer_id": p.ManufacturerID,
			"product_id": p.ProductID, "product_name": p.ProductName, "inquiry_id": p.InquiryID,
			"summary": p.Summary, "processed": p.Processed, "gap_note": p.GapNote,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result, err = loadView(tx, p, role)
		if err != nil {
			return err
		}
		return saveResult(tx, entry, p.ID, result)
	})
	return result, err
}

func canAct(role, action string) (bool, bool) {
	switch action {
	case "submitInquiry", "withdrawInquiry":
		return role == data.Reseller, true
	case "flagGap":
		return role == data.Manufacturer, true
	case "requestInfo":
		return role == data.Manufacturer || role == data.Reseller, true
	case "replyInfo":
		return role == data.Customer || role == data.Reseller, true
	default:
		return false, false
	}
}

func parseAction(action string, raw []byte) (actionFields, error) {
	switch action {
	case "submitInquiry":
		var payload struct {
			Product string `json:"product"`
			Summary string `json:"summary"`
		}
		if err := actionPayload(raw, &payload); err != nil {
			return actionFields{}, err
		}
		payload.Summary = strings.TrimSpace(payload.Summary)
		if _, err := parseID(payload.Product); err != nil || !fits(payload.Summary, 3000) {
			return actionFields{}, ErrInvalid
		}
		return actionFields{Product: payload.Product, Summary: payload.Summary}, nil
	case "withdrawInquiry":
		return actionFields{}, actionPayload(raw, &struct{}{})
	default:
		var payload struct {
			Note string `json:"note"`
		}
		if err := actionPayload(raw, &payload); err != nil {
			return actionFields{}, err
		}
		payload.Note = strings.TrimSpace(payload.Note)
		if !fits(payload.Note, 3000) {
			return actionFields{}, ErrInvalid
		}
		return actionFields{Note: payload.Note}, nil
	}
}

func applyAction(tx *gorm.DB, actor data.User, p *channelProject, action string, fields actionFields) (channelEvent, error) {
	role := actor.Organization.Kind
	switch action {
	case "submitInquiry":
		productID, _ := parseID(fields.Product)
		var product data.Product
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("published = ?", true).First(&product, productID).Error; err != nil {
			return channelEvent{}, err
		}
		p.ProductID, p.ManufacturerID, p.ProductName = &product.ID, &product.ManufacturerID, product.Name
		p.InquiryID = "RFQ-" + strconv.FormatUint(p.ID, 10) + "-" + strconv.FormatUint(uint64(p.Revision)+1, 10)
		p.Stage, p.Summary = inquired, fields.Summary
		return eventFor(actor, *p, upstream, "代理商提交询价", p.Summary), nil
	case "withdrawInquiry":
		event := eventFor(actor, *p, dealerOnly, "已撤回未处理询价", "")
		p.ManufacturerID, p.ProductID = nil, nil
		p.Stage, p.ProductName, p.InquiryID, p.Summary, p.GapNote, p.Processed = requested, "", "", "", "", false
		return event, nil
	case "flagGap":
		p.GapNote, p.Processed = fields.Note, true
		return eventFor(actor, *p, upstream, "厂商标记暂不支持的附加需求", fields.Note), nil
	case "requestInfo":
		if role == data.Reseller {
			p.Stage = customerInfo
			return eventFor(actor, *p, downstream, "请求补充资料", fields.Note), nil
		}
		p.Stage, p.Processed = vendorInfo, true
		return eventFor(actor, *p, upstream, "请求补充资料", fields.Note), nil
	case "replyInfo":
		if role == data.Customer {
			p.Stage = requested
			if p.InquiryID != "" {
				p.Stage = vendorInfo
			}
			return eventFor(actor, *p, downstream, "已回复补充资料", fields.Note), nil
		}
		p.Stage, p.Processed = inquired, true
		return eventFor(actor, *p, upstream, "已回复补充资料", fields.Note), nil
	default:
		return channelEvent{}, ErrNotFound
	}
}
