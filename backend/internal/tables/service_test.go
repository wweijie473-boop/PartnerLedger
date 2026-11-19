package tables

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

func TestSheetStorage(t *testing.T) {
	if os.Getenv("PARTNERLEDGER_TABLES_INTEGRATION_TEST") != "1" {
		t.Skip("requires the migrated development database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := data.OpenSQL(os.Getenv("PARTNERLEDGER_MYSQL_DSN"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := New(db)
	org := uint64(time.Now().UnixNano())
	owner := data.User{ID: 91, OrganizationID: org}
	member := data.User{ID: 92, OrganizationID: org}
	outsider := data.User{ID: 91, OrganizationID: org + 1}
	columns := make([]Column, MaxColumns)
	for i := range columns {
		columns[i] = Column{Label: "内容", Kind: "auto"}
	}
	columns[1].Kind = "number"
	settings := Settings{Name: "集成测试", Visibility: "private", RowCount: MaxRows, Columns: columns}
	id, _ := newID()
	sheet, err := svc.Create(ctx, owner, id, settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DELETE FROM pl_sheet_documents WHERE id=? AND organization_id=?", sheet.ID, org); err != nil {
			t.Error(err)
		}
	})
	again, err := svc.Create(ctx, owner, id, settings)
	if err != nil || again.ID != sheet.ID {
		t.Fatalf("create replay: %s %v", again.ID, err)
	}
	if _, err := svc.Get(ctx, member, sheet.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("private sheet leaked: %v", err)
	}
	for _, actor := range []data.User{member, outsider} {
		result, err := svc.List(ctx, actor, "")
		if err != nil || len(result.Items) != 0 {
			t.Fatalf("private sheet listed: %v %+v", err, result)
		}
	}
	set := func(r, c int, input string) Edit { return Edit{Row: &r, Col: &c, Input: &input} }
	change := func(rev uint64, cells ...Edit) Change {
		id, _ := newID()
		return Change{RequestID: id, Revision: rev, Cells: cells}
	}
	wrong := change(1, set(0, 0, "must be rolled back"), set(0, 1, "not a number"))
	if _, err := svc.Apply(ctx, owner, sheet.ID, wrong); err != (CellError{Index: 1}) {
		t.Fatalf("invalid input: %v", err)
	}
	window, err := svc.ReadWindow(ctx, owner, sheet.ID, Area{Height: 1, Width: 2})
	if err != nil || window.Revision != 1 || len(window.Cells) != 0 {
		t.Fatalf("invalid batch left changes: %+v %v", window, err)
	}
	settings.Visibility = "org_read"
	meta := change(1)
	meta.Settings = &settings
	if _, err := svc.Apply(ctx, owner, sheet.ID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReadWindow(ctx, outsider, sheet.ID, Area{Height: 1, Width: 1}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-organization read: %v", err)
	}
	if err := svc.ExportCSV(ctx, outsider, sheet.ID, 0, 10, &bytes.Buffer{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-organization export: %v", err)
	}
	if _, err := svc.Apply(ctx, member, sheet.ID, change(2, set(0, 0, "denied"))); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unexpected write permission: %v", err)
	}
	settings.Visibility = "org_edit"
	meta = change(2)
	meta.Settings = &settings
	if _, err := svc.Apply(ctx, owner, sheet.ID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(ctx, member, sheet.ID, meta); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member may change sharing: %v", err)
	}
	edits := []Edit{set(0, 0, "=UNSUPPORTED()")}
	for i := 1; i < MaxChanges; i++ {
		edits = append(edits, set(MaxRows-i, MaxColumns-1, "测试"))
	}
	batch := change(3, edits...)
	results := make(chan error, 2)
	for range 2 {
		go func() {
			rev, err := svc.Apply(ctx, member, sheet.ID, batch)
			if err == nil && rev != 4 {
				err = errors.New("unexpected revision")
			}
			results <- err
		}()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Apply(ctx, owner, sheet.ID, change(3, set(0, 0, "overwrite"))); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale write accepted: %v", err)
	}
	window, err = svc.ReadWindow(ctx, owner, sheet.ID, Area{Row: MaxRows - 1, Col: MaxColumns - 1, Height: 1, Width: 1})
	if err != nil || window.Revision != 4 || len(window.Cells) != 1 || window.Cells[0].Input != "测试" {
		t.Fatalf("last row could not be read: %+v %v", window, err)
	}
	var csv bytes.Buffer
	if err := svc.ExportCSV(ctx, member, sheet.ID, 0, 1, &csv); err != nil || !strings.Contains(csv.String(), "'=UNSUPPORTED()") {
		t.Fatalf("export did not neutralize formula: %v", err)
	}
	if _, err := svc.Apply(ctx, member, sheet.ID, change(4, set(0, 0, ""))); err != nil {
		t.Fatal(err)
	}
	window, err = svc.ReadWindow(ctx, member, sheet.ID, Area{Height: 1, Width: 1})
	if err != nil || len(window.Cells) != 0 {
		t.Fatalf("clear did not delete sparse cell: %+v %v", window, err)
	}
}
