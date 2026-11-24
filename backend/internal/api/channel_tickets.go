package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type channelServiceTicket struct {
	ID             uint64                `json:"id,string"`
	ManufacturerID uint64                `json:"manufacturer_id,string"`
	ResellerID     uint64                `json:"reseller_id,string"`
	OrderID        *uint64               `json:"order_id,string,omitempty"`
	Category       string                `json:"category"`
	Subject        string                `json:"subject"`
	Detail         string                `json:"detail"`
	Status         string                `json:"status"`
	Revision       uint32                `json:"revision"`
	CreatedAt      time.Time             `json:"created_at"`
	CreatedBy      uint64                `json:"-"`
	RequestID      string                `json:"-"`
	RequestHash    string                `json:"-"`
	Replies        []channelTicketAction `json:"replies" gorm:"-"`
}

func (channelServiceTicket) TableName() string { return "pl_channel_service_tickets" }

type channelTicketAction struct {
	TicketID  uint64    `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Revision  uint32    `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Role      string    `json:"role"`
	Action    string    `json:"-"`
	ActorID   uint64    `json:"-"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"time"`
}

func (channelTicketAction) TableName() string { return "pl_channel_ticket_actions" }

func (s server) listServiceTickets(c *gin.Context) {
	after, ok := operationsCursor(c)
	if !ok {
		return
	}
	actor := principal(c)
	items := make([]channelServiceTicket, 0)
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(retailScope(actor)+" = ? AND id > ?", actor.OrganizationID, after).
			Order("id").Limit(operationsPageSize + 1).Find(&items).Error; err != nil || len(items) == 0 {
			return err
		}
		ids := make([]uint64, 0, len(items))
		positions := make(map[uint64]int, len(items))
		for i := range items {
			ids = append(ids, items[i].ID)
			positions[items[i].ID] = i
			items[i].Replies = []channelTicketAction{}
		}
		var actions []channelTicketAction
		if err := tx.Where("ticket_id IN ? AND action <> 'close'", ids).Order("ticket_id, revision").Find(&actions).Error; err != nil {
			return err
		}
		for _, action := range actions {
			item := &items[positions[action.TicketID]]
			item.Replies = append(item.Replies, action)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	operationsPage(c, items, func(row channelServiceTicket) uint64 { return row.ID }, err)
}

func (s server) createServiceTicket(c *gin.Context) {
	if !operationsRole(c, data.Reseller) {
		return
	}
	var input struct {
		ManufacturerID uint64 `json:"manufacturer_id,string"`
		OrderID        uint64 `json:"order_id,string"`
		Category       string `json:"category"`
		Subject        string `json:"subject"`
		Detail         string `json:"detail"`
		RequestID      string `json:"request_id"`
	}
	if !decode(c, &input) {
		return
	}
	input.Subject, input.Detail = strings.TrimSpace(input.Subject), strings.TrimSpace(input.Detail)
	switch input.Category {
	case "产品使用", "授权与交付", "订单与结算":
	default:
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	if (input.OrderID == 0) == (input.ManufacturerID == 0) || input.Subject == "" || input.Detail == "" ||
		utf8.RuneCountInString(input.Subject) > 80 || utf8.RuneCountInString(input.Detail) > 500 || !retailRequest.MatchString(input.RequestID) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	actor := principal(c)
	db := s.db.WithContext(c.Request.Context())
	body, _ := json.Marshal(input)
	digest := sha256.Sum256(body)
	row := channelServiceTicket{ResellerID: actor.OrganizationID, ManufacturerID: input.ManufacturerID, Category: input.Category,
		Subject: input.Subject, Detail: input.Detail, Status: "open", Revision: 1, CreatedBy: actor.ID,
		RequestID: input.RequestID, RequestHash: hex.EncodeToString(digest[:])}
	if input.OrderID != 0 {
		var order RetailOrder
		if err := db.Select("id", "manufacturer_id").Where("reseller_id = ?", actor.OrganizationID).Take(&order, input.OrderID).Error; err != nil {
			retailRespond(c, nil, err)
			return
		}
		row.OrderID, row.ManufacturerID = &order.ID, order.ManufacturerID
	} else if err := requireRetailPartnership(db, input.ManufacturerID, actor.OrganizationID); err != nil {
		retailRespond(c, nil, err)
		return
	}
	err := db.Create(&row).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		var prior channelServiceTicket
		err = db.Where("created_by = ? AND request_id = ?", actor.ID, input.RequestID).Take(&prior).Error
		if err == nil && prior.RequestHash != row.RequestHash {
			err = retailConflict
		}
		row = prior
	}
	retailRespond(c, gin.H{"id": strconv.FormatUint(row.ID, 10)}, err)
}

func (s server) actServiceTicket(c *gin.Context) {
	actor := principal(c)
	action := c.Param("action")
	role, kind := "dealer", data.Reseller
	start, next := "resolved", "closed"
	switch action {
	case "resolve":
		role, kind, start, next = "vendor", data.Manufacturer, "open", "resolved"
	case "reopen":
		next = "open"
	case "close":
	default:
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	if !operationsRole(c, kind) {
		return
	}
	id, ok := productID(c)
	if !ok {
		return
	}
	var input struct {
		Revision uint32 `json:"revision"`
		Reply    string `json:"reply"`
	}
	if !decode(c, &input) {
		return
	}
	input.Reply = strings.TrimSpace(input.Reply)
	if input.Revision == 0 || input.Revision > 21 || utf8.RuneCountInString(input.Reply) > 500 ||
		(action == "close") != (input.Reply == "") {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row channelServiceTicket
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(retailScope(actor)+" = ?", actor.OrganizationID).Take(&row, id).Error; err != nil {
			return err
		}
		revision := input.Revision + 1
		if row.Revision >= revision {
			var prior channelTicketAction
			err := tx.Where("ticket_id = ? AND revision = ?", id, revision).Take(&prior).Error
			if err == nil && prior.ActorID == actor.ID && prior.Action == action && prior.Text == input.Reply {
				return nil
			}
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return retailConflict
		}
		// After the last allowed reply, the reseller can still close the case.
		if row.Revision != input.Revision || row.Status != start || (action == "reopen" && row.Revision >= 20) {
			return retailConflict
		}
		record := channelTicketAction{TicketID: id, Revision: revision, Role: role, Action: action, ActorID: actor.ID, Text: input.Reply}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"status": next, "revision": revision}).Error
	})
	retailRespond(c, nil, err)
}
