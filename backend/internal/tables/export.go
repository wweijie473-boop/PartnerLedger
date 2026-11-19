package tables

import (
	"context"
	"database/sql"
	"encoding/csv"
	"io"
	"strings"
	"unicode"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const MaxExportRows = 10000

func (s Service) ExportCSV(ctx context.Context, actor data.User, id string, start int, count int, out io.Writer) error {
	if start < 0 || start >= MaxRows || count < 1 || count > MaxExportRows {
		return ErrInvalid
	}
	select {
	case s.exportSlots <- struct{}{}:
		defer func() { <-s.exportSlots }()
	default:
		return ErrBusy
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sheet, err := fetch(ctx, tx, actor, id, "")
	if err != nil {
		return err
	}
	var bytes int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(OCTET_LENGTH(input)),0) FROM pl_sheet_cells
		WHERE sheet_id=? AND row_no>=? AND row_no<?`, id, start, start+count).Scan(&bytes); err != nil {
		return err
	}
	if bytes > 8<<20 {
		return ErrInvalid
	}
	rows, err := tx.QueryContext(ctx, `SELECT row_no,col_no,input FROM pl_sheet_cells
		WHERE sheet_id=? AND row_no>=? AND row_no<? ORDER BY row_no,col_no`, id, start, start+count)
	if err != nil {
		return err
	}
	defer rows.Close()
	if _, err := io.WriteString(out, "\uFEFF"); err != nil {
		return err
	}
	csvOut := csv.NewWriter(out)
	record := make([]string, len(sheet.Columns))
	for i, column := range sheet.Columns {
		record[i] = safeCSV(column.Label)
	}
	if err := csvOut.Write(record); err != nil {
		return err
	}
	clear(record)
	row := start
	hasData := false
	for rows.Next() {
		var cell Cell
		if err := rows.Scan(&cell.Row, &cell.Col, &cell.Input); err != nil {
			return err
		}
		for row < cell.Row {
			if err := csvOut.Write(record); err != nil {
				return err
			}
			clear(record)
			row++
		}
		record[cell.Col], hasData = safeCSV(cell.Input), true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasData {
		if err := csvOut.Write(record); err != nil {
			return err
		}
	}
	csvOut.Flush()
	if err := csvOut.Error(); err != nil {
		return err
	}
	return tx.Commit()
}

func safeCSV(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) && !decimalPattern.MatchString(trimmed) {
		return "'" + value
	}
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") {
		return "'" + value
	}
	return value
}
