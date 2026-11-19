package api

import (
	"errors"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/auth"
	"gorm.io/gorm"
)

const registrationWindow = 600
const registrationAttempts = 10

var registrationThrottle = redis.NewScript(`
local attempts = redis.call("INCR", KEYS[1])
if attempts == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return attempts
`)

func (s server) register(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var input struct {
		Username         string `json:"username"`
		Password         string `json:"password"`
		OrganizationName string `json:"organization_name"`
		OrganizationKind string `json:"organization_kind"`
	}
	if !decode(c, &input) {
		return
	}

	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	ctx := c.Request.Context()
	key := "partnerledger:registration:" + ip
	attempts, err := registrationThrottle.Run(ctx, s.cache, []string{key}, registrationWindow).Int()
	if err != nil {
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
		return
	}
	if attempts > registrationAttempts {
		c.Header("Retry-After", strconv.Itoa(registrationWindow))
		respond(c, http.StatusTooManyRequests, "registration_rate_limited", nil)
		return
	}

	organizationID, err := auth.CreateOrganization(ctx, s.db, input.OrganizationName, input.OrganizationKind, input.Username, input.Password)
	switch {
	case errors.Is(err, auth.ErrRegistration):
		respond(c, http.StatusBadRequest, "invalid_registration", nil)
	case errors.Is(err, gorm.ErrDuplicatedKey):
		respond(c, http.StatusConflict, "username_unavailable", nil)
	case err != nil:
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
	default:
		respond(c, http.StatusCreated, "success", gin.H{"organization_id": strconv.FormatUint(organizationID, 10)})
	}
}
