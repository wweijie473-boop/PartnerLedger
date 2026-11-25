package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/ai"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const maxRawAudioBytes = 3 << 20

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

func (s server) transcribeAudio(c *gin.Context) {
	if principal(c).Organization.Kind != data.Reseller {
		respond(c, http.StatusForbidden, "forbidden", nil)
		return
	}
	var input struct {
		Audio  string `json:"audio"`
		Format string `json:"format"`
	}
	if !decodeWithLimit(c, &input, 6<<20) {
		return
	}
	format := strings.ToLower(strings.TrimSpace(input.Format))
	cleaned := strings.TrimSpace(input.Audio)
	if format != "wav" || cleaned == "" {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil || len(raw) < 44 || len(raw) > maxRawAudioBytes ||
		!bytes.Equal(raw[:4], []byte("RIFF")) || !bytes.Equal(raw[8:12], []byte("WAVE")) {
		respond(c, http.StatusBadRequest, "invalid_request", nil)
		return
	}
	transcriber, ok := s.parser.(interface {
		TranscribeAudio(context.Context, string, string) (string, error)
	})
	if !ok {
		aiWorkspaceResult(c, nil, ai.ErrUnavailable)
		return
	}
	text, err := transcriber.TranscribeAudio(c.Request.Context(), cleaned, format)
	aiWorkspaceResult(c, gin.H{"text": text}, err)
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
