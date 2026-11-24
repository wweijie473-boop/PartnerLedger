package tables

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const (
	maxSearchText   = 160
	maxSearchScan   = 2048
	maxSearchResult = 60
)

type SearchInput struct {
	Text            string  `json:"text"`
	MatchCase       bool    `json:"matchCase"`
	WholeCell       bool    `json:"wholeCell"`
	IncludeFormulas bool    `json:"includeFormulas"`
	Replacement     *string `json:"replacement,omitempty"`
	After           string  `json:"after,omitempty"`
	Revision        uint64  `json:"revision,omitempty"`
}

type Match struct {
	Cell
	Proposed *string `json:"proposed,omitempty"`
	Invalid  bool    `json:"invalid,omitempty"`
}

type SearchPage struct {
	Revision   uint64  `json:"revision"`
	Matches    []Match `json:"matches"`
	Scanned    int     `json:"scanned"`
	NextCursor *string `json:"nextCursor"`
}

func (s Service) Search(ctx context.Context, actor data.User, id string, query SearchInput) (SearchPage, error) {
	page := SearchPage{Matches: []Match{}}
	if query.Text == "" || utf8.RuneCountInString(query.Text) > maxSearchText ||
		query.Replacement != nil && utf8.RuneCountInString(*query.Replacement) > MaxInputBytes {
		return page, ErrInvalid
	}
	args := []any{id}
	where := "sheet_id=?"
	if query.After != "" {
		row, col, ok := searchCursor(query.After)
		if !ok || query.Revision == 0 {
			return page, ErrInvalid
		}
		where += " AND (row_no>? OR (row_no=? AND col_no>?))"
		args = append(args, row, row, col)
	}
	args = append(args, maxSearchScan+1)

	select {
	case s.searchSlots <- struct{}{}:
		defer func() { <-s.searchSlots }()
	default:
		return page, ErrSearchBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	sheet, err := fetch(ctx, tx, actor, id, "")
	if err != nil {
		return page, err
	}
	page.Revision = sheet.Revision
	if query.Replacement != nil && !sheet.CanEdit {
		return page, ErrForbidden
	}
	if query.Revision != 0 && sheet.Revision != query.Revision {
		return page, ErrConflict
	}
	matcher, err := compileSearch(query)
	if err != nil {
		return page, ErrInvalid
	}
	rows, err := tx.QueryContext(ctx, "SELECT row_no,col_no,input FROM pl_sheet_cells WHERE "+where+
		" ORDER BY row_no,col_no LIMIT ?", args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var last Cell
	for rows.Next() {
		if page.Scanned == maxSearchScan || len(page.Matches) == maxSearchResult {
			cursor := strconv.Itoa(last.Row) + ":" + strconv.Itoa(last.Col)
			page.NextCursor = &cursor
			break
		}
		if err := rows.Scan(&last.Row, &last.Col, &last.Input); err != nil {
			return page, err
		}
		page.Scanned++
		if last.Col >= len(sheet.Columns) {
			continue
		}
		kind := sheet.Columns[last.Col].Kind
		if !query.IncludeFormulas && (kind == "auto" || kind == "number") && strings.HasPrefix(last.Input, "=") {
			continue
		}
		if result, ok := searchCell(matcher, last, kind, query.Replacement); ok {
			page.Matches = append(page.Matches, result)
		}
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if err := rows.Close(); err != nil {
		return page, err
	}
	return page, tx.Commit()
}

func searchCursor(value string) (int, int, bool) {
	r, c, ok := strings.Cut(value, ":")
	row, rowErr := strconv.Atoi(r)
	col, colErr := strconv.Atoi(c)
	return row, col, ok && rowErr == nil && colErr == nil && row >= 0 && row < MaxRows && col >= 0 && col < MaxColumns
}

func compileSearch(query SearchInput) (*regexp.Regexp, error) {
	pattern := regexp.QuoteMeta(query.Text)
	if query.WholeCell {
		pattern = `\A` + pattern + `\z`
	}
	if !query.MatchCase {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func searchCell(matcher *regexp.Regexp, cell Cell, kind string, replacement *string) (Match, bool) {
	match := Match{Cell: cell}
	if replacement == nil {
		return match, matcher.MatchString(cell.Input)
	}
	ranges := matcher.FindAllStringIndex(cell.Input, -1)
	if len(ranges) == 0 {
		return match, false
	}
	size := len(cell.Input)
	for _, span := range ranges {
		size += len(*replacement) - (span[1] - span[0])
	}
	if size > MaxInputBytes {
		match.Invalid = true
		return match, true
	}
	var proposed strings.Builder
	proposed.Grow(size)
	offset := 0
	for _, span := range ranges {
		proposed.WriteString(cell.Input[offset:span[0]])
		proposed.WriteString(*replacement)
		offset = span[1]
	}
	proposed.WriteString(cell.Input[offset:])
	value := proposed.String()
	if !validInput(kind, value) {
		match.Invalid = true
		return match, true
	}
	match.Proposed = &value
	return match, true
}
