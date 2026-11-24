package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/tables"
	"gorm.io/gorm"
)

type RequirementParser interface {
	ParseRequirement(context.Context, string) (ai.Requirement, error)
}

type server struct {
	db      *gorm.DB
	sql     *sql.DB
	cache   *redis.Client
	auth    *auth.Service
	parser  RequirementParser
	channel channel.Service
	tables  tables.Service
}

func New(db *gorm.DB, sqlDB *sql.DB, cache *redis.Client, parser RequirementParser) http.Handler {
	s := server{db: db, sql: sqlDB, cache: cache, auth: auth.New(db, cache), parser: parser, channel: channel.New(db), tables: tables.New(sqlDB)}
	r := gin.New()
	r.Use(gin.Recovery())
	r.NoRoute(func(c *gin.Context) { respond(c, http.StatusNotFound, "not_found", nil) })
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) { respond(c, http.StatusMethodNotAllowed, "method_not_allowed", nil) })
	r.GET("/healthz", func(c *gin.Context) { respond(c, http.StatusOK, "success", gin.H{"status": "alive"}) })
	r.GET("/readyz", s.ready)

	api := r.Group("/api")
	api.POST("/auth/login", s.login)
	api.POST("/auth/register", s.register)
	secured := api.Group("", s.requireAuth)
	s.registerRetail(api, secured)
	secured.POST("/auth/logout", s.logout)
	secured.GET("/user/profile", func(c *gin.Context) { respond(c, http.StatusOK, "success", principal(c)) })
	secured.GET("/org/current", func(c *gin.Context) { respond(c, http.StatusOK, "success", principal(c).Organization) })
	secured.GET("/search/v1/records", s.searchSiteRecords)
	secured.GET("/products", s.listProducts)
	secured.GET("/products/:id", s.getProduct)
	secured.POST("/products", s.requireManufacturer, s.createProduct)
	secured.PUT("/products/:id", s.requireManufacturer, s.updateProduct)
	secured.GET("/products/:id/knowledge", s.getKnowledge)
	secured.PUT("/products/:id/knowledge", s.requireManufacturer, s.putKnowledge)
	secured.POST("/ai/parse-requirement", s.parseRequirement)
	secured.POST("/ai/workspace/route", s.routeWorkspace)
	secured.POST("/ai/match-products", s.matchProducts)
	projects := secured.Group("/channel/v1")
	projects.GET("/organization-links", s.channelLinks)
	projects.GET("/projects", s.listChannelProjects)
	projects.GET("/projects/workspace", s.projectWorkspace)
	projects.POST("/projects", s.createChannelProject)
	projects.GET("/projects/:id", s.getChannelProject)
	projects.GET("/projects/:id/opportunities", s.listProjectOpportunities)
	projects.POST("/projects/:id/actions/:action", s.actOnChannelProject)
	projects.GET("/projects/:id/inquiry-draft", s.latestInquiryDraft)
	projects.POST("/projects/:id/inquiry-draft", s.createInquiryDraft)
	projects.GET("/opportunities/desk", s.opportunityDesk)
	projects.POST("/opportunities/preview", s.previewOpportunity)
	projects.GET("/opportunities/:id/recommendation", s.opportunityRecommendation)
	projects.POST("/opportunities", s.captureOpportunity)
	projects.POST("/opportunities/partners", s.addOpportunityPartner)
	projects.POST("/opportunities/:id/actions/:action", s.actOnOpportunity)
	projects.GET("/visits", s.listVisits)
	projects.POST("/visits", s.saveVisit)
	projects.GET("/visits/:id", s.getVisit)
	projects.PUT("/visits/:id", s.saveVisit)
	projects.DELETE("/visits/:id", s.deleteVisit)
	projects.POST("/visits/:id/confirm", s.confirmVisit)
	s.registerSheets(secured.Group("/tables/v1/sheets"))
	return r
}

func (s server) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if !data.SchemaReady(ctx, s.sql) || !tables.SchemaReady(ctx, s.sql) || s.cache.Ping(ctx).Err() != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	respond(c, http.StatusOK, "success", gin.H{"status": "ready"})
}

func (s server) login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(c, &input) {
		return
	}
	user, token, err := s.auth.Login(c.Request.Context(), input.Username, input.Password)
	switch {
	case errors.Is(err, auth.ErrCredentials):
		respond(c, http.StatusUnauthorized, "invalid_credentials", nil)
	case err != nil:
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
	default:
		c.Header("Cache-Control", "no-store")
		respond(c, http.StatusOK, "success", gin.H{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   int(auth.SessionTTL.Seconds()),
			"user":         user,
		})
	}
}

func (s server) requireAuth(c *gin.Context) {
	scheme, token, _ := strings.Cut(c.GetHeader("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") || len(token) != 43 {
		respond(c, http.StatusUnauthorized, "authentication_required", nil)
		c.Abort()
		return
	}
	user, err := s.auth.Authenticate(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrSession) {
			respond(c, http.StatusUnauthorized, "authentication_required", nil)
		} else {
			respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		}
		c.Abort()
		return
	}
	c.Set("principal", user)
	c.Set("access_token", token)
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func (s server) logout(c *gin.Context) {
	if err := s.auth.Logout(c.Request.Context(), c.GetString("access_token")); err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	respond(c, http.StatusOK, "success", nil)
}

func (s server) requireManufacturer(c *gin.Context) {
	if principal(c).Organization.Kind != data.Manufacturer {
		respond(c, http.StatusForbidden, "forbidden", nil)
		c.Abort()
		return
	}
	c.Next()
}

func principal(c *gin.Context) data.User {
	return c.MustGet("principal").(data.User)
}

func decode(c *gin.Context, target any) bool {
	return decodeWithLimit(c, target, 16<<10)
}

func decodeWithLimit(c *gin.Context, target any, limit int64) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, limit))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return false
	}
	return true
}

func respond(c *gin.Context, status int, message string, data any) {
	c.JSON(status, gin.H{"code": status, "message": message, "data": data})
}
