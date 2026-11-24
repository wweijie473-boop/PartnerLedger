package api

import (
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

var insufficientAllowance = errors.New("insufficient service allowance")

type channelFinanceRequest struct {
	ID              uint64     `json:"id,string"`
	ManufacturerID  uint64     `json:"manufacturer_id,string"`
	ResellerID      uint64     `json:"reseller_id,string"`
	Kind            string     `json:"kind"`
	OrderID         *uint64    `json:"order_id,string,omitempty"`
	Reference       *string    `json:"reference,omitempty"`
	Title           string     `json:"title"`
	AmountCents     uint64     `json:"amount_cents,string"`
	Status          string     `json:"status"`
	ResultReference string     `json:"result_reference"`
	CreatedBy       uint64     `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	HandledBy       *uint64    `json:"-"`
	HandledAt       *time.Time `json:"handled_at"`
}

func (channelFinanceRequest) TableName() string { return "pl_channel_finance_requests" }

type channelServiceAllowance struct {
	ManufacturerID uint64 `gorm:"primaryKey;autoIncrement:false"`
	ResellerID     uint64 `gorm:"primaryKey;autoIncrement:false"`
	LimitCents     uint64
	UpdatedBy      uint64
	UpdatedAt      time.Time
}

func (channelServiceAllowance) TableName() string { return "pl_channel_service_allowances" }

type operationsPartner struct {
	ID           uint64 `json:"id,string"`
	Name         string `json:"name"`
	LimitCents   uint64 `json:"limit_cents,string"`
	PendingCents uint64 `json:"pending_cents,string"`
	PaidCents    uint64 `json:"paid_cents,string"`
}

func (s server) registerChannelOperations(group *gin.RouterGroup) {
	group.Use(func(c *gin.Context) {
		kind := principal(c).Organization.Kind
		if kind != data.Reseller && kind != data.Manufacturer {
			respond(c, http.StatusForbidden, "forbidden", nil)
			c.Abort()
			return
		}
		c.Next()
	})
	group.GET("/partners", s.operationsPartners)
	group.GET("/finance", s.listFinanceRequests)
	group.POST("/finance/:kind", s.createFinanceRequest)
	group.POST("/finance/:kind/:id/complete", s.completeFinanceRequest)
	group.PUT("/allowances/:id", s.setServiceAllowance)
	group.GET("/tickets", s.listServiceTickets)
	group.POST("/tickets", s.createServiceTicket)
	group.POST("/tickets/:id/:action", s.actServiceTicket)
}

func operationsCursor(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.DefaultQuery("after", "0"), 10, 64)
	if err != nil {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
	}
	return id, err == nil
}

func operationsPage[T any](c *gin.Context, items []T, key func(T) uint64, err error) {
	var next *string
	if len(items) > operationsPageSize {
		items = items[:operationsPageSize]
		id := strconv.FormatUint(key(items[len(items)-1]), 10)
		next = &id
	}
	retailRespond(c, gin.H{"items": items, "next_cursor": next}, err)
}

func operationsRole(c *gin.Context, kind string) bool {
	if principal(c).Organization.Kind == kind {
		return true
	}
	respond(c, http.StatusForbidden, "forbidden", nil)
	return false
}

func requireRetailPartnership(tx *gorm.DB, manufacturer, reseller uint64) error {
	var order RetailOrder
	return tx.Select("id").Where("manufacturer_id = ? AND reseller_id = ?", manufacturer, reseller).Take(&order).Error
}

func (s server) operationsPartners(c *gin.Context) {
	after, ok := operationsCursor(c)
	if !ok {
		return
	}
	actor := principal(c)
	own := retailScope(actor)
	peer := "manufacturer_id"
	if actor.Organization.Kind == data.Manufacturer {
		peer = "reseller_id"
	}
	items := make([]operationsPartner, 0)
	err := s.db.WithContext(c.Request.Context()).Raw(`
		SELECT org.id, org.name, COALESCE(allowance.limit_cents, 0) AS limit_cents,
			COALESCE(SUM(CASE WHEN claim.status = 'pending' THEN claim.amount_cents END), 0) AS pending_cents,
			COALESCE(SUM(CASE WHEN claim.status = 'paid' THEN claim.amount_cents END), 0) AS paid_cents
		FROM (SELECT DISTINCT `+peer+` AS id FROM pl_retail_orders
			WHERE `+own+` = ? AND `+peer+` > ? ORDER BY id LIMIT ?) partners
		JOIN pl_organizations org ON org.id = partners.id
		LEFT JOIN pl_channel_service_allowances allowance ON allowance.`+own+` = ? AND allowance.`+peer+` = org.id
		LEFT JOIN pl_channel_finance_requests claim ON claim.`+own+` = ? AND claim.`+peer+` = org.id AND claim.kind = 'claims'
		GROUP BY org.id, org.name, allowance.limit_cents ORDER BY org.id`,
		actor.OrganizationID, after, operationsPageSize+1, actor.OrganizationID, actor.OrganizationID).Scan(&items).Error
	operationsPage(c, items, func(row operationsPartner) uint64 { return row.ID }, err)
}

func (s server) listFinanceRequests(c *gin.Context) {
	after, ok := operationsCursor(c)
	if !ok {
		return
	}
	actor := principal(c)
	items := make([]channelFinanceRequest, 0)
	err := s.db.WithContext(c.Request.Context()).Where(retailScope(actor)+" = ? AND id > ?", actor.OrganizationID, after).
		Order("id").Limit(operationsPageSize + 1).Find(&items).Error
	operationsPage(c, items, func(row channelFinanceRequest) uint64 { return row.ID }, err)
}

func financeStatus(kind string) (start, end string) {
	switch kind {
	case "topups":
		return "pending", "credited"
	case "invoices":
		return "requested", "issued"
	case "claims":
		return "pending", "paid"
	}
	return "", ""
}

func (s server) createFinanceRequest(c *gin.Context) {
	if !operationsRole(c, data.Reseller) {
		return
	}
	var input struct {
		ManufacturerID uint64 `json:"manufacturer_id,string"`
		OrderID        uint64 `json:"order_id,string"`
		Reference      string `json:"reference"`
		Title          string `json:"title"`
		AmountCents    uint64 `json:"amount_cents"`
	}
	if !decode(c, &input) {
		return
	}
	kind := c.Param("kind")
	start, _ := financeStatus(kind)
	input.Reference, input.Title = strings.TrimSpace(input.Reference), strings.TrimSpace(input.Title)
	invalid := start == ""
	if kind == "invoices" {
		invalid = input.OrderID == 0 || input.Title == "" || utf8.RuneCountInString(input.Title) > 80 ||
			input.ManufacturerID != 0 || input.AmountCents != 0 || input.Reference != ""
	} else {
		invalid = invalid || input.ManufacturerID == 0 || input.Reference == "" || utf8.RuneCountInString(input.Reference) > 80 ||
			input.AmountCents == 0 || input.AmountCents > maxRetailPrice || input.OrderID != 0 || input.Title != ""
	}
	if invalid {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	actor := principal(c)
	db := s.db.WithContext(c.Request.Context())
	row := channelFinanceRequest{ManufacturerID: input.ManufacturerID, ResellerID: actor.OrganizationID, Kind: kind,
		Title: input.Title, AmountCents: input.AmountCents, Status: start, CreatedBy: actor.ID}
	if kind == "invoices" {
		var order RetailOrder
		if err := db.Where("reseller_id = ?", actor.OrganizationID).Take(&order, input.OrderID).Error; err != nil {
			retailRespond(c, nil, err)
			return
		}
		if order.PaidAt == nil {
			retailRespond(c, nil, retailConflict)
			return
		}
		row.OrderID, row.ManufacturerID = &order.ID, order.ManufacturerID
		row.AmountCents = order.UnitCents * uint64(order.Quantity)
	} else {
		if err := requireRetailPartnership(db, input.ManufacturerID, actor.OrganizationID); err != nil {
			retailRespond(c, nil, err)
			return
		}
		row.Reference = &input.Reference
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var allowance channelServiceAllowance
		if kind == "claims" {
			if err := lockAllowance(tx, row.ManufacturerID, row.ResellerID, &allowance); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return insufficientAllowance
				}
				return err
			}
		}
		if err := findFinanceReplay(tx, row); !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if kind == "claims" {
			used, err := allocatedAmount(tx, row.ManufacturerID, row.ResellerID)
			if err != nil {
				return err
			}
			if used > allowance.LimitCents || row.AmountCents > allowance.LimitCents-used {
				return insufficientAllowance
			}
		}
		return tx.Create(&row).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		err = findFinanceReplay(db, row)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = retailConflict
		}
	}
	if errors.Is(err, insufficientAllowance) {
		respond(c, http.StatusConflict, "insufficient_service_allowance", nil)
		return
	}
	retailRespond(c, nil, err)
}

func findFinanceReplay(db *gorm.DB, row channelFinanceRequest) error {
	query := db.Where("manufacturer_id = ? AND kind = ?", row.ManufacturerID, row.Kind)
	if row.OrderID != nil {
		query = query.Where("order_id = ?", *row.OrderID)
	} else {
		query = query.Where("reference = ?", *row.Reference)
	}
	var prior channelFinanceRequest
	if err := query.Take(&prior).Error; err != nil {
		return err
	}
	if prior.ResellerID != row.ResellerID || prior.AmountCents != row.AmountCents || prior.Title != row.Title {
		return retailConflict
	}
	return nil
}

func lockAllowance(tx *gorm.DB, manufacturer, reseller uint64, allowance *channelServiceAllowance) error {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("manufacturer_id = ? AND reseller_id = ?", manufacturer, reseller).Take(allowance).Error
}

func allocatedAmount(tx *gorm.DB, manufacturer, reseller uint64) (uint64, error) {
	var used uint64
	err := tx.Model(&channelFinanceRequest{}).Where("manufacturer_id = ? AND reseller_id = ? AND kind = 'claims'", manufacturer, reseller).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&used).Error
	return used, err
}

func (s server) setServiceAllowance(c *gin.Context) {
	if !operationsRole(c, data.Manufacturer) {
		return
	}
	reseller, ok := productID(c)
	if !ok {
		return
	}
	var input struct {
		LimitCents *uint64 `json:"limit_cents"`
	}
	if !decode(c, &input) {
		return
	}
	if input.LimitCents == nil || *input.LimitCents > maxRetailPrice {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	actor := principal(c)
	db := s.db.WithContext(c.Request.Context())
	if err := requireRetailPartnership(db, actor.OrganizationID, reseller); err != nil {
		retailRespond(c, nil, err)
		return
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		allowance := channelServiceAllowance{ManufacturerID: actor.OrganizationID, ResellerID: reseller, UpdatedBy: actor.ID}
		// The insert or no-op update takes the same row lock used to reserve claims.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&allowance).Error; err != nil {
			return err
		}
		used, err := allocatedAmount(tx, actor.OrganizationID, reseller)
		if err != nil {
			return err
		}
		if *input.LimitCents < used {
			return insufficientAllowance
		}
		return tx.Model(&allowance).Updates(map[string]any{"limit_cents": *input.LimitCents, "updated_by": actor.ID}).Error
	})
	if errors.Is(err, insufficientAllowance) {
		respond(c, http.StatusConflict, "insufficient_service_allowance", nil)
		return
	}
	retailRespond(c, nil, err)
}

func (s server) completeFinanceRequest(c *gin.Context) {
	if !operationsRole(c, data.Manufacturer) {
		return
	}
	id, ok := productID(c)
	if !ok {
		return
	}
	var input struct {
		Reference string `json:"result_reference"`
	}
	if !decode(c, &input) {
		return
	}
	kind := c.Param("kind")
	start, end := financeStatus(kind)
	input.Reference = strings.TrimSpace(input.Reference)
	if start == "" || input.Reference == "" || utf8.RuneCountInString(input.Reference) > 80 {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	actor := principal(c)
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row channelFinanceRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("manufacturer_id = ? AND kind = ?", actor.OrganizationID, kind).Take(&row, id).Error; err != nil {
			return err
		}
		if row.Status == end && row.ResultReference == input.Reference {
			return nil
		}
		if row.Status != start {
			return retailConflict
		}
		return tx.Model(&row).Updates(map[string]any{"status": end, "result_reference": input.Reference,
			"handled_by": actor.ID, "handled_at": time.Now().UTC()}).Error
	})
	retailRespond(c, nil, err)
}
