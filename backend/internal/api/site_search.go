package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/catalog"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

const siteResultLimit = 3

type siteSearchHit struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Href        string `json:"href"`
}

type siteSearchInput struct {
	terms  []string
	number uint64
}

func parseSiteSearch(raw string) (siteSearchInput, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || utf8.RuneCountInString(raw) > 120 {
		return siteSearchInput{}, false
	}
	terms := strings.Fields(raw)
	if len(terms) > 8 {
		return siteSearchInput{}, false
	}
	var number uint64
	if len(terms) == 1 {
		value := strings.TrimPrefix(terms[0], "#")
		if n, err := strconv.ParseUint(value, 10, 64); err == nil {
			number = n
		}
	}
	return siteSearchInput{terms: terms, number: number}, true
}

func siteSearchScope(actor data.User) (role, orderView, ticketsView string) {
	switch actor.Organization.Kind {
	case data.Reseller:
		return "dealer", "retail-orders", "service-tickets"
	case data.Manufacturer:
		return "vendor", "manufacturer-receipts", "vendor-tickets"
	case data.Customer:
		return "customer", "retail-orders", ""
	default:
		return "", "", ""
	}
}

func searchLink(role, view, key string, id uint64) string {
	values := url.Values{"view": {view}, key: {strconv.FormatUint(id, 10)}}
	return "/login/" + role + "?" + values.Encode()
}

func (s server) searchSiteRecords(c *gin.Context) {
	input, ok := parseSiteSearch(c.Query("q"))
	if !ok {
		respond(c, http.StatusBadRequest, "invalid_site_search", nil)
		return
	}
	actor := principal(c)
	role, orderView, ticketsView := siteSearchScope(actor)
	if role == "" {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	items := make([]siteSearchHit, 0, 4*siteResultLimit)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projects, err := channel.SearchProjects(tx, actor, input.terms, input.number, siteResultLimit)
		if err != nil {
			return err
		}
		for _, project := range projects {
			description := "项目 #" + strconv.FormatUint(project.ID, 10)
			if project.OrderID != 0 {
				description += " · 渠道订单 #" + strconv.FormatUint(project.OrderID, 10)
			}
			items = append(items, siteSearchHit{"project", project.Title, description, searchLink(role, "projects", "project", project.ID)})
		}

		var orders []RetailOrder
		query := tx.Model(&RetailOrder{}).Where(retailScope(actor)+" = ?", actor.OrganizationID)
		if err := input.filter(query, "id", "CONCAT_WS(' ', id, '直购订单', product_name, product_version)").
			Select("id", "product_name", "product_version").Order("id DESC").Limit(siteResultLimit).Find(&orders).Error; err != nil {
			return err
		}
		for _, order := range orders {
			items = append(items, siteSearchHit{"order", "直购订单 #" + strconv.FormatUint(order.ID, 10),
				order.ProductName + " · " + order.ProductVersion, searchLink(role, orderView, "order_id", order.ID)})
		}

		if ticketsView != "" {
			var tickets []channelServiceTicket
			query := tx.Model(&channelServiceTicket{}).Where(retailScope(actor)+" = ?", actor.OrganizationID)
			if err := input.filter(query, "id", "CONCAT_WS(' ', id, '工单', subject, category)").
				Select("id", "subject", "category").Order("id DESC").Limit(siteResultLimit).Find(&tickets).Error; err != nil {
				return err
			}
			for _, ticket := range tickets {
				items = append(items, siteSearchHit{"ticket", ticket.Subject, "工单 #" + strconv.FormatUint(ticket.ID, 10) + " · " + ticket.Category,
					searchLink(role, ticketsView, "ticket", ticket.ID)})
			}
		}

		if role == "customer" {
			return nil
		}
		var products []data.Product
		if err := input.filter(catalog.Visible(tx, actor), "id", "CONCAT_WS(' ', id, '产品', name, version)").
			Select("id", "name", "version").Order("id DESC").Limit(siteResultLimit).Find(&products).Error; err != nil {
			return err
		}
		for _, product := range products {
			items = append(items, siteSearchHit{"product", product.Name, "产品 · " + product.Version, searchLink(role, "catalog", "product", product.ID)})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	respond(c, http.StatusOK, "success", gin.H{"items": items})
}

func (input siteSearchInput) filter(query *gorm.DB, numberColumn, expression string) *gorm.DB {
	if input.number != 0 {
		return query.Where(numberColumn+" = ?", input.number)
	}
	return data.MatchSearchTerms(query, expression, input.terms)
}
