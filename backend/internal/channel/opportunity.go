package channel

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	opportunityOpen       = "open"
	opportunityIntroduced = "introduced"
	opportunityWon        = "won"
	opportunityLost       = "lost"
	opportunitySettled    = "settled"
)

var categories = map[string]string{
	"integration": "系统对接服务", "connector": "成品接口 / 连接器", "plugin": "开发工具插件",
	"migration": "数据迁移", "training": "实施培训", "other": "其他能力",
}

var systems = map[string]string{
	"erp": "ERP", "mes": "MES", "oa": "OA", "ide": "IDE", "wms": "WMS", "crm": "CRM", "other": "其他系统",
}

var chinaTime = time.FixedZone("UTC+8", 8*60*60)

type Capability struct {
	Category string `json:"category"`
	Source   string `json:"source"`
	Target   string `json:"target"`
}

func (c Capability) valid(requireCategory bool) bool {
	return (categories[c.Category] != "" || !requireCategory && c.Category == "") &&
		(c.Source == "" || systems[c.Source] != "") && (c.Target == "" || systems[c.Target] != "")
}

type PartnerInput struct {
	Capability
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	Evidence string `json:"evidence"`
}

type channelPartner struct {
	ID             uint64    `json:"id,string"`
	OrganizationID uint64    `json:"-"`
	CreatedBy      uint64    `json:"-"`
	CreatedAt      time.Time `json:"createdAt"`
	PartnerInput   `gorm:"embedded"`
}

type OpportunityInput struct {
	Capability
	Note string `json:"note"`
}

type CaptureInput struct {
	RequestID string           `json:"requestId"`
	ProjectID string           `json:"projectId"`
	Payload   OpportunityInput `json:"payload"`
}

type AddPartnerInput struct {
	RequestID string       `json:"requestId"`
	Payload   PartnerInput `json:"payload"`
}

type channelOpportunity struct {
	ID               uint64
	OrganizationID   uint64
	ProjectID        uint64
	ProjectTitle     string `gorm:"->"`
	OpportunityInput `gorm:"embedded"`
	NoteHash         string
	Revision         uint32
	Stage            string
	PartnerID        *uint64
	FollowUp         string
	FollowReason     string
	FeeAmount        *string
	FeeTerms         string
	Outcome          string
	ContactReviews   map[string]contactReview `gorm:"serializer:json"`
	Rejections       []rejection              `gorm:"serializer:json"`
}

type contactChecks struct {
	Version         string `json:"version"`
	InterfaceAccess string `json:"interfaceAccess"`
	Region          string `json:"region"`
	Availability    string `json:"availability"`
}

func (c contactChecks) blocked() bool {
	return c.Version == "blocked" || c.InterfaceAccess == "blocked" ||
		c.Region == "blocked" || c.Availability == "blocked"
}

type contactReview struct {
	Checks contactChecks `json:"checks"`
	Note   string        `json:"note"`
}

type rejection struct {
	PartnerID string    `json:"partnerId"`
	Name      string    `json:"name"`
	Reason    string    `json:"reason"`
	At        time.Time `json:"at"`
}

type channelOpportunityEvent struct {
	ID            uint64
	OpportunityID uint64
	ActorUserID   uint64
	At            time.Time
	Action        string
	Label         string
	Detail        json.RawMessage
}

type channelOpportunityRead struct {
	OpportunityID uint64   `gorm:"primaryKey;autoIncrement:false"`
	UserID        uint64   `gorm:"primaryKey;autoIncrement:false"`
	PartnerIDs    []string `gorm:"serializer:json"`
}

type channelPartnerCase struct {
	OpportunityID uint64 `gorm:"primaryKey;autoIncrement:false"`
	PartnerID     uint64
	Category      string
}

type matchView struct {
	PartnerID   string        `json:"partnerId"`
	Name        string        `json:"name"`
	Owner       string        `json:"owner"`
	Evidence    string        `json:"evidence"`
	Reason      string        `json:"reason"`
	Caveat      string        `json:"caveat"`
	Checks      contactChecks `json:"checks"`
	ReviewNote  string        `json:"reviewNote"`
	specificity int
}

type opportunityHistory struct {
	At    time.Time `json:"at"`
	Label string    `json:"label"`
}

type OpportunityView struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	ProjectTitle string `json:"projectTitle"`
	OpportunityInput
	Revision        uint32                   `json:"revision"`
	Stage           string                   `json:"stage"`
	PartnerID       string                   `json:"partnerId"`
	FollowUp        string                   `json:"followUp"`
	FollowReason    string                   `json:"followReason"`
	FeeAmount       string                   `json:"feeAmount"`
	FeeTerms        string                   `json:"feeTerms"`
	Outcome         string                   `json:"outcome"`
	KnownPartnerIDs []string                 `json:"knownPartnerIds"`
	ContactReviews  map[string]contactReview `json:"contactReviews"`
	Rejections      []rejection              `json:"rejections"`
	History         []opportunityHistory     `json:"history"`
	OpportunityRecommendation
}

type OpportunityRecommendation struct {
	Questions []string    `json:"questions"`
	Matches   []matchView `json:"matches"`
	Brief     string      `json:"brief"`
}

func idString(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func optionalID(id *uint64) string {
	if id == nil {
		return ""
	}
	return idString(*id)
}

func questionsFor(c Capability) []string {
	switch c.Category {
	case "":
		return []string{"需要购买成品接口、委托开发对接服务，还是使用开发工具插件？"}
	case "other":
		return []string{"这项能力暂未归类，请先与客户澄清；不会自动推荐人选。"}
	}
	result := []string{}
	if c.Source == "" || c.Source == "other" {
		result = append(result, "涉及哪类软件或开发工具？")
	}
	if (c.Category == "integration" || c.Category == "connector" || c.Category == "migration") &&
		(c.Target == "" || c.Target == "other") {
		result = append(result, "数据要从哪个系统连接或迁移到哪个目标系统？")
	}
	return result
}

func safeBrief(c Capability) string {
	label := categories[c.Category]
	if label == "" {
		label = "待澄清"
	}
	b := "有一项企业软件配套需求。能力：" + label + "。"
	labels := []string{}
	for _, system := range []string{c.Source, c.Target} {
		if label := systems[system]; label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) != 0 {
		b += "涉及系统：" + strings.Join(labels, " → ") + "。"
	}
	return b + "请确认支持的版本、交付方式和是否可承接。"
}

func matchesFor(item channelOpportunity, partners []channelPartner) []matchView {
	matches := []matchView{}
	if len(questionsFor(item.Capability)) > 0 {
		return matches
	}
	excluded := make(map[string]bool, len(item.Rejections))
	for _, rejected := range item.Rejections {
		excluded[rejected.PartnerID] = true
	}
	for _, partner := range partners {
		id := idString(partner.ID)
		if excluded[id] || item.Category != partner.Category ||
			item.Source != "" && partner.Source != "" && item.Source != partner.Source ||
			item.Target != "" && partner.Target != "" && item.Target != partner.Target {
			continue
		}
		aligned := []string{categories[item.Category]}
		missing := []string{}
		for _, pair := range [][2]string{{item.Source, partner.Source}, {item.Target, partner.Target}} {
			switch {
			case pair[0] != "" && pair[0] == pair[1]:
				aligned = append(aligned, systems[pair[0]])
			case pair[0] != "" && pair[1] == "":
				missing = append(missing, systems[pair[0]])
			}
		}
		caveat := ""
		if partner.Evidence == "" {
			caveat = "仅有团队登记的能力，尚无案例佐证。"
		}
		if len(missing) != 0 {
			caveat += "未记录对 " + strings.Join(missing, "、") + " 的支持。"
		}
		review := item.ContactReviews[id]
		matches = append(matches, matchView{
			PartnerID: id, Name: partner.Name, Owner: partner.Owner, Evidence: partner.Evidence,
			Reason: strings.Join(aligned, " · "), Caveat: caveat + "确切版本、档期和报价仍需向伙伴核实。",
			Checks: review.Checks, ReviewNote: review.Note, specificity: len(aligned),
		})
	}
	slices.SortStableFunc(matches, func(a, b matchView) int {
		if (a.Evidence != "") != (b.Evidence != "") {
			if a.Evidence != "" {
				return -1
			}
			return 1
		}
		return b.specificity - a.specificity
	})
	return matches
}

func opportunityView(item channelOpportunity, partners []channelPartner, history []opportunityHistory, known []string) OpportunityView {
	fee := ""
	if item.FeeAmount != nil {
		fee = *item.FeeAmount
	}
	if known == nil {
		known = []string{}
	}
	if history == nil {
		history = []opportunityHistory{}
	}
	return OpportunityView{
		ID: idString(item.ID), ProjectID: idString(item.ProjectID), ProjectTitle: item.ProjectTitle,
		OpportunityInput: item.OpportunityInput, Revision: item.Revision, Stage: item.Stage, PartnerID: optionalID(item.PartnerID),
		FollowUp: item.FollowUp, FollowReason: item.FollowReason, FeeAmount: fee, FeeTerms: item.FeeTerms, Outcome: item.Outcome,
		KnownPartnerIDs: known, ContactReviews: item.ContactReviews, Rejections: item.Rejections, History: history,
		OpportunityRecommendation: recommendationFor(item, partners),
	}
}

func recommendationFor(item channelOpportunity, partners []channelPartner) OpportunityRecommendation {
	return OpportunityRecommendation{
		Questions: questionsFor(item.Capability), Matches: matchesFor(item, partners), Brief: safeBrief(item.Capability),
	}
}
