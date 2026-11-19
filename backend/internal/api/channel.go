package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
)

func (s server) channelLinks(c *gin.Context) {
	items, err := s.channel.Links(c.Request.Context(), principal(c))
	channelResult(c, http.StatusOK, gin.H{"items": items}, err)
}

func (s server) listChannelProjects(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "50"))
	after, afterErr := strconv.ParseUint(c.DefaultQuery("after", "0"), 10, 64)
	if limitErr != nil || afterErr != nil || limit < 1 || limit > 100 {
		respond(c, http.StatusBadRequest, "invalid_pagination", nil)
		return
	}
	page, err := s.channel.List(c.Request.Context(), principal(c), after, limit)
	channelResult(c, http.StatusOK, page, err)
}

func (s server) createChannelProject(c *gin.Context) {
	var input channel.CreateInput
	if !decode(c, &input) {
		return
	}
	view, err := s.channel.Create(c.Request.Context(), principal(c), input)
	channelResult(c, http.StatusCreated, view, err)
}

func (s server) getChannelProject(c *gin.Context) {
	id, ok := channelProjectID(c)
	if !ok {
		return
	}
	view, err := s.channel.Get(c.Request.Context(), principal(c), id)
	channelResult(c, http.StatusOK, view, err)
}

func (s server) actOnChannelProject(c *gin.Context) {
	id, ok := channelProjectID(c)
	if !ok {
		return
	}
	var input channel.ActionInput
	if !decode(c, &input) {
		return
	}
	view, err := s.channel.Act(c.Request.Context(), principal(c), id, c.Param("action"), input)
	channelResult(c, http.StatusOK, view, err)
}

func channelProjectID(c *gin.Context) (uint64, bool) {
	value := c.Param("id")
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 || value != strconv.FormatUint(id, 10) {
		respond(c, http.StatusNotFound, "not_found", nil)
		return 0, false
	}
	return id, true
}

func channelResult(c *gin.Context, status int, result any, err error) {
	switch {
	case err == nil:
		respond(c, status, "success", result)
	case errors.Is(err, channel.ErrInvalid), errors.Is(err, channel.ErrLinkRequired):
		respond(c, http.StatusBadRequest, err.Error(), nil)
	case errors.Is(err, channel.ErrForbidden):
		respond(c, http.StatusForbidden, "forbidden", nil)
	case errors.Is(err, channel.ErrNotFound):
		respond(c, http.StatusNotFound, "not_found", nil)
	case errors.Is(err, channel.ErrTransition), errors.Is(err, channel.ErrRevision), errors.Is(err, channel.ErrIdempotency):
		respond(c, http.StatusConflict, err.Error(), nil)
	default:
		respond(c, http.StatusServiceUnavailable, "dependencies_unavailable", nil)
	}
}
