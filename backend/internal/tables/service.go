package tables

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

const (
	MaxRows        = 100000
	MaxColumns     = 128
	MaxWindowCells = 2048
	MaxChanges     = 250
	MaxInputBytes  = 1000
	pageSize       = 50
)

var (
	ErrInvalid     = errors.New("invalid_sheet_request")
	ErrForbidden   = errors.New("sheet_write_forbidden")
	ErrConflict    = errors.New("sheet_revision_conflict")
	ErrIdempotency = errors.New("sheet_request_conflict")
	ErrNotFound    = errors.New("sheet_not_found")
	ErrBusy        = errors.New("sheet_export_busy")
	uuidPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	decimalPattern = regexp.MustCompile(`^-?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)$`)
)

type CellError struct{ Index int }

func (e CellError) Error() string { return "invalid_sheet_cell" }

type Column struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type Settings struct {
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
	RowCount   int      `json:"rowCount"`
	Columns    []Column `json:"columns"`
}

type Sheet struct {
	ID string `json:"id"`
	Settings
	Revision  uint64    `json:"revision"`
	CanEdit   bool      `json:"canEdit"`
	IsOwner   bool      `json:"isOwner"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Page struct {
	Items      []Sheet `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

type Cell struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Input string `json:"input"`
}

type Edit struct {
	Row   *int    `json:"row"`
	Col   *int    `json:"col"`
	Input *string `json:"input"`
}

type Window struct {
	Revision uint64 `json:"revision"`
	Cells    []Cell `json:"cells"`
}

type Area struct{ Row, Col, Height, Width int }

type Change struct {
	RequestID string    `json:"requestId"`
	Revision  uint64    `json:"revision"`
	Cells     []Edit    `json:"cells"`
	Settings  *Settings `json:"settings,omitempty"`
}

type Service struct {
	db          *sql.DB
	exportSlots chan struct{}
}

func New(db *sql.DB) Service { return Service{db: db, exportSlots: make(chan struct{}, 2)} }

const fields = "id, owner_id, name, visibility, row_count, columns_json, revision, updated_at"
const access = "organization_id = ? AND (owner_id = ? OR visibility <> 'private')"

func validSettings(cfg *Settings) bool {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" || utf8.RuneCountInString(cfg.Name) > 120 || cfg.RowCount < 1 || cfg.RowCount > MaxRows ||
		len(cfg.Columns) < 1 || len(cfg.Columns) > MaxColumns {
		return false
	}
	if cfg.Visibility != "private" && cfg.Visibility != "org_read" && cfg.Visibility != "org_edit" {
		return false
	}
	for i := range cfg.Columns {
		col := &cfg.Columns[i]
		col.Label = strings.TrimSpace(col.Label)
		if col.Label == "" || utf8.RuneCountInString(col.Label) > 80 {
			return false
		}
		switch col.Kind {
		case "auto", "text", "number", "date", "checkbox":
		default:
			return false
		}
	}
	return true
}

func validInput(kind, input string) bool {
	if len(input) > MaxInputBytes {
		return false
	}
	if input == "" || kind == "text" || kind == "auto" {
		return true
	}
	switch kind {
	case "number":
		return decimalPattern.MatchString(input) || strings.HasPrefix(input, "=")
	case "date":
		day, err := time.Parse(time.DateOnly, input)
		return err == nil && day.Year() > 0 && day.Format(time.DateOnly) == input
	case "checkbox":
		return input == "true" || input == "false"
	}
	return false
}

func scanSheet(row interface{ Scan(...any) error }, actor data.User) (Sheet, error) {
	var sheet Sheet
	var owner uint64
	var columns []byte
	err := row.Scan(&sheet.ID, &owner, &sheet.Name, &sheet.Visibility, &sheet.RowCount, &columns, &sheet.Revision, &sheet.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Sheet{}, ErrNotFound
	}
	if err != nil {
		return Sheet{}, err
	}
	sheet.IsOwner = owner == actor.ID
	sheet.CanEdit = sheet.IsOwner || sheet.Visibility == "org_edit"
	err = json.Unmarshal(columns, &sheet.Columns)
	return sheet, err
}

func fetch(ctx context.Context, tx interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, actor data.User, id string, lock string) (Sheet, error) {
	if !uuidPattern.MatchString(id) {
		return Sheet{}, ErrNotFound
	}
	return scanSheet(tx.QueryRowContext(ctx, "SELECT "+fields+" FROM pl_sheet_documents WHERE id = ? AND "+access+lock,
		id, actor.OrganizationID, actor.ID), actor)
}

func (s Service) Get(ctx context.Context, actor data.User, id string) (Sheet, error) {
	return fetch(ctx, s.db, actor, id, "")
}

func (s Service) List(ctx context.Context, actor data.User, after string) (Page, error) {
	page := Page{Items: []Sheet{}}
	if after != "" && !uuidPattern.MatchString(after) {
		return page, ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+fields+" FROM pl_sheet_documents WHERE "+access+
		" AND id > ? ORDER BY id LIMIT ?", actor.OrganizationID, actor.ID, after, pageSize+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		if len(page.Items) == pageSize {
			page.NextCursor = &page.Items[pageSize-1].ID
			break
		}
		sheet, err := scanSheet(rows, actor)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, sheet)
	}
	return page, rows.Err()
}

func (s Service) Create(ctx context.Context, actor data.User, requestID string, cfg Settings) (Sheet, error) {
	if !uuidPattern.MatchString(requestID) || !validSettings(&cfg) {
		return Sheet{}, ErrInvalid
	}
	columns, err := encodeColumns(cfg.Columns)
	if err != nil {
		return Sheet{}, err
	}
	hash, err := digest(cfg)
	if err != nil {
		return Sheet{}, err
	}
	id, err := newID()
	if err != nil {
		return Sheet{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pl_sheet_documents
		(id, organization_id, owner_id, name, visibility, row_count, columns_json, revision, create_request, create_hash, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?, UTC_TIMESTAMP(6))`,
		id, actor.OrganizationID, actor.ID, cfg.Name, cfg.Visibility, cfg.RowCount, columns, requestID, hash)
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		var old string
		err = s.db.QueryRowContext(ctx, "SELECT id, create_hash FROM pl_sheet_documents WHERE organization_id=? AND owner_id=? AND create_request=?",
			actor.OrganizationID, actor.ID, requestID).Scan(&id, &old)
		if errors.Is(err, sql.ErrNoRows) || err == nil && old != hash {
			return Sheet{}, ErrIdempotency
		}
	}
	if err != nil {
		return Sheet{}, err
	}
	return s.Get(ctx, actor, id)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func encodeColumns(columns []Column) ([]byte, error) {
	return json.Marshal(columns)
}

func (s Service) ReadWindow(ctx context.Context, actor data.User, id string, area Area) (Window, error) {
	w := Window{Cells: []Cell{}}
	if area.Row < 0 || area.Col < 0 || area.Row >= MaxRows || area.Col >= MaxColumns ||
		area.Height < 1 || area.Width < 1 || area.Height > MaxWindowCells || area.Width > MaxColumns ||
		area.Height*area.Width > MaxWindowCells {
		return w, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return w, err
	}
	defer tx.Rollback()
	sheet, err := fetch(ctx, tx, actor, id, "")
	if err != nil {
		return w, err
	}
	w.Revision = sheet.Revision
	rows, err := tx.QueryContext(ctx, `SELECT row_no, col_no, input FROM pl_sheet_cells
		WHERE sheet_id = ? AND row_no >= ? AND row_no < ? AND col_no >= ? AND col_no < ?
		ORDER BY row_no, col_no`, id, area.Row, area.Row+area.Height, area.Col, area.Col+area.Width)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var cell Cell
		if err := rows.Scan(&cell.Row, &cell.Col, &cell.Input); err != nil {
			return w, err
		}
		w.Cells = append(w.Cells, cell)
	}
	if err := rows.Err(); err != nil {
		return w, err
	}
	return w, tx.Commit()
}

func digest(value any) (string, error) {
	b, err := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), err
}
