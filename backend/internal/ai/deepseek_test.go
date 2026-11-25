package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRequirementRetainsUnknownLicenseCount(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("unexpected provider request")
		}
		var payload struct {
			Model          string            `json:"model"`
			ResponseFormat map[string]string `json:"response_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Model != "test-model" ||
			payload.ResponseFormat["type"] != "json_object" {
			t.Error("structured output not requested")
		}
		content := `{"enterprise_type":"制造业","employee_count":120,"required_modules":["财务"],"deployment_mode":"","license_count":null,"version":" v5.1 ","missing_fields":["license_count"],"follow_up_questions":["需要多少账号？"]}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}},
		})
	}))
	defer mock.Close()
	model := NewDeepSeek("test-key", "test-model")
	model.endpoint = mock.URL
	result, err := model.ParseRequirement(context.Background(), "我们是120人的制造公司，需要 v5.1 版本和财务模块")
	if err != nil {
		t.Fatal(err)
	}
	if result.EmployeeCount == nil || *result.EmployeeCount != 120 || result.LicenseCount != nil ||
		result.Version != "v5.1" || !result.NeedUserConfirmation {
		t.Fatalf("unexpected requirement: %+v", result)
	}
	withoutVersion, err := model.ParseRequirement(context.Background(), "我们是120人的制造公司，需要财务模块")
	if err != nil || withoutVersion.Version != "" {
		t.Fatalf("unexpected version for input without a version: %+v %v", withoutVersion, err)
	}
}

func TestParseRequirementHandlesProviderRateLimit(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer mock.Close()
	model := NewDeepSeek("test-key", "test-model")
	model.endpoint = mock.URL
	_, err := model.ParseRequirement(context.Background(), "示例")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected model unavailable, got %v", err)
	}
}

func TestWorkspaceRouterAcceptsOnlyRegisteredSkills(t *testing.T) {
	markup := 25.0
	qty := 3
	badMarkup := 900.0
	for _, test := range []struct {
		name   string
		intent string
		slots  *WorkspaceSlots
		valid  bool
	}{
		{"prepare_inquiry", "prepare_inquiry", nil, true},
		{"sales_quote_with_slots", "sales_quote", &WorkspaceSlots{MarkupPercent: &markup, Note: " 含首年巡检 "}, true},
		{"assisted_order_with_qty", "assisted_order", &WorkspaceSlots{Quantity: &qty}, true},
		{"capture_visit", "capture_visit_opportunity", nil, true},
		{"invalid_markup_rejected", "sales_quote", &WorkspaceSlots{MarkupPercent: &badMarkup}, false},
		{"delete_project", "delete_project", nil, false},
		{"empty_intent", "", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				content, _ := json.Marshal(WorkspaceRoute{
					Intent:                test.intent,
					UseProjectRequirement: true,
					Slots:                 test.slots,
				})
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(content)}}},
				})
			}))
			defer mock.Close()
			model := NewDeepSeek("test-key", "test-model")
			model.endpoint = mock.URL
			route, err := model.RouteWorkspace(context.Background(), "请处理当前业务")
			if test.valid {
				if err != nil || route.Intent != test.intent || !route.UseProjectRequirement {
					t.Fatalf("unexpected route: %+v, %v", route, err)
				}
				if test.name == "sales_quote_with_slots" && (route.Slots == nil || route.Slots.Note != "含首年巡检") {
					t.Fatalf("expected trimmed slots, got %+v", route.Slots)
				}
			} else if !errors.Is(err, ErrResponse) {
				t.Fatalf("unexpected error for %s: %v", test.name, err)
			}
		})
	}
}
