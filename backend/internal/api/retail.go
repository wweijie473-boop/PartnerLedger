package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var retailCode = regexp.MustCompile("^[a-f0-9]{32}$")
var retailRequest = regexp.MustCompile("^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$")
var retailConflict = errors.New("retail order conflict")

const maxRetailPrice = 100000000
const retailCommissionCents = "((unit_cents * quantity * commission_bps + 5000) DIV 10000)"

type retailOffer struct {
	ProductID     uint64 `json:"product_id,string" gorm:"primaryKey"`
	UnitCents     uint64 `json:"unit_cents"`
	CommissionBPS uint32 `json:"commission_bps"`
	Active        bool   `json:"active"`
	Annual        bool   `json:"annual"`
}

func (retailOffer) TableName() string { return "pl_retail_offers" }

type offerCard struct {
	ProductID      uint64 `json:"product_id,string"`
	ManufacturerID uint64 `json:"manufacturer_id,string"`
	Manufacturer   string `json:"manufacturer"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	Description    string `json:"description"`
	UnitCents      uint64 `json:"unit_cents"`
	Annual         bool   `json:"annual"`
}

type retailPromotion struct {
	ID             uint64 `json:"-"`
	ResellerID     uint64 `json:"-"`
	ManufacturerID uint64 `json:"manufacturer_id,string"`
	Code           string `json:"code"`
}

func (retailPromotion) TableName() string { return "pl_retail_promotions" }

type RetailOrder struct {
	ID             uint64     `json:"id,string"`
	ManufacturerID uint64     `json:"-"`
	ResellerID     uint64     `json:"-"`
	CustomerID     uint64     `json:"-"`
	ProductID      uint64     `json:"-"`
	ProductName    string     `json:"product_name"`
	ProductVersion string     `json:"product_version"`
	Source         string     `json:"source"`
	Status         string     `json:"status"`
	Quantity       uint32     `json:"quantity"`
	UnitCents      uint64     `json:"unit_cents"`
	CommissionBPS  uint32     `json:"-"`
	CreatedBy      uint64     `json:"-"`
	RequestID      string     `json:"-"`
	RequestHash    string     `json:"-"`
	ReceiptRef     *string    `json:"-"`
	PayoutRef      *string    `json:"-"`
	PaidAt         *time.Time `json:"paid_at"`
	Annual         bool       `json:"annual"`
	SettledAt      *time.Time `json:"-"`
	RemindedAt     *time.Time `json:"-"`
	ReminderCount  uint32     `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (RetailOrder) TableName() string { return "pl_retail_orders" }

type retailOrderRow struct {
	RetailOrder
	Manufacturer string            `json:"manufacturer"`
	Reseller     string            `json:"reseller"`
	Customer     string            `json:"customer"`
	TotalCents   uint64            `json:"total_cents" gorm:"-"`
	Commission   *retailCommission `json:"commission,omitempty" gorm:"-"`
	Receipt      *string           `json:"receipt_ref,omitempty" gorm:"-"`
	Payout       *string           `json:"payout_ref,omitempty" gorm:"-"`
}

type retailSettlementSummary struct {
	SettlementCents uint64 `json:"settlement_cents,string"`
	SettledCents    uint64 `json:"settled_cents,string"`
	Reminded        uint64 `json:"reminded"`
}
type retailCommission struct {
	RateBPS       uint32     `json:"rate_bps"`
	Cents         uint64     `json:"cents"`
	SettledAt     *time.Time `json:"settled_at"`
	RemindedAt    *time.Time `json:"reminded_at"`
	ReminderCount uint32     `json:"reminder_count"`
}

func (s server) registerRetail(api, signedIn *gin.RouterGroup) {
	api.GET("/retail/v1/store/:code", s.retailStore)
	retail := signedIn.Group("/retail/v1")
	retail.GET("/offers", s.retailOffers)
	retail.GET("/manufacturer-offers", s.listManufacturerOffers)
	retail.PUT("/manufacturer-offers/:id", s.setRetailOffer)
	retail.POST("/promotion", s.makeRetailPromotion)
	retail.GET("/orders", s.listRetailOrders)
	retail.POST("/orders", s.createRetailOrder)
	retail.GET("/orders/requests/:requestId", s.retailOrderByRequest)
	retail.POST("/orders/:id/:action", s.advanceRetailOrder)
	s.registerDealerOperations(retail.Group("/operations"))
	s.registerChannelOperations(retail.Group("/operations"))
}

func publishedOffers(db *gorm.DB) *gorm.DB {
	return db.Table("pl_retail_offers AS offer").
		Joins("JOIN pl_products AS product ON product.id = offer.product_id").
		Joins("JOIN pl_organizations AS maker ON maker.id = product.manufacturer_id").
		Where("offer.active = ? AND product.published = ?", true, true)
}

const offerFields = "product.id AS product_id, product.manufacturer_id, maker.name AS manufacturer, product.name, product.version, product.description, offer.unit_cents, offer.annual"

func retailRespond(c *gin.Context, value any, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		respond(c, http.StatusNotFound, "not_found", nil)
	case errors.Is(err, retailConflict), errors.Is(err, gorm.ErrDuplicatedKey):
		respond(c, http.StatusConflict, "retail_conflict", nil)
	case err != nil:
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
	default:
		respond(c, http.StatusOK, "success", value)
	}
}

func (s server) retailStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	code := c.Param("code")
	if !retailCode.MatchString(code) {
		retailRespond(c, nil, gorm.ErrRecordNotFound)
		return
	}
	db := s.db.WithContext(c.Request.Context())
	var promo retailPromotion
	if err := db.Where("code = ?", code).First(&promo).Error; err != nil {
		retailRespond(c, nil, err)
		return
	}
	retailOfferPage(c, publishedOffers(db).Where("maker.id = ?", promo.ManufacturerID))
}

func (s server) retailOffers(c *gin.Context) {
	if principal(c).Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	retailOfferPage(c, publishedOffers(s.db.WithContext(c.Request.Context())))
}

func retailOfferPage(c *gin.Context, query *gorm.DB) {
	after, err := strconv.ParseUint(c.DefaultQuery("after", "0"), 10, 64)
	if err != nil {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	items := make([]offerCard, 0)
	err = query.Select(offerFields).Where("product.id > ?", after).Order("product.id").Limit(101).Scan(&items).Error
	var next *string
	if len(items) > 100 {
		items = items[:100]
		cursor := strconv.FormatUint(items[99].ProductID, 10)
		next = &cursor
	}
	retailRespond(c, gin.H{"items": items, "next_cursor": next}, err)
}

func (s server) listManufacturerOffers(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Manufacturer {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	items := make([]retailOffer, 0)
	err := s.db.WithContext(c.Request.Context()).Model(&retailOffer{}).
		Joins("JOIN pl_products ON pl_products.id = pl_retail_offers.product_id").
		Where("pl_products.manufacturer_id = ?", actor.OrganizationID).Order("product_id").Find(&items).Error
	retailRespond(c, gin.H{"items": items}, err)
}

func (s server) setRetailOffer(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Manufacturer {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	id, ok := productID(c)
	if !ok {
		return
	}
	var input struct {
		UnitCents     uint64 `json:"unit_cents"`
		CommissionBPS uint32 `json:"commission_bps"`
		Active        bool   `json:"active"`
		Annual        *bool  `json:"annual"`
	}
	if !decode(c, &input) {
		return
	}
	if input.UnitCents == 0 || input.UnitCents > maxRetailPrice || input.CommissionBPS > 10000 {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	db := s.db.WithContext(c.Request.Context())
	row := retailOffer{ProductID: id, UnitCents: input.UnitCents, CommissionBPS: input.CommissionBPS, Active: input.Active}
	err := db.Transaction(func(tx *gorm.DB) error {
		var product data.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("manufacturer_id = ?", actor.OrganizationID).First(&product, id).Error; err != nil {
			return err
		}
		if input.Active && !product.Published {
			return retailConflict
		}
		if input.Annual != nil {
			row.Annual = *input.Annual
		} else {
			var current retailOffer
			err := tx.Take(&current, "product_id = ?", id).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			row.Annual = current.Annual
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error
	})
	retailRespond(c, row, err)
}

func (s server) makeRetailPromotion(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		ManufacturerID uint64 `json:"manufacturer_id,string"`
	}
	if !decode(c, &input) {
		return
	}
	code := make([]byte, 16)
	if _, err := rand.Read(code); err != nil {
		retailRespond(c, nil, err)
		return
	}
	row := retailPromotion{ResellerID: actor.OrganizationID, ManufacturerID: input.ManufacturerID, Code: hex.EncodeToString(code)}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := publishedOffers(tx).Where("maker.id = ?", input.ManufacturerID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Where("reseller_id = ? AND manufacturer_id = ?", actor.OrganizationID, input.ManufacturerID).First(&row).Error
	})
	retailRespond(c, row, err)
}

func retailScope(actor data.User) string {
	switch actor.Organization.Kind {
	case data.Manufacturer:
		return "manufacturer_id"
	case data.Reseller:
		return "reseller_id"
	default:
		return "customer_id"
	}
}

func (s server) listRetailOrders(c *gin.Context) {
	actor := principal(c)
	db := s.db.WithContext(c.Request.Context())
	scope := retailScope(actor)
	commissionView := c.Query("view") == "commission" && actor.Organization.Kind != data.Customer
	before, err := strconv.ParseUint(c.DefaultQuery("before", "0"), 10, 64)
	if err != nil {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	items := make([]retailOrderRow, 0)
	query := db.Table("pl_retail_orders AS orders").
		Select("orders.*, maker.name AS manufacturer, partner.name AS reseller, client.name AS customer").
		Joins("JOIN pl_organizations AS maker ON maker.id = orders.manufacturer_id").
		Joins("JOIN pl_organizations AS partner ON partner.id = orders.reseller_id").
		Joins("JOIN pl_organizations AS client ON client.id = orders.customer_id").
		Where("orders."+scope+" = ?", actor.OrganizationID)
	if rawID, exists := c.GetQuery("order_id"); exists {
		id, parseErr := strconv.ParseUint(rawID, 10, 64)
		if parseErr != nil || id == 0 {
			respond(c, http.StatusBadRequest, "invalid_request", nil)
			return
		}
		query = query.Where("orders.id = ?", id)
	}
	if before != 0 {
		query = query.Where("orders.id < ?", before)
	}
	switch c.Query("status") {
	case "":
		if commissionView {
			query = query.Where("orders.status IN ?", []string{"settlement", "settled"})
		}
	case "paid":
		if actor.Organization.Kind != data.Customer {
			respond(c, http.StatusBadRequest, "invalid_request", nil)
			return
		}
		query = query.Where("orders.status IN ?", []string{"settlement", "settled"})
	case "confirmation", "payment", "declined":
		query = query.Where("orders.status = ?", c.Query("status"))
	case "settlement", "settled":
		if actor.Organization.Kind == data.Customer {
			respond(c, http.StatusBadRequest, "invalid_request", nil)
			return
		}
		query = query.Where("orders.status = ?", c.Query("status"))
	default:
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	err = query.Order("orders.id DESC").Limit(51).Scan(&items).Error
	if err != nil {
		retailRespond(c, nil, err)
		return
	}
	var next *string
	if len(items) > 50 {
		items = items[:50]
		cursor := strconv.FormatUint(items[49].ID, 10)
		next = &cursor
	}
	for i := range items {
		item := &items[i]
		item.TotalCents = item.UnitCents * uint64(item.Quantity)
		if actor.Organization.Kind != data.Customer {
			item.Commission = &retailCommission{
				RateBPS: item.CommissionBPS, Cents: (item.TotalCents*uint64(item.CommissionBPS) + 5000) / 10000,
				SettledAt: item.SettledAt, RemindedAt: item.RemindedAt, ReminderCount: item.ReminderCount,
			}
			item.Payout = item.PayoutRef
			if actor.Organization.Kind == data.Manufacturer {
				item.Receipt = item.ReceiptRef
			}
		} else {
			if item.Status == "settlement" || item.Status == "settled" {
				item.Status = "paid"
			}
		}
	}
	result := gin.H{"items": items, "next_cursor": next}
	if commissionView {
		var summary retailSettlementSummary
		err = db.Model(&RetailOrder{}).Where(scope+" = ? AND status IN ?", actor.OrganizationID, []string{"settlement", "settled"}).
			Select("COALESCE(SUM(CASE WHEN status = 'settlement' THEN " + retailCommissionCents + " ELSE 0 END), 0) AS settlement_cents, " +
				"COALESCE(SUM(CASE WHEN status = 'settled' THEN " + retailCommissionCents + " ELSE 0 END), 0) AS settled_cents, " +
				"COUNT(CASE WHEN status = 'settlement' AND reminder_count > 0 THEN 1 END) AS reminded").Scan(&summary).Error
		result["summary"] = summary
	}
	retailRespond(c, result, err)
}

type retailOrderInput struct {
	ProductID         uint64 `json:"product_id,string"`
	CustomerID        uint64 `json:"customer_id,string,omitempty"`
	Code              string `json:"code,omitempty"`
	Quantity          uint32 `json:"quantity"`
	ExpectedUnitCents uint64 `json:"expected_unit_cents"`
	ExpectedAnnual    *bool  `json:"expected_annual,omitempty"`
	RequestID         string `json:"request_id"`
}

func (s server) retailOrderByRequest(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Customer && actor.Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	requestID := c.Param("requestId")
	if !retailRequest.MatchString(requestID) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	var row RetailOrder
	err := s.db.WithContext(c.Request.Context()).Select("id").
		Where("created_by = ? AND request_id = ? AND "+retailScope(actor)+" = ?", actor.ID, requestID, actor.OrganizationID).
		Take(&row).Error
	retailRespond(c, gin.H{"id": strconv.FormatUint(row.ID, 10)}, err)
}

func (s server) createRetailOrder(c *gin.Context) {
	actor := principal(c)
	var input retailOrderInput
	if !decode(c, &input) {
		return
	}
	assisted := actor.Organization.Kind == data.Reseller
	direct := actor.Organization.Kind == data.Customer
	if !assisted && !direct {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	if input.ProductID == 0 || input.Quantity < 1 || input.Quantity > 10000 || !retailRequest.MatchString(input.RequestID) ||
		(assisted && (input.CustomerID == 0 || input.Code != "")) ||
		(direct && (!retailCode.MatchString(input.Code) || input.CustomerID != 0)) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	raw, _ := json.Marshal(input)
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	db := s.db.WithContext(c.Request.Context())
	var row RetailOrder
	lookup := func(tx *gorm.DB) error {
		err := tx.Where("created_by = ? AND request_id = ? AND "+retailScope(actor)+" = ?", actor.ID, input.RequestID, actor.OrganizationID).First(&row).Error
		if err == nil && row.RequestHash != fingerprint {
			return retailConflict
		}
		return err
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		// Serialize attempts before reading the order or current terms. A retry must see
		// an earlier committed order even if the manufacturer has since changed the offer.
		var creator data.User
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ?", actor.OrganizationID).Take(&creator, actor.ID).Error; err != nil {
			return err
		}
		if err := lookup(tx); !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var offer offerCard
		if err := publishedOffers(tx).Where("product.id = ?", input.ProductID).Select(offerFields).Take(&offer).Error; err != nil {
			return err
		}
		if offer.UnitCents != input.ExpectedUnitCents || (input.ExpectedAnnual != nil && offer.Annual != *input.ExpectedAnnual) {
			return retailConflict
		}
		var rate retailOffer
		if err := tx.First(&rate, "product_id = ?", offer.ProductID).Error; err != nil {
			return err
		}
		row = RetailOrder{
			ManufacturerID: offer.ManufacturerID, ProductID: offer.ProductID, ProductName: offer.Name, ProductVersion: offer.Version,
			Quantity: input.Quantity, UnitCents: offer.UnitCents, CommissionBPS: rate.CommissionBPS,
			Annual:    offer.Annual,
			CreatedBy: actor.ID, RequestID: input.RequestID, RequestHash: fingerprint,
		}
		if assisted {
			var count int64
			if err := tx.Table("pl_channel_links").Where("reseller_id = ? AND customer_id = ?", actor.OrganizationID, input.CustomerID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
			row.ResellerID, row.CustomerID, row.Source, row.Status = actor.OrganizationID, input.CustomerID, "assisted", "confirmation"
		} else {
			var promo retailPromotion
			if err := tx.Where("code = ? AND manufacturer_id = ?", input.Code, offer.ManufacturerID).First(&promo).Error; err != nil {
				return err
			}
			row.ResellerID, row.CustomerID, row.Source, row.Status = promo.ResellerID, actor.OrganizationID, "promotion", "payment"
		}
		return tx.Create(&row).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		err = lookup(db)
	}
	retailRespond(c, gin.H{"id": strconv.FormatUint(row.ID, 10)}, err)
}

func (s server) advanceRetailOrder(c *gin.Context) {
	actor := principal(c)
	id, ok := productID(c)
	if !ok {
		return
	}
	var input struct {
		Reference string `json:"reference,omitempty"`
	}
	if !decode(c, &input) {
		return
	}
	ref := strings.TrimSpace(input.Reference)
	action := c.Param("action")
	kind := actor.Organization.Kind
	allowed := (kind == data.Customer && (action == "confirm" || action == "decline")) ||
		(kind == data.Reseller && action == "remind") || (kind == data.Manufacturer && (action == "receipt" || action == "settle"))
	if !allowed {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	if (action == "receipt" || action == "settle") && (ref == "" || utf8.RuneCountInString(ref) > 100) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	db := s.db.WithContext(c.Request.Context())
	now := time.Now().UTC()
	err := db.Transaction(func(tx *gorm.DB) error {
		var row RetailOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(retailScope(actor)+" = ?", actor.OrganizationID).First(&row, id).Error; err != nil {
			return err
		}
		changes := map[string]any{}
		switch action {
		case "confirm", "decline":
			next := "payment"
			if action == "decline" {
				next = "declined"
			}
			if row.Status == next || (action == "confirm" && row.PaidAt != nil) {
				return nil
			}
			if row.Status != "confirmation" {
				return retailConflict
			}
			changes["status"] = next
		case "receipt":
			if row.ReceiptRef != nil && *row.ReceiptRef == ref {
				return nil
			}
			if row.Status != "payment" {
				return retailConflict
			}
			next := "settlement"
			if row.CommissionBPS == 0 {
				next = "settled"
				changes["settled_at"] = now
			}
			changes["status"], changes["receipt_ref"], changes["paid_at"] = next, ref, now
			if row.Annual {
				changes["renewal_due_on"] = annualExpiry(now)
			}
		case "remind":
			if row.Status != "settlement" {
				return retailConflict
			}
			if row.RemindedAt != nil && now.Sub(*row.RemindedAt) < 24*time.Hour {
				return nil
			}
			changes["reminded_at"], changes["reminder_count"] = now, row.ReminderCount+1
		case "settle":
			if row.PayoutRef != nil && *row.PayoutRef == ref {
				return nil
			}
			if row.Status != "settlement" {
				return retailConflict
			}
			changes["status"], changes["payout_ref"], changes["settled_at"] = "settled", ref, now
		}
		return tx.Model(&row).Updates(changes).Error
	})
	retailRespond(c, nil, err)
}
