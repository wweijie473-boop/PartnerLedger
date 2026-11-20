package channel

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var feePattern = regexp.MustCompile(`^([0-9]{1,8})(?:\.([0-9]{1,2}))?$`)

type followInput struct {
	Date   string `json:"followUp"`
	Reason string `json:"followReason"`
}

type introduceInput struct {
	PartnerID string `json:"partnerId"`
	Consent   bool   `json:"consent"`
	FollowUp  string `json:"followUp"`
	FeeAmount string `json:"feeAmount"`
	FeeTerms  string `json:"feeTerms"`
}

type reviewInput struct {
	PartnerID string `json:"partnerId"`
	contactReview
}

type rejectInput struct {
	PartnerID string `json:"partnerId"`
	Reason    string `json:"reason"`
}

type outcomeInput struct {
	Outcome  string `json:"outcome"`
	Remember bool   `json:"remember"`
}

func positiveFee(value string) bool {
	return strings.Trim(value, "0.") != ""
}

func normalizeCurrency(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	m := feePattern.FindStringSubmatch(value)
	if m == nil {
		return "", false
	}
	whole, _ := strconv.ParseUint(m[1], 10, 32)
	return idString(whole) + "." + (m[2] + "00")[:2], true
}

func parseOpportunityAction(action string, raw []byte) (any, error) {
	switch action {
	case "acknowledge", "settle":
		var p struct{}
		return p, actionPayload(raw, &p)
	case "clarify":
		var p Capability
		if err := actionPayload(raw, &p); err != nil || !p.valid(true) {
			return nil, ErrInvalid
		}
		return p, nil
	case "contactReview":
		var p reviewInput
		if err := actionPayload(raw, &p); err != nil {
			return nil, err
		}
		p.Note = strings.TrimSpace(p.Note)
		if _, err := parseID(p.PartnerID); err != nil || p.Note != "" && !fits(p.Note, 500) {
			return nil, ErrInvalid
		}
		for _, field := range []*string{&p.Checks.Version, &p.Checks.InterfaceAccess, &p.Checks.Region, &p.Checks.Availability} {
			switch *field {
			case "":
				*field = "unknown"
			case "unknown", "confirmed", "blocked":
			default:
				return nil, ErrInvalid
			}
		}
		return p, nil
	case "rejectPartner":
		var p rejectInput
		if err := actionPayload(raw, &p); err != nil {
			return nil, err
		}
		p.Reason = strings.TrimSpace(p.Reason)
		if _, err := parseID(p.PartnerID); err != nil || !fits(p.Reason, 500) {
			return nil, ErrInvalid
		}
		return p, nil
	case "introduce":
		var p introduceInput
		if err := actionPayload(raw, &p); err != nil {
			return nil, err
		}
		fee, ok := normalizeCurrency(p.FeeAmount)
		p.FeeAmount, p.FeeTerms = fee, strings.TrimSpace(p.FeeTerms)
		if _, err := parseID(p.PartnerID); err != nil || !p.Consent || p.FollowUp == "" || !validDate(p.FollowUp) ||
			!ok || (p.FeeTerms != "" || positiveFee(fee)) && !fits(p.FeeTerms, 300) {
			return nil, ErrInvalid
		}
		return p, nil
	case "follow":
		var p followInput
		if err := actionPayload(raw, &p); err != nil || p.Date == "" || !validDate(p.Date) {
			return nil, ErrInvalid
		}
		switch p.Reason {
		case "", "customer", "partner", "fee":
			return p, nil
		default:
			return nil, ErrInvalid
		}
	case "win", "lose":
		var p outcomeInput
		if err := actionPayload(raw, &p); err != nil {
			return nil, err
		}
		p.Outcome = strings.TrimSpace(p.Outcome)
		if !fits(p.Outcome, 500) || action == "lose" && p.Remember {
			return nil, ErrInvalid
		}
		return p, nil
	default:
		return nil, ErrNotFound
	}
}

func (s Service) ActOnOpportunity(ctx context.Context, actor data.User, id uint64, action string, input ActionInput) (OpportunityView, error) {
	if !dealer(actor) {
		return OpportunityView{}, ErrForbidden
	}
	if !requestIDPattern.MatchString(input.RequestID) || input.Revision == 0 {
		return OpportunityView{}, ErrInvalid
	}
	var result OpportunityView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item channelOpportunity
		if err := opportunities(tx, actor).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("pl_channel_opportunities.id = ?", id).Take(&item).Error; err != nil {
			return err
		}
		payload, err := parseOpportunityAction(action, input.Payload)
		if err != nil {
			return err
		}
		operation := "opportunity:" + idString(id) + ":" + action + ":" + idString(actor.ID)
		entry, replay, err := claim(tx, actor.OrganizationID, operation, input.RequestID, struct {
			Revision uint32
			Payload  any
		}{input.Revision, payload})
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, &result)
		}
		if input.Revision != item.Revision {
			return ErrRevision
		}
		if !opportunityActionAllowed(item, action) {
			return ErrTransition
		}
		label, err := applyOpportunityAction(tx, actor, &item, action, payload)
		if err != nil {
			return err
		}
		item.Revision++
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		audit := payload
		if action == "introduce" {
			audit = struct {
				introduceInput
				ConsentSource string `json:"consentSource"`
			}{payload.(introduceInput), "manual_dealer_confirmation"}
		}
		if err := addOpportunityEvent(tx, actor.ID, item.ID, action, label, audit); err != nil {
			return err
		}
		result, err = loadOpportunity(tx, actor, item)
		if err != nil {
			return err
		}
		return saveResult(tx, entry, &item.ProjectID, result)
	})
	return result, err
}

func opportunityActionAllowed(item channelOpportunity, action string) bool {
	switch action {
	case "follow":
		return item.Stage == opportunityOpen || item.Stage == opportunityIntroduced ||
			item.Stage == opportunityWon && item.FeeAmount != nil && positiveFee(*item.FeeAmount)
	case "win":
		return item.Stage == opportunityIntroduced
	case "lose":
		return item.Stage == opportunityOpen || item.Stage == opportunityIntroduced
	case "settle":
		return item.Stage == opportunityWon && item.FeeAmount != nil && positiveFee(*item.FeeAmount)
	default:
		return item.Stage == opportunityOpen
	}
}

func applyOpportunityAction(tx *gorm.DB, actor data.User, item *channelOpportunity, action string, payload any) (string, error) {
	switch p := payload.(type) {
	case Capability:
		item.Capability = p
		return "更新能力与系统范围", nil
	case reviewInput:
		if _, err := findCandidate(tx, *item, p.PartnerID); err != nil {
			return "", err
		}
		item.ContactReviews[p.PartnerID] = p.contactReview
		return "记录伙伴联系前核对结果", nil
	case rejectInput:
		match, err := findCandidate(tx, *item, p.PartnerID)
		if err != nil {
			return "", err
		}
		item.Rejections = append(item.Rejections, rejection{match.PartnerID, match.Name, p.Reason, time.Now().UTC()})
		return "记录伙伴未匹配原因", nil
	case introduceInput:
		match, err := findCandidate(tx, *item, p.PartnerID)
		if err != nil {
			return "", err
		}
		if match.Checks.blocked() {
			return "", ErrInvalid
		}
		partnerID, _ := parseID(p.PartnerID)
		item.Stage, item.PartnerID, item.FollowUp, item.FollowReason = opportunityIntroduced, &partnerID, p.FollowUp, "partner"
		item.FeeTerms = p.FeeTerms
		if p.FeeAmount != "" {
			item.FeeAmount = &p.FeeAmount
		}
		return "登记经双方同意的引荐", nil
	case followInput:
		if p.Reason == "" {
			p.Reason = map[string]string{opportunityOpen: "customer", opportunityIntroduced: "partner", opportunityWon: "fee"}[item.Stage]
		}
		if p.Reason == "fee" && item.Stage != opportunityWon {
			return "", ErrInvalid
		}
		item.FollowUp, item.FollowReason = p.Date, p.Reason
		return "更新下次跟进日期", nil
	case outcomeInput:
		item.Outcome = p.Outcome
		if action == "lose" {
			item.Stage = opportunityLost
			return "登记未促成原因", nil
		}
		item.Stage = opportunityWon
		if p.Remember && item.PartnerID != nil {
			if err := tx.Create(&channelPartnerCase{item.ID, *item.PartnerID, item.Category}).Error; err != nil {
				return "", err
			}
		}
		if item.FeeAmount != nil && positiveFee(*item.FeeAmount) {
			item.FollowReason = "fee"
		}
		return "登记已促成合作", nil
	}
	if action == "settle" {
		item.Stage = opportunitySettled
		return "人工登记介绍费已结清", nil
	}
	partners, err := loadPartners(tx, actor.OrganizationID)
	if err != nil {
		return "", err
	}
	return "已查看当前伙伴依据", markOpportunityRead(tx, item.ID, actor.ID, matchesFor(*item, partners))
}

func findCandidate(tx *gorm.DB, item channelOpportunity, partnerID string) (matchView, error) {
	id, _ := parseID(partnerID)
	var partner channelPartner
	if err := tx.Where("organization_id = ?", item.OrganizationID).First(&partner, id).Error; err != nil {
		return matchView{}, err
	}
	matches := matchesFor(item, []channelPartner{partner})
	if len(matches) == 0 {
		return matchView{}, ErrTransition
	}
	return matches[0], nil
}
