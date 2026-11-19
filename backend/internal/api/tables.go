package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/tables"
)

func (s server) registerSheets(group *gin.RouterGroup) {
	group.GET("", func(c *gin.Context) {
		page, err := s.tables.List(c.Request.Context(), principal(c), c.Query("after"))
		sheetResult(c, http.StatusOK, page, err)
	})
	group.POST("", func(c *gin.Context) {
		var input struct {
			RequestID string `json:"requestId"`
			tables.Settings
		}
		if !decodeSheet(c, &input) {
			return
		}
		sheet, err := s.tables.Create(c.Request.Context(), principal(c), input.RequestID, input.Settings)
		sheetResult(c, http.StatusCreated, sheet, err)
	})
	group.GET("/:id", func(c *gin.Context) {
		sheet, err := s.tables.Get(c.Request.Context(), principal(c), c.Param("id"))
		sheetResult(c, http.StatusOK, sheet, err)
	})
	group.GET("/:id/window", s.readSheetWindow)
	group.POST("/:id/changes", func(c *gin.Context) {
		var change tables.Change
		if !decodeSheet(c, &change) {
			return
		}
		revision, err := s.tables.Apply(c.Request.Context(), principal(c), c.Param("id"), change)
		sheetResult(c, http.StatusOK, gin.H{"revision": revision}, err)
	})
	group.GET("/:id/csv", s.exportSheet)
}

func (s server) readSheetWindow(c *gin.Context) {
	row, rowErr := strconv.Atoi(c.Query("row"))
	col, colErr := strconv.Atoi(c.Query("col"))
	height, heightErr := strconv.Atoi(c.Query("height"))
	width, widthErr := strconv.Atoi(c.Query("width"))
	if rowErr != nil || colErr != nil || heightErr != nil || widthErr != nil {
		sheetResult(c, 0, nil, tables.ErrInvalid)
		return
	}
	result, err := s.tables.ReadWindow(c.Request.Context(), principal(c), c.Param("id"),
		tables.Area{Row: row, Col: col, Height: height, Width: width})
	sheetResult(c, http.StatusOK, result, err)
}

func (s server) exportSheet(c *gin.Context) {
	start, startErr := strconv.Atoi(c.DefaultQuery("start", "0"))
	count, countErr := strconv.Atoi(c.DefaultQuery("count", "10000"))
	if startErr != nil || countErr != nil {
		sheetResult(c, 0, nil, tables.ErrInvalid)
		return
	}
	var buffer bytes.Buffer
	if err := s.tables.ExportCSV(c.Request.Context(), principal(c), c.Param("id"), start, count, &buffer); err != nil {
		sheetResult(c, 0, nil, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=sheet.csv")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
}

func decodeSheet(c *gin.Context, input any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 384<<10))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(input)
	if err == nil {
		err = decoder.Decode(new(any))
		if err == io.EOF {
			return true
		}
	}
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		respond(c, http.StatusRequestEntityTooLarge, "sheet_request_too_large", nil)
	} else {
		sheetResult(c, 0, nil, tables.ErrInvalid)
	}
	return false
}

func sheetResult(c *gin.Context, status int, value any, err error) {
	var cell tables.CellError
	switch {
	case err == nil:
		respond(c, status, "success", value)
	case errors.Is(err, tables.ErrInvalid):
		respond(c, http.StatusBadRequest, err.Error(), nil)
	case errors.As(err, &cell):
		respond(c, http.StatusUnprocessableEntity, cell.Error(), gin.H{"index": cell.Index})
	case errors.Is(err, tables.ErrNotFound):
		respond(c, http.StatusNotFound, err.Error(), nil)
	case errors.Is(err, tables.ErrForbidden):
		respond(c, http.StatusForbidden, err.Error(), nil)
	case errors.Is(err, tables.ErrBusy):
		c.Header("Retry-After", "2")
		respond(c, http.StatusTooManyRequests, err.Error(), nil)
	case errors.Is(err, tables.ErrConflict), errors.Is(err, tables.ErrIdempotency):
		respond(c, http.StatusConflict, err.Error(), value)
	default:
		respond(c, http.StatusServiceUnavailable, "sheet_storage_unavailable", nil)
	}
}
