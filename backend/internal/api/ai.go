package api

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

func (s server) parseRequirement(c *gin.Context) {
	if principal(c).Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		Text string `json:"text"`
	}
	if !decode(c, &input) {
		return
	}
	text := strings.TrimSpace(input.Text)
	if text == "" || utf8.RuneCountInString(text) > 2000 {
		respond(c, http.StatusBadRequest, "invalid_requirement", nil)
		return
	}
	requirement, err := s.parser.ParseRequirement(c.Request.Context(), text)
	switch {
	case errors.Is(err, ai.ErrUnavailable):
		respond(c, http.StatusServiceUnavailable, "model_unavailable", nil)
	case err != nil:
		respond(c, http.StatusBadGateway, "model_response_error", nil)
	default:
		respond(c, http.StatusOK, "success", requirement)
	}
}
