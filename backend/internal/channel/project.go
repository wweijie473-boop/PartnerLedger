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
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	CustomerName       *string      `json:"customerName,omitempty"`
	Requirement        *string      `json:"requirement,omitempty"`
	DueDate            string       `json:"dueDate,omitempty"`
	Product            string       `json:"product,omitempty"`
	ProductName        string       `json:"productName,omitempty"`
	Stage              string       `json:"stage"`
	StageLabel         string       `json:"stageLabel"`
	Revision           uint32       `json:"revision"`
	IsTodo             bool         `json:"isTodo"`
	NextStep           string       `json:"nextStep"`
	Actions            []string     `json:"actions"`
	Inquiry            *inquiryView `json:"inquiry,omitempty"`
	Channel            *[]quoteView `json:"channelQuotes,omitempty"`
	Sales              *[]quoteView `json:"salesQuotes,omitempty"`
	Order              *orderView   `json:"order,omitempty"`
	SupplyNote         string       `json:"supplyNote,omitempty"`
	ImplementationNote string       `json:"implementationNote,omitempty"`
	Timeline           []eventView  `json:"timeline"`
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
	channelQuoted: "渠道已报价", channelRevision: "渠道报价待改版", "channel_wait": "渠道报价已发送",
	salesQuoted: "待客户审阅", salesRevision: "对客报价待改版", confirmed: "已确认待下单",
	ordered: "待厂商供货", supplied: "待代理商实施", "shipped": "已供货",
	acceptance: "待客户验收", rectification: "待整改", completed: "已验收",
}

var actionLabels = map[string]string{
	"requestInfo": "要求补资料", "replyInfo": "回复资料",
	"submitInquiry": "提交厂商询价", "withdrawInquiry": "撤回询价",
	"flagGap":      "标记不支持的附加需求",
	"channelQuote": "发送渠道报价", "reviseChannel": "请求厂商改价",
	"salesQuote": "发送对客报价", "reviseSales": "请求修改方案",
	"confirmSales": "确认此版报价", "placeOrder": "确认下单",
	"supply": "登记供货与授权", "implement": "提交实施验收", "rectify": "退回整改", "accept": "确认验收",
}

var stageOwners = map[string]string{
	requested: data.Reseller, customerInfo: data.Customer, inquired: data.Manufacturer, vendorInfo: data.Reseller,
	channelQuoted: data.Reseller, channelRevision: data.Manufacturer, salesQuoted: data.Customer,
	salesRevision: data.Reseller, confirmed: data.Reseller, ordered: data.Manufacturer,
	supplied: data.Reseller, acceptance: data.Customer, rectification: data.Reseller,
}

func actionsFor(p channelProject, role string, state commercialState, today string) []string {
	switch role {
	case data.Customer:
		if p.Stage == customerInfo {
			return []string{"replyInfo"}
		}
	case data.Manufacturer:
		if p.Stage == inquired {
			if p.GapNote == "" {
				return []string{"channelQuote", "requestInfo", "flagGap"}
			}
			return []string{"channelQuote", "requestInfo"}
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
	return commercialActions(p, role, state, today)
}

func visibleStage(stage, role string) string {
	switch role {
	case data.Customer:
		switch stage {
		case inquired, vendorInfo, channelQuoted, channelRevision:
			stage = "processing"
		}
	case data.Manufacturer:
		switch stage {
		case customerInfo:
			stage = vendorInfo
		case channelQuoted, salesQuoted, salesRevision, confirmed:
			stage = "channel_wait"
		case supplied, acceptance, rectification:
			stage = "shipped"
		}
	}
	return stage
}

func projectView(p channelProject, role string, events []channelEvent, state commercialState, today string) View {
	item, actions := projectListItem(p, role, state, today)
	v := View{
		ID: item.ID, Title: item.Title, CustomerName: item.CustomerName, DueDate: item.DueDate,
		Stage: item.Stage, StageLabel: item.StageLabel, IsTodo: item.IsTodo, NextStep: item.NextStep,
		Revision: p.Revision, Actions: actions, Timeline: []eventView{},
	}
	if role != data.Manufacturer {
		v.Requirement = &p.Requirement
	}
	applyCommercialView(&v, role, state, today)
	if p.ProductID != nil && (role != data.Customer || len(state.salesQuotes) != 0) {
		v.Product = idString(*p.ProductID)
		v.ProductName = p.ProductName
	}
	if role != data.Customer {
		if p.InquiryID != "" && p.ProductID != nil {
			v.Inquiry = &inquiryView{p.InquiryID, p.Summary, p.Processed, p.GapNote}
		}
	}
	for _, e := range events {
		if role == data.Reseller || role == data.Customer && (e.Scope == downstream || e.Scope == allParties) ||
			role == data.Manufacturer && (e.Scope == upstream || e.Scope == allParties) && e.InquiryID == p.InquiryID {
			v.Timeline = append(v.Timeline, eventView{e.At, e.Label, e.Detail})
		}
	}
	return v
}

func projectListItem(p channelProject, role string, state commercialState, today string) (ProjectListItem, []string) {
	stage := visibleStage(p.Stage, role)
	actions := actionsFor(p, role, state, today)
	item := ProjectListItem{
		ID: strconv.FormatUint(p.ID, 10), Title: p.Title, Stage: stage, StageLabel: stageLabels[stage],
		NextStep: stageLabels[stage],
	}
	if len(actions) > 0 && (stageOwners[p.Stage] == role ||
		role == data.Manufacturer && actions[0] == "channelQuote" && currentQuote(state.channelQuotes).statusOn(today) == expiredQuote) {
		item.IsTodo = true
		item.NextStep = actionLabels[actions[0]]
	}
	if role == data.Manufacturer {
		item.Title = p.ProductName + " · 渠道采购项目"
	} else {
		item.CustomerName, item.DueDate = &p.Customer.Name, p.DueDate
	}
	return item, actions
}
