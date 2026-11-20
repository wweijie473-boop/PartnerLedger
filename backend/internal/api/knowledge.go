package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/catalog"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s server) getKnowledge(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	profile := catalog.ProductKnowledgeProfile{Knowledge: catalog.EmptyKnowledge()}
	actor := principal(c)
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var product data.Product
		if err := catalog.Visible(tx, actor).Select("id").First(&product, id).Error; err != nil {
			return err
		}
		err := tx.Where("product_id = ?", id).Take(&profile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	knowledgeResult(c, profile.Knowledge, err)
}

func (s server) putKnowledge(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	var input catalog.Knowledge
	if !decodeWithLimit(c, &input, 128<<10) {
		return
	}
	if err := input.Normalize(); err != nil {
		knowledgeResult(c, nil, err)
		return
	}
	actor := principal(c)
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var product data.Product
		if err := catalog.Visible(tx, actor).Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&product, id).Error; err != nil {
			return err
		}
		profile := catalog.ProductKnowledgeProfile{ProductID: id, Knowledge: input, UpdatedBy: actor.ID}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&profile).Error
	})
	knowledgeResult(c, input, err)
}

func (s server) matchProducts(c *gin.Context) {
	actor := principal(c)
	if actor.Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input catalog.Requirements
	if !decodeWithLimit(c, &input, 40<<10) {
		return
	}
	if err := input.Normalize(); err != nil {
		knowledgeResult(c, nil, err)
		return
	}
	matches, err := catalog.MatchPublished(s.db.WithContext(c.Request.Context()), actor, input)
	knowledgeResult(c, gin.H{"items": matches}, err)
}

func knowledgeResult(c *gin.Context, result any, err error) {
	switch {
	case err == nil:
		respond(c, http.StatusOK, "success", result)
	case errors.Is(err, gorm.ErrRecordNotFound):
		respond(c, http.StatusNotFound, "not_found", nil)
	case errors.Is(err, catalog.ErrInvalidKnowledge), errors.Is(err, catalog.ErrInvalidRequirements):
		respond(c, http.StatusBadRequest, err.Error(), nil)
	default:
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
	}
}
