package api

import (
	"database/sql"
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

const operationsPageSize = 50

// A purchase grants visibility of the customer's name for servicing that purchase.
// Assisted orders still require a channel link.
const dealerCustomerOrganizations = `
	SELECT customer_id FROM pl_channel_links WHERE reseller_id = ?
	UNION SELECT customer_id FROM pl_retail_orders WHERE reseller_id = ?`

type dealerCustomerProfile struct {
	ID           uint64
	ResellerID   uint64
	CustomerID   *uint64
	ProspectName *string
	Contact      string
	Note         string
}

func (dealerCustomerProfile) TableName() string { return "pl_dealer_customer_profiles" }

type operationsCustomer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Contact  string `json:"contact"`
	Note     string `json:"note"`
	Linked   bool   `json:"linked"`
	Prospect bool   `json:"prospect"`
}

type renewalEntry struct {
	ID             uint64     `json:"id,string"`
	OrderID        uint64     `json:"order_id,string"`
	Customer       string     `json:"customer"`
	ProductName    string     `json:"product_name"`
	ProductVersion string     `json:"product_version"`
	Source         string     `json:"source"`
	Quantity       uint32     `json:"quantity"`
	PaidAt         time.Time  `json:"paid_at"`
	DueAt          string     `json:"due_at"`
	Note           string     `json:"note"`
	ContactedAt    *time.Time `json:"contacted_at"`
}

type operationsTotals struct {
	Customers              uint64 `json:"customers"`
	Orders                 uint64 `json:"orders"`
	PromotionOrders        uint64 `json:"promotion_orders"`
	Renewals30Days         uint64 `json:"renewals_30_days" gorm:"column:renewals_30_days"`
	PendingCommissionCents uint64 `json:"pending_commission_cents,string"`
}

func (s server) registerDealerOperations(group *gin.RouterGroup) {
	group.Use(func(c *gin.Context) {
		if principal(c).Organization.Kind != data.Reseller {
			respond(c, http.StatusForbidden, "forbidden", nil)
			c.Abort()
			return
		}
		c.Next()
	})
	group.GET("/overview", s.dealerOverview)
	group.GET("/customers", s.dealerCustomers)
	group.POST("/customers", s.saveDealerCustomer)
	group.PUT("/customers/:id", s.saveDealerCustomer)
	group.GET("/renewals", s.dealerRenewals)
	group.PUT("/renewals/:id", s.saveRenewalNote)
}

func annualExpiry(paid time.Time) string {
	year, month, day := paid.In(chinaTime).Date()
	if month == time.February && day == 29 {
		day = 28
	}
	return time.Date(year+1, month, day, 0, 0, 0, 0, chinaTime).Format(time.DateOnly)
}

var chinaTime = time.FixedZone("Asia/Shanghai", 8*60*60)

func (s server) dealerOverview(c *gin.Context) {
	dealer := principal(c).OrganizationID
	today := time.Now().In(chinaTime).Format(time.DateOnly)
	var totals operationsTotals
	err := s.db.WithContext(c.Request.Context()).Raw(`
		SELECT (SELECT COUNT(*) FROM (`+dealerCustomerOrganizations+`) known)
			+ (SELECT COUNT(*) FROM pl_dealer_customer_profiles WHERE reseller_id = ? AND customer_id IS NULL) AS customers,
			COUNT(*) AS orders, COUNT(CASE WHEN source = 'promotion' THEN 1 END) AS promotion_orders,
			COUNT(CASE WHEN renewal_due_on BETWEEN ? AND DATE_ADD(?, INTERVAL 30 DAY) THEN 1 END) AS renewals_30_days,
			COALESCE(SUM(CASE WHEN status = 'settlement' THEN `+retailCommissionCents+` ELSE 0 END), 0) AS pending_commission_cents
		FROM pl_retail_orders WHERE reseller_id = ?`,
		dealer, dealer, dealer, today, today, dealer).Scan(&totals).Error
	retailRespond(c, totals, err)
}

func (s server) dealerCustomers(c *gin.Context) {
	dealer := principal(c).OrganizationID
	after := c.Query("after")
	if after != "" {
		if _, _, ok := customerProfileKey(after); !ok {
			respond(c, http.StatusBadRequest, "invalid_request", nil)
			return
		}
	}
	items := make([]operationsCustomer, 0)
	err := s.db.WithContext(c.Request.Context()).Raw(`
		SELECT * FROM (
			SELECT CONCAT('o', org.id) AS id, org.name, COALESCE(profile.contact, '') AS contact,
				COALESCE(profile.note, '') AS note, link.customer_id IS NOT NULL AS linked, FALSE AS prospect
			FROM (`+dealerCustomerOrganizations+`) known
			JOIN pl_organizations org ON org.id = known.customer_id
			LEFT JOIN pl_channel_links link ON link.customer_id = org.id AND link.reseller_id = ?
			LEFT JOIN pl_dealer_customer_profiles profile ON profile.customer_id = org.id AND profile.reseller_id = ?
			UNION ALL
			SELECT CONCAT('p', id), prospect_name, contact, note, FALSE, TRUE
			FROM pl_dealer_customer_profiles WHERE reseller_id = ? AND customer_id IS NULL
		) customers WHERE id > ? ORDER BY id LIMIT ?`,
		dealer, dealer, dealer, dealer, dealer, after, operationsPageSize+1).Scan(&items).Error
	var next *string
	if len(items) > operationsPageSize {
		items = items[:operationsPageSize]
		next = &items[len(items)-1].ID
	}
	retailRespond(c, gin.H{"items": items, "next_cursor": next}, err)
}

func customerProfileKey(key string) (byte, uint64, bool) {
	if len(key) < 2 || (key[0] != 'p' && key[0] != 'o') {
		return 0, 0, false
	}
	id, err := strconv.ParseUint(key[1:], 10, 64)
	return key[0], id, err == nil && id > 0 && strconv.FormatUint(id, 10) == key[1:]
}

func (s server) saveDealerCustomer(c *gin.Context) {
	var input struct {
		Name    string `json:"name"`
		Contact string `json:"contact"`
		Note    string `json:"note"`
	}
	if !decode(c, &input) {
		return
	}
	dealer := principal(c).OrganizationID
	input.Contact, input.Note, input.Name = strings.TrimSpace(input.Contact), strings.TrimSpace(input.Note), strings.TrimSpace(input.Name)
	key := c.Param("id")
	kind, id, valid := customerProfileKey(key)
	if (key != "" && !valid) || utf8.RuneCountInString(input.Name) > 120 ||
		utf8.RuneCountInString(input.Contact) > 40 || utf8.RuneCountInString(input.Note) > 200 ||
		(kind != 'o' && (input.Name == "" || utf8.RuneCountInString(input.Name) > 80)) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	db := s.db.WithContext(c.Request.Context())
	profile := dealerCustomerProfile{ResellerID: dealer, Contact: input.Contact, Note: input.Note}
	var err error
	switch kind {
	case 'o':
		err = db.Transaction(func(tx *gorm.DB) error {
			var name string
			if err := tx.Raw("SELECT name FROM pl_organizations WHERE id = ? AND id IN ("+dealerCustomerOrganizations+")",
				id, dealer, dealer).Row().Scan(&name); err != nil {
				return err
			}
			if input.Name != "" && input.Name != name {
				return retailConflict
			}
			profile.CustomerID = &id
			return tx.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"contact", "note"})}).Create(&profile).Error
		})
	case 'p':
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND reseller_id = ? AND customer_id IS NULL", id, dealer).Take(&profile).Error; err != nil {
				return err
			}
			return tx.Model(&profile).Updates(map[string]any{"prospect_name": input.Name, "contact": input.Contact, "note": input.Note}).Error
		})
	default:
		profile.ProspectName = &input.Name
		err = db.Create(&profile).Error
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			err = db.Where("reseller_id = ? AND prospect_name = ?", dealer, input.Name).Take(&profile).Error
			if err == nil && (profile.Contact != input.Contact || profile.Note != input.Note || *profile.ProspectName != input.Name) {
				err = retailConflict
			}
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = gorm.ErrRecordNotFound
	}
	if errors.Is(err, retailConflict) || errors.Is(err, gorm.ErrDuplicatedKey) {
		respond(c, http.StatusConflict, "customer_profile_conflict", nil)
		return
	}
	retailRespond(c, nil, err)
}

func (s server) dealerRenewals(c *gin.Context) {
	dealer := principal(c).OrganizationID
	after, err := strconv.ParseUint(c.DefaultQuery("after", "0"), 10, 64)
	if err != nil {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	items := make([]renewalEntry, 0)
	err = s.db.WithContext(c.Request.Context()).Raw(`
		SELECT orders.id, orders.id AS order_id, client.name AS customer, orders.product_name, orders.product_version,
			orders.source, orders.quantity, orders.paid_at, DATE_FORMAT(orders.renewal_due_on, '%Y-%m-%d') AS due_at,
			COALESCE(followup.note, '') AS note, followup.contacted_at
		FROM pl_retail_orders orders JOIN pl_organizations client ON client.id = orders.customer_id
		LEFT JOIN pl_dealer_renewal_notes followup ON orders.id = followup.order_id
		WHERE orders.reseller_id = ? AND orders.renewal_due_on IS NOT NULL AND orders.id > ?
		ORDER BY orders.id LIMIT ?`, dealer, after, operationsPageSize+1).Scan(&items).Error
	var next *string
	if len(items) > operationsPageSize {
		items = items[:operationsPageSize]
		cursor := strconv.FormatUint(items[len(items)-1].ID, 10)
		next = &cursor
	}
	retailRespond(c, gin.H{"items": items, "next_cursor": next}, err)
}

func (s server) saveRenewalNote(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	var input struct {
		Note string `json:"note"`
	}
	if !decode(c, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" || utf8.RuneCountInString(input.Note) > 300 {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row RetailOrder
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reseller_id = ? AND renewal_due_on IS NOT NULL", principal(c).OrganizationID).Take(&row, id).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.Exec(`
			INSERT INTO pl_dealer_renewal_notes (order_id, note, contacted_at) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE contacted_at = IF(BINARY note = BINARY ?, contacted_at, ?), note = ?`,
			row.ID, input.Note, now, input.Note, now, input.Note).Error
	})
	retailRespond(c, nil, err)
}
