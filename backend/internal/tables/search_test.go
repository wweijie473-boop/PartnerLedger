package tables

import (
	"strings"
	"testing"
)

func TestSearchLiteralReplacementAndValidation(t *testing.T) {
	query := SearchInput{Text: "[erp]", WholeCell: false}
	matcher, err := compileSearch(query)
	if err != nil {
		t.Fatal(err)
	}
	replacement := "$1\\目录"
	got, ok := searchCell(matcher, Cell{Input: "[ERP] + [erp]"}, "text", &replacement)
	if !ok || got.Proposed == nil || *got.Proposed != "$1\\目录 + $1\\目录" {
		t.Fatalf("literal replacement mismatch: %+v", got)
	}
	query.WholeCell, query.MatchCase = true, true
	matcher, _ = compileSearch(query)
	if matcher.MatchString("[ERP]") || matcher.MatchString("[erp]\n") || !matcher.MatchString("[erp]") {
		t.Fatal("whole-cell case-sensitive search used partial or case-insensitive matching")
	}
	query.Text = "20"
	matcher, _ = compileSearch(query)
	got, ok = searchCell(matcher, Cell{Input: "20"}, "number", &replacement)
	if !ok || !got.Invalid || got.Proposed != nil {
		t.Fatal("non-numeric preview should be invalid")
	}
	query.Text, query.WholeCell = "a", false
	matcher, _ = compileSearch(query)
	replacement = strings.Repeat("b", 700)
	got, ok = searchCell(matcher, Cell{Input: "aa"}, "text", &replacement)
	if !ok || !got.Invalid || got.Proposed != nil {
		t.Fatal("overlength preview should be invalid")
	}
}
