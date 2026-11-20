package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/catalog"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

type productInput struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Published   bool   `json:"published"`
}

func (s server) catalog(c *gin.Context) *gorm.DB {
	return catalog.Visible(s.db.WithContext(c.Request.Context()), principal(c))
}

func (s server) listProducts(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "20"))
	after, afterErr := strconv.ParseUint(c.DefaultQuery("after", "0"), 10, 64)
	if limitErr != nil || afterErr != nil || limit < 1 || limit > 100 {
		respond(c, http.StatusBadRequest, "invalid_pagination", nil)
		return
	}
	products := make([]data.Product, 0)
	query := s.catalog(c).Where("id > ?", after).Order("id ASC").Limit(limit + 1)
	if err := query.Find(&products).Error; err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	var next *string
	if len(products) > limit {
		products = products[:limit]
		cursor := strconv.FormatUint(products[limit-1].ID, 10)
		next = &cursor
	}
	respond(c, http.StatusOK, "success", gin.H{"items": products, "next_cursor": next})
}

func (s server) getProduct(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	var product data.Product
	err := s.catalog(c).First(&product, id).Error
	productResult(c, http.StatusOK, product, err)
}

func (s server) createProduct(c *gin.Context) {
	input, ok := readProduct(c)
	if !ok {
		return
	}
	product := data.Product{
		ManufacturerID: principal(c).OrganizationID,
		Name:           input.Name,
		Version:        input.Version,
		Description:    input.Description,
		Published:      input.Published,
	}
	err := s.db.WithContext(c.Request.Context()).Create(&product).Error
	productResult(c, http.StatusCreated, product, err)
}

func (s server) updateProduct(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	input, ok := readProduct(c)
	if !ok {
		return
	}
	manufacturer := principal(c).OrganizationID
	var product data.Product
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		write := tx.Model(&data.Product{}).Where("id = ? AND manufacturer_id = ?", id, manufacturer).Updates(map[string]any{
			"name": input.Name, "version": input.Version, "description": input.Description, "published": input.Published,
		})
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("manufacturer_id = ?", manufacturer).First(&product, id).Error
	})
	productResult(c, http.StatusOK, product, err)
}

func productID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respond(c, http.StatusBadRequest, "invalid_product_id", nil)
		return 0, false
	}
	return id, true
}

func readProduct(c *gin.Context) (productInput, bool) {
	var p productInput
	if !decode(c, &p) {
		return p, false
	}
	p.Name, p.Version, p.Description = strings.TrimSpace(p.Name), strings.TrimSpace(p.Version), strings.TrimSpace(p.Description)
	if p.Name == "" || p.Version == "" || utf8.RuneCountInString(p.Name) > 150 ||
		utf8.RuneCountInString(p.Version) > 100 || utf8.RuneCountInString(p.Description) > 2000 {
		respond(c, http.StatusBadRequest, "invalid_product", nil)
		return p, false
	}
	return p, true
}

func productResult(c *gin.Context, status int, product data.Product, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		respond(c, http.StatusNotFound, "not_found", nil)
	case err != nil:
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
	default:
		respond(c, status, "success", product)
	}
}
