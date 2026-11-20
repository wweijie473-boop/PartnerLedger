package channel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/catalog"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InquiryDraftInput struct {
	RequestID   string               `json:"requestId"`
	Revision    uint32               `json:"revision"`
	Confirmed   bool                 `json:"confirmed"`
	ProductID   string               `json:"productId"`
	Requirement catalog.Requirements `json:"requirement"`
}

type channelInquiryDraft struct {
	ID              uint64               `json:"id,string"`
	ProjectID       uint64               `json:"projectId,string"`
	ProjectRevision uint32               `json:"projectRevision"`
	ProductID       uint64               `json:"productId,string"`
	CreatedBy       uint64               `json:"-"`
	Requirement     catalog.Requirements `json:"requirement" gorm:"serializer:json"`
	Summary         string               `json:"summary"`
	CreatedAt       time.Time            `json:"createdAt"`
}

func (s Service) CreateInquiryDraft(ctx context.Context, actor data.User, id uint64, input InquiryDraftInput) (channelInquiryDraft, error) {
	var draft channelInquiryDraft
	if !dealer(actor) {
		return draft, ErrForbidden
	}
	productID, err := parseID(input.ProductID)
	if err != nil || !input.Confirmed || input.Revision == 0 || !requestIDPattern.MatchString(input.RequestID) ||
		input.Requirement.Normalize() != nil {
		return draft, ErrInvalid
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p channelProject
		if err := visibleProjects(tx, actor).Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, id).Error; err != nil {
			return err
		}
		entry, replay, err := claim(tx, actor.OrganizationID, "inquiry-draft:"+idString(id)+":"+idString(actor.ID), input.RequestID, input)
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, &draft)
		}
		if p.Revision != input.Revision {
			return ErrRevision
		}
		if p.Stage != requested {
			return ErrTransition
		}
		var product data.Product
		if err := catalog.Visible(tx, actor).Select("id").First(&product, productID).Error; err != nil {
			return err
		}
		draft = channelInquiryDraft{
			ProjectID: p.ID, ProjectRevision: p.Revision, ProductID: product.ID, CreatedBy: actor.ID,
			Requirement: input.Requirement, Summary: inquirySummary(input.Requirement),
		}
		if err := tx.Create(&draft).Error; err != nil {
			return err
		}
		return saveResult(tx, entry, &p.ID, draft)
	})
	return draft, err
}

func (s Service) LatestInquiryDraft(ctx context.Context, actor data.User, id uint64) (channelInquiryDraft, error) {
	var draft channelInquiryDraft
	if !dealer(actor) {
		return draft, ErrForbidden
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := dealerProject(tx, actor, id)
		if err != nil {
			return err
		}
		if p.Stage != requested {
			return ErrNotFound
		}
		published := catalog.Visible(tx, actor).Select("id")
		return tx.Where("project_id = ? AND project_revision = ? AND created_by = ? AND product_id IN (?)",
			id, p.Revision, actor.ID, published).Last(&draft).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return draft, err
}

func inquirySummary(r catalog.Requirements) string {
	parts := make([]string, 0, 6)
	if r.EnterpriseType != "" {
		parts = append(parts, "企业类型："+r.EnterpriseType)
	}
	if r.EmployeeCount != nil {
		parts = append(parts, fmt.Sprintf("员工人数：%d", *r.EmployeeCount))
	}
	if len(r.RequiredModules) > 0 {
		parts = append(parts, "所需模块："+strings.Join(r.RequiredModules, "、"))
	}
	if r.DeploymentMode != "" {
		parts = append(parts, "部署要求："+r.DeploymentMode)
	}
	if r.Version != "" {
		parts = append(parts, "版本要求："+r.Version)
	}
	if r.LicenseCount != nil {
		parts = append(parts, fmt.Sprintf("授权席位：%d", *r.LicenseCount))
	} else {
		parts = append(parts, "授权席位：待确认")
	}
	return strings.Join(parts, "；")
}
