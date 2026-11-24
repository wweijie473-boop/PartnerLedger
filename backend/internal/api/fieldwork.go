package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/channel"
)

func (s server) opportunityDesk(c *gin.Context) {
	record, err := s.channel.Desk(c.Request.Context(), principal(c))
	channelResult(c, http.StatusOK, record, err)
}

func (s server) previewOpportunity(c *gin.Context) {
	var capability channel.Capability
	if !decodeWithLimit(c, &capability, 2048) {
		return
	}
	result, err := s.channel.RecommendPartners(c.Request.Context(), principal(c), 0, capability)
	channelResult(c, http.StatusOK, result, err)
}

func (s server) opportunityRecommendation(c *gin.Context) {
	id, ok := channelResourceID(c)
	if !ok {
		return
	}
	result, err := s.channel.RecommendPartners(c.Request.Context(), principal(c), id, channel.Capability{})
	channelResult(c, http.StatusOK, result, err)
}

func (s server) captureOpportunity(c *gin.Context) {
	var input channel.CaptureInput
	if !decode(c, &input) {
		return
	}
	record, err := s.channel.CaptureOpportunity(c.Request.Context(), principal(c), input)
	channelResult(c, http.StatusCreated, record, err)
}

func (s server) addOpportunityPartner(c *gin.Context) {
	var input channel.AddPartnerInput
	if !decode(c, &input) {
		return
	}
	record, err := s.channel.AddPartner(c.Request.Context(), principal(c), input)
	channelResult(c, http.StatusCreated, record, err)
}

func (s server) actOnOpportunity(c *gin.Context) {
	id, ok := channelResourceID(c)
	if !ok {
		return
	}
	var input channel.ActionInput
	if !decode(c, &input) {
		return
	}
	record, err := s.channel.ActOnOpportunity(c.Request.Context(), principal(c), id, c.Param("action"), input)
	channelResult(c, http.StatusOK, record, err)
}

func (s server) listVisits(c *gin.Context) {
	items, err := s.channel.Visits(c.Request.Context(), principal(c))
	channelResult(c, http.StatusOK, gin.H{"items": items}, err)
}

func (s server) getVisit(c *gin.Context) {
	id, ok := channelResourceID(c)
	if !ok {
		return
	}
	record, err := s.channel.Visit(c.Request.Context(), principal(c), id)
	channelResult(c, http.StatusOK, record, err)
}

func (s server) saveVisit(c *gin.Context) {
	var id uint64
	status := http.StatusCreated
	if c.Request.Method == http.MethodPut {
		var ok bool
		id, ok = channelResourceID(c)
		if !ok {
			return
		}
		status = http.StatusOK
	}
	var input channel.SaveVisitInput
	if !decode(c, &input) {
		return
	}
	record, err := s.channel.SaveVisit(c.Request.Context(), principal(c), id, input)
	channelResult(c, status, record, err)
}

func (s server) confirmVisit(c *gin.Context) {
	id, ok := channelResourceID(c)
	if !ok {
		return
	}
	var input channel.ConfirmVisitInput
	if !decode(c, &input) {
		return
	}
	record, err := s.channel.ConfirmVisit(c.Request.Context(), principal(c), id, input)
	channelResult(c, http.StatusOK, record, err)
}

func (s server) deleteVisit(c *gin.Context) {
	id, ok := channelResourceID(c)
	if !ok {
		return
	}
	var input channel.VisitRevisionInput
	if !decode(c, &input) {
		return
	}
	record, err := s.channel.DeleteVisit(c.Request.Context(), principal(c), id, input)
	channelResult(c, http.StatusOK, record, err)
}
