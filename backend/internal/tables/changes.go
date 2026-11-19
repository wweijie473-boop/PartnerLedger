package tables

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

func (s Service) Apply(ctx context.Context, actor data.User, id string, change Change) (uint64, error) {
	if change.Revision == 0 || !uuidPattern.MatchString(change.RequestID) ||
		len(change.Cells) > MaxChanges || len(change.Cells) == 0 && change.Settings == nil {
		return 0, ErrInvalid
	}
	if change.Settings != nil && !validSettings(change.Settings) {
		return 0, ErrInvalid
	}
	hash, err := digest(change)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	sheet, err := fetch(ctx, tx, actor, id, " FOR UPDATE")
	if err != nil {
		return 0, err
	}
	if !sheet.CanEdit || change.Settings != nil && !sheet.IsOwner {
		return 0, ErrForbidden
	}
	var original string
	var savedRevision uint64
	err = tx.QueryRowContext(ctx, `SELECT request_hash, revision FROM pl_sheet_changes
		WHERE sheet_id = ? AND actor_id = ? AND request_id = ?`, id, actor.ID, change.RequestID).Scan(&original, &savedRevision)
	if err == nil {
		if original != hash {
			return sheet.Revision, ErrIdempotency
		}
		return savedRevision, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if sheet.Revision != change.Revision {
		return sheet.Revision, ErrConflict
	}
	cfg := sheet.Settings
	if change.Settings != nil {
		cfg = *change.Settings
		if cfg.RowCount < sheet.RowCount || len(cfg.Columns) < len(sheet.Columns) {
			return 0, ErrInvalid
		}
		for i, previous := range sheet.Columns {
			if cfg.Columns[i].Kind != previous.Kind {
				return 0, ErrInvalid
			}
		}
	}
	cells, err := validatedCells(cfg, change.Cells)
	if err != nil {
		return 0, err
	}
	if err := applyCells(ctx, tx, id, cells); err != nil {
		return 0, err
	}
	revision := sheet.Revision + 1
	if change.Settings != nil {
		columns, err := encodeColumns(cfg.Columns)
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE pl_sheet_documents SET name=?, visibility=?, row_count=?,
			columns_json=?, revision=?, updated_at=UTC_TIMESTAMP(6) WHERE id=?`,
			cfg.Name, cfg.Visibility, cfg.RowCount, columns, revision, id)
		if err != nil {
			return 0, err
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE pl_sheet_documents
		SET revision=?, updated_at=UTC_TIMESTAMP(6) WHERE id=?`, revision, id); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO pl_sheet_changes
		(sheet_id, actor_id, request_id, request_hash, revision, cell_count, changed_at)
		VALUES (?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))`, id, actor.ID, change.RequestID, hash, revision, len(cells)); err != nil {
		return 0, err
	}
	return revision, tx.Commit()
}

func validatedCells(cfg Settings, edits []Edit) ([]Cell, error) {
	cells := make([]Cell, 0, len(edits))
	seen := make(map[[2]int]struct{}, len(edits))
	for i, edit := range edits {
		if edit.Row == nil || edit.Col == nil || edit.Input == nil || *edit.Row < 0 || *edit.Row >= cfg.RowCount ||
			*edit.Col < 0 || *edit.Col >= len(cfg.Columns) || !validInput(cfg.Columns[*edit.Col].Kind, *edit.Input) {
			return nil, CellError{Index: i}
		}
		location := [2]int{*edit.Row, *edit.Col}
		if _, exists := seen[location]; exists {
			return nil, CellError{Index: i}
		}
		seen[location] = struct{}{}
		cells = append(cells, Cell{Row: *edit.Row, Col: *edit.Col, Input: *edit.Input})
	}
	return cells, nil
}

func applyCells(ctx context.Context, tx *sql.Tx, id string, cells []Cell) error {
	var inserts, deletes []string
	var writeArgs []any
	deleteArgs := []any{id}
	for _, cell := range cells {
		if cell.Input == "" {
			deletes = append(deletes, "(?, ?)")
			deleteArgs = append(deleteArgs, cell.Row, cell.Col)
		} else {
			inserts = append(inserts, "(?, ?, ?, ?)")
			writeArgs = append(writeArgs, id, cell.Row, cell.Col, cell.Input)
		}
	}
	if len(deletes) > 0 {
		if _, err := tx.ExecContext(ctx, "DELETE FROM pl_sheet_cells WHERE sheet_id=? AND (row_no,col_no) IN ("+
			strings.Join(deletes, ",")+")", deleteArgs...); err != nil {
			return err
		}
	}
	if len(inserts) > 0 {
		_, err := tx.ExecContext(ctx, "INSERT INTO pl_sheet_cells (sheet_id,row_no,col_no,input) VALUES "+
			strings.Join(inserts, ",")+" AS incoming ON DUPLICATE KEY UPDATE input=incoming.input", writeArgs...)
		return err
	}
	return nil
}
