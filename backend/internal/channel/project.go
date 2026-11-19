package channel

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const (
	requested    = "requested"
	customerInfo = "customer_info"
	inquired     = "inquired"
	vendorInfo   = "vendor_info"
	downstream   = "downstream"
	upstream     = "upstream"
	dealerOnly   = "dealer"
)

type channelProject struct {
	ID             uint64
	CustomerID     uint64
	Customer       data.Organization
	ResellerID     uint64
	ManufacturerID *uint64
	ProductID      *uint64
	ProductName    string
	Title          string
	Requirement    string
	DueDate        string
	Stage          string
	Revision       uint32
	InquiryID      string
	Summary        string
	Processed      bool
	GapNote        string
}

type channelEvent struct {
	ID          uint64
	ProjectID   uint64
	ActorUserID uint64
	ActorOrgID  uint64
	At          time.Time `gorm:"autoCreateTime"`
	Scope       string
	InquiryID   string
	Label       string
	Detail      string
}

type channelRequest struct {
	OrganizationID uint64 `gorm:"primaryKey;autoIncrement:false"`
	Operation      string `gorm:"primaryKey"`
	RequestID      string `gorm:"primaryKey"`
	RequestHash    string
	ProjectID      *uint64
	Result         json.RawMessage
}

type View struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	CustomerName *string            `json:"customerName,omitempty"`
	Requirement  *string            `json:"requirement,omitempty"`
	DueDate      string             `json:"dueDate,omitempty"`
	Product      string             `json:"product,omitempty"`
	Stage        string             `json:"stage"`
	StageLabel   string             `json:"stageLabel"`
	Revision     uint32             `json:"revision"`
	IsTodo       bool               `json:"isTodo"`
	NextStep     string             `json:"nextStep"`
	Actions      []string           `json:"actions"`
	Inquiry      *inquiryView       `json:"inquiry,omitempty"`
	Channel      *[]json.RawMessage `json:"channelQuotes,omitempty"`
	Sales        *[]json.RawMessage `json:"salesQuotes,omitempty"`
	Timeline     []eventView        `json:"timeline"`
}

type inquiryView struct {
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Processed bool   `json:"processed"`
	GapNote   string `json:"gapNote,omitempty"`
}

type eventView struct {
	At     time.Time `json:"at"`
	Label  string    `json:"label"`
	Detail string    `json:"detail"`
}

var stageLabels = map[string]string{
	requested: "待整理需求", customerInfo: "待客户补资料",
	inquired: "待厂商处理", vendorInfo: "待补询价资料", "processing": "代理商跟进中",
}

var actionLabels = map[string]string{
	"requestInfo": "要求补资料", "replyInfo": "回复资料",
	"submitInquiry": "提交厂商询价", "withdrawInquiry": "撤回询价",
	"flagGap": "标记不支持的附加需求",
}

func actionsFor(p channelProject, role string) []string {
	switch role {
	case data.Customer:
		if p.Stage == customerInfo {
			return []string{"replyInfo"}
		}
	case data.Manufacturer:
		if p.Stage == inquired {
			if p.GapNote == "" {
				return []string{"requestInfo", "flagGap"}
			}
			return []string{"requestInfo"}
		}
	case data.Reseller:
		switch p.Stage {
		case requested:
			return []string{"submitInquiry", "requestInfo"}
		case inquired:
			if !p.Processed {
				return []string{"withdrawInquiry"}
			}
		case vendorInfo:
			return []string{"replyInfo", "requestInfo"}
		}
	}
	return []string{}
}

func projectView(p channelProject, role string, events []channelEvent) View {
	stage := p.Stage
	switch role {
	case data.Customer:
		if stage == inquired || stage == vendorInfo {
			stage = "processing"
		}
	case data.Manufacturer:
		if stage == customerInfo {
			stage = vendorInfo
		}
	}
	actions := actionsFor(p, role)
	v := View{
		ID: strconv.FormatUint(p.ID, 10), Title: p.Title, Stage: stage, StageLabel: stageLabels[stage],
		Revision: p.Revision, Actions: actions, NextStep: stageLabels[stage], Timeline: []eventView{},
	}
	if len(actions) > 0 && actions[0] != "withdrawInquiry" {
		v.IsTodo = true
		v.NextStep = actionLabels[actions[0]]
	}
	if role == data.Manufacturer {
		v.Title = p.ProductName + " · 渠道采购项目"
	} else {
		v.CustomerName, v.Requirement, v.DueDate = &p.Customer.Name, &p.Requirement, p.DueDate
		v.Sales = &[]json.RawMessage{}
	}
	if role != data.Customer {
		v.Channel = &[]json.RawMessage{}
		if p.InquiryID != "" && p.ProductID != nil {
			v.Product = strconv.FormatUint(*p.ProductID, 10)
			v.Inquiry = &inquiryView{p.InquiryID, p.Summary, p.Processed, p.GapNote}
		}
	}
	for _, e := range events {
		if role == data.Reseller || role == data.Customer && e.Scope == downstream ||
			role == data.Manufacturer && e.Scope == upstream && e.InquiryID == p.InquiryID {
			v.Timeline = append(v.Timeline, eventView{e.At, e.Label, e.Detail})
		}
	}
	return v
}
