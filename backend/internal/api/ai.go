package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

func (s server) parseRequirement(c *gin.Context) {
	text, ok := readAIWorkspaceText(c)
	if !ok {
		return
	}
	requirement, err := s.parser.ParseRequirement(c.Request.Context(), text)
	aiWorkspaceResult(c, requirement, err)
}

func (s server) routeWorkspace(c *gin.Context) {
	text, ok := readAIWorkspaceText(c)
	if !ok {
		return
	}
	router, ok := s.parser.(interface {
		RouteWorkspace(context.Context, string) (ai.WorkspaceRoute, error)
	})
	if !ok {
		aiWorkspaceResult(c, nil, ai.ErrUnavailable)
		return
	}
	route, err := router.RouteWorkspace(c.Request.Context(), text)
	aiWorkspaceResult(c, route, err)
}

func readAIWorkspaceText(c *gin.Context) (string, bool) {
	if principal(c).Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return "", false
	}
	var input struct {
		Text string `json:"text"`
	}
	if !decode(c, &input) {
		return "", false
	}
	text := strings.TrimSpace(input.Text)
	if text == "" || utf8.RuneCountInString(text) > 2000 {
		respond(c, http.StatusBadRequest, "invalid_requirement", nil)
		return "", false
	}
	return text, true
}

func aiWorkspaceResult(c *gin.Context, result any, err error) {
	switch {
	case errors.Is(err, ai.ErrUnavailable):
		respond(c, http.StatusServiceUnavailable, "model_unavailable", nil)
	case err != nil:
		respond(c, http.StatusBadGateway, "model_response_error", nil)
	default:
		respond(c, http.StatusOK, "success", result)
	}
}
