package channel

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

const (
	channelQuoted   = "channel_quoted"
	channelRevision = "channel_revision"
	salesQuoted     = "sales_quoted"
	salesRevision   = "sales_revision"
	confirmed       = "confirmed"
	ordered         = "ordered"
	supplied        = "supplied"
	acceptance      = "acceptance"
	rectification   = "rectification"
	completed       = "completed"
	allParties      = "all"
)

const (
	channelKind  = "channel"
	salesKind    = "sales"
	activeQuote  = "active"
	oldQuote     = "superseded"
	expiredQuote = "expired"
)

var ErrQuoteExpired = errors.New("quote_expired")

type quoteLine struct {
	Label     string `json:"label"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unitPrice"`
}

type quoteDraft struct {
	Lines        []quoteLine `json:"lines"`
	ValidUntil   string      `json:"validUntil"`
	DeliveryDays int         `json:"deliveryDays"`
	Note         string      `json:"note"`
}

type channelQuote struct {
	ID             uint64
	ProjectID      uint64
	Kind           string
	Version        uint32
	Status         string
	ChannelQuoteID *uint64
	Lines          []quoteLine `gorm:"column:quote_lines;serializer:json"`
	TotalCents     uint64
	ValidUntil     time.Time
	DeliveryDays   int
	Note           string
	CreatedBy      uint64
	CreatedAt      time.Time
}

type channelOrder struct {
	ID                 uint64
	ProjectID          uint64
	ChannelQuoteID     uint64
	SalesQuoteID       uint64
	SupplyNote         string
	ImplementationNote string
	CreatedBy          uint64
	CreatedAt          time.Time
}

type quoteView struct {
	ID           string      `json:"id"`
	Version      uint32      `json:"version"`
	Status       string      `json:"status"`
	Lines        []quoteLine `json:"lines"`
	Total        string      `json:"total"`
	ValidUntil   string      `json:"validUntil"`
	DeliveryDays int         `json:"deliveryDays"`
	Note         string      `json:"note"`
}

type orderView struct {
	ID             string `json:"id"`
	ChannelQuoteID string `json:"channelQuoteId,omitempty"`
	SalesQuoteID   string `json:"salesQuoteId,omitempty"`
}

type commercialState struct {
	channelQuotes []channelQuote
	salesQuotes   []channelQuote
	order         *channelOrder
}

func loadCommercial(tx *gorm.DB, ids []uint64) (map[uint64]commercialState, error) {
	result := make(map[uint64]commercialState)
	var quotes []channelQuote
	if err := tx.Where("project_id IN ?", ids).Order("version ASC").Find(&quotes).Error; err != nil {
		return nil, err
	}
	for _, q := range quotes {
		state := result[q.ProjectID]
		if q.Kind == channelKind {
			state.channelQuotes = append(state.channelQuotes, q)
		} else {
			state.salesQuotes = append(state.salesQuotes, q)
		}
		result[q.ProjectID] = state
	}
	var orders []channelOrder
	if err := tx.Where("project_id IN ?", ids).Find(&orders).Error; err != nil {
		return nil, err
	}
	for i := range orders {
		state := result[orders[i].ProjectID]
		state.order = &orders[i]
		result[orders[i].ProjectID] = state
	}
	return result, nil
}

func currentQuote(quotes []channelQuote) channelQuote {
	if len(quotes) == 0 {
		return channelQuote{}
	}
	return quotes[len(quotes)-1]
}

func (q channelQuote) statusOn(today string) string {
	if q.Status == activeQuote || q.Status == confirmed {
		if q.ValidUntil.Format(time.DateOnly) < today {
			return expiredQuote
		}
	}
	return q.Status
}

func (q channelQuote) validOn(today string) bool {
	status := q.statusOn(today)
	return status == activeQuote || status == confirmed
}

func normalizeQuote(q *quoteDraft) error {
	if q.ValidUntil == "" || !validDate(q.ValidUntil) || q.DeliveryDays < 1 || q.DeliveryDays > 3650 ||
		len(q.Lines) == 0 || len(q.Lines) > 100 {
		return ErrInvalid
	}
	q.Note = strings.TrimSpace(q.Note)
	if q.Note != "" && !fits(q.Note, 3000) {
		return ErrInvalid
	}
	for i := range q.Lines {
		line := &q.Lines[i]
		line.Label = strings.TrimSpace(line.Label)
		price, ok := normalizeCurrency(line.UnitPrice)
		if !fits(line.Label, 120) || line.Quantity < 1 || line.Quantity > 1000000 || !ok || price == "" {
			return ErrInvalid
		}
		line.UnitPrice = price
	}
	_, err := totalQuoteCents(q.Lines)
	return err
}

func totalQuoteCents(lines []quoteLine) (uint64, error) {
	const maxCents = 9007199254740991
	var total uint64
	for _, line := range lines {
		cents, err := strconv.ParseUint(strings.ReplaceAll(line.UnitPrice, ".", ""), 10, 64)
		if err != nil {
			return 0, ErrInvalid
		}
		total += cents * uint64(line.Quantity)
		if total > maxCents {
			return 0, ErrInvalid
		}
	}
	if total == 0 {
		return 0, ErrInvalid
	}
	return total, nil
}

func viewsForQuotes(quotes []channelQuote, today string) *[]quoteView {
	views := make([]quoteView, 0, len(quotes))
	for _, q := range quotes {
		cents := idString(q.TotalCents % 100)
		if len(cents) == 1 {
			cents = "0" + cents
		}
		views = append(views, quoteView{
			ID: idString(q.ID), Version: q.Version, Status: q.statusOn(today), Lines: q.Lines,
			Total: idString(q.TotalCents/100) + "." + cents, ValidUntil: q.ValidUntil.Format(time.DateOnly),
			DeliveryDays: q.DeliveryDays, Note: q.Note,
		})
	}
	return &views
}

func applyCommercialView(view *View, role string, state commercialState, today string) {
	if role != data.Customer {
		view.Channel = viewsForQuotes(state.channelQuotes, today)
	}
	if role != data.Manufacturer {
		view.Sales = viewsForQuotes(state.salesQuotes, today)
	}
	if state.order == nil {
		return
	}
	order := state.order
	view.Order = &orderView{ID: idString(order.ID)}
	view.SupplyNote = order.SupplyNote
	if role != data.Customer {
		view.Order.ChannelQuoteID = idString(order.ChannelQuoteID)
	}
	if role != data.Manufacturer {
		view.Order.SalesQuoteID = idString(order.SalesQuoteID)
		view.ImplementationNote = order.ImplementationNote
	}
}

func commercialActions(p channelProject, role string, state commercialState, today string) []string {
	channel, sales := currentQuote(state.channelQuotes), currentQuote(state.salesQuotes)
	activeChannel := channel.validOn(today)
	activeSales := sales.validOn(today)
	switch role {
	case data.Customer:
		switch p.Stage {
		case salesQuoted:
			if activeSales {
				return []string{"confirmSales", "reviseSales"}
			}
			return []string{"reviseSales"}
		case acceptance:
			return []string{"accept", "rectify"}
		}
	case data.Manufacturer:
		switch p.Stage {
		case channelQuoted, channelRevision:
			return []string{"channelQuote"}
		case salesQuoted, salesRevision, confirmed:
			if channel.statusOn(today) == expiredQuote {
				return []string{"channelQuote"}
			}
		case ordered:
			return []string{"supply"}
		}
	case data.Reseller:
		switch p.Stage {
		case channelQuoted, salesRevision, salesQuoted:
			if activeChannel {
				return []string{"salesQuote", "reviseChannel"}
			}
			return []string{"reviseChannel"}
		case confirmed:
			if !activeChannel {
				return []string{"reviseChannel"}
			}
			if !activeSales {
				return []string{"salesQuote"}
			}
			return []string{"placeOrder"}
		case supplied, rectification:
			return []string{"implement"}
		}
	}
	return []string{}
}
