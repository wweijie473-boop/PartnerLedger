package channel

import (
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func expiredForAction(p channelProject, state commercialState, action, today string) bool {
	channel := currentQuote(state.channelQuotes)
	sales := currentQuote(state.salesQuotes)
	switch action {
	case "confirmSales":
		return p.Stage == salesQuoted && sales.statusOn(today) == expiredQuote
	case "salesQuote":
		switch p.Stage {
		case channelQuoted, salesQuoted, salesRevision, confirmed:
			return channel.statusOn(today) == expiredQuote
		}
	case "placeOrder":
		return p.Stage == confirmed && (channel.statusOn(today) == expiredQuote || sales.statusOn(today) == expiredQuote)
	}
	return false
}

func applyCommercialAction(tx *gorm.DB, actor data.User, p *channelProject, action string, fields actionFields,
	state commercialState, today string) ([]channelEvent, error) {
	channel, sales := currentQuote(state.channelQuotes), currentQuote(state.salesQuotes)
	event := func(scope, label, detail string) []channelEvent {
		return []channelEvent{eventFor(actor, *p, scope, label, detail)}
	}
	switch action {
	case "channelQuote":
		id, err := writeQuote(tx, *p, actor.ID, channelKind, channel, nil, *fields.Quote, today)
		if err != nil {
			return nil, err
		}
		if err := markQuote(tx, sales, oldQuote); err != nil {
			return nil, err
		}
		p.Stage, p.Processed = channelQuoted, true
		return event(upstream, "厂商发送渠道报价", idString(id)), nil
	case "salesQuote":
		id, err := writeQuote(tx, *p, actor.ID, salesKind, sales, &channel.ID, *fields.Quote, today)
		if err != nil {
			return nil, err
		}
		p.Stage = salesQuoted
		return event(downstream, "代理商发送对客报价", idString(id)), nil
	case "reviseChannel":
		if err := markQuote(tx, channel, oldQuote); err != nil {
			return nil, err
		}
		if err := markQuote(tx, sales, oldQuote); err != nil {
			return nil, err
		}
		p.Stage = channelRevision
		return event(upstream, "代理商请求厂商改价", fields.Note), nil
	case "reviseSales":
		if err := markQuote(tx, sales, oldQuote); err != nil {
			return nil, err
		}
		p.Stage = salesRevision
		return event(downstream, "客户请求修改对客方案", fields.Note), nil
	case "confirmSales":
		if fields.QuoteID != idString(sales.ID) {
			return nil, ErrTransition
		}
		if err := markQuote(tx, sales, confirmed); err != nil {
			return nil, err
		}
		p.Stage = confirmed
		return event(downstream, "客户确认报价", fields.QuoteID), nil
	case "placeOrder":
		if fields.QuoteID != idString(sales.ID) || sales.Status != confirmed ||
			sales.ChannelQuoteID == nil || *sales.ChannelQuoteID != channel.ID {
			return nil, ErrTransition
		}
		order := channelOrder{ProjectID: p.ID, ChannelQuoteID: channel.ID, SalesQuoteID: sales.ID, CreatedBy: actor.ID}
		if err := tx.Create(&order).Error; err != nil {
			return nil, err
		}
		if err := markQuote(tx, channel, ordered); err != nil {
			return nil, err
		}
		if err := markQuote(tx, sales, ordered); err != nil {
			return nil, err
		}
		p.Stage = ordered
		return event(allParties, "代理商确认下单", idString(order.ID)), nil
	case "supply":
		if err := updateOrderNote(tx, state.order, "supply_note", fields.Note); err != nil {
			return nil, err
		}
		p.Stage = supplied
		return event(allParties, "厂商已完成供货或授权", fields.Note), nil
	case "implement":
		if err := updateOrderNote(tx, state.order, "implementation_note", fields.Note); err != nil {
			return nil, err
		}
		p.Stage = acceptance
		return event(downstream, "代理商已提交实施验收", fields.Note), nil
	case "rectify":
		p.Stage = rectification
		return event(downstream, "客户退回整改", fields.Note), nil
	case "accept":
		p.Stage = completed
		return []channelEvent{
			eventFor(actor, *p, downstream, "客户完成验收", fields.Note),
			eventFor(actor, *p, upstream, "项目已验收", ""),
		}, nil
	default:
		return nil, ErrNotFound
	}
}

func writeQuote(tx *gorm.DB, project channelProject, actorID uint64, kind string, prior channelQuote,
	channelQuoteID *uint64, draft quoteDraft, today string) (uint64, error) {
	if draft.ValidUntil < today {
		return 0, ErrInvalid
	}
	total, err := totalQuoteCents(draft.Lines)
	if err != nil {
		return 0, err
	}
	expires, err := time.Parse(time.DateOnly, draft.ValidUntil)
	if err != nil {
		return 0, ErrInvalid
	}
	if err := markQuote(tx, prior, oldQuote); err != nil {
		return 0, err
	}
	quote := channelQuote{
		ProjectID: project.ID, Kind: kind, Version: prior.Version + 1, Status: activeQuote, ChannelQuoteID: channelQuoteID,
		Lines: draft.Lines, TotalCents: total, ValidUntil: expires, DeliveryDays: draft.DeliveryDays,
		Note: draft.Note, CreatedBy: actorID,
	}
	err = tx.Create(&quote).Error
	return quote.ID, err
}

func markQuote(tx *gorm.DB, quote channelQuote, status string) error {
	if quote.ID == 0 || quote.Status == status {
		return nil
	}
	return tx.Model(&channelQuote{}).Where("id = ?", quote.ID).Update("status", status).Error
}

func updateOrderNote(tx *gorm.DB, order *channelOrder, column, note string) error {
	if order == nil {
		return ErrTransition
	}
	return tx.Model(&channelOrder{}).Where("id = ?", order.ID).Update(column, note).Error
}
