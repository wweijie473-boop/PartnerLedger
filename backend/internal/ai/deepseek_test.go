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
		content := `{"enterprise_type":"制造业","employee_count":120,"required_modules":["财务"],"deployment_mode":"","license_count":null,"missing_fields":["授权数"],"follow_up_questions":["需要多少账号？"]}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}},
		})
	}))
	defer mock.Close()
	model := NewDeepSeek("test-key", "test-model")
	model.endpoint = mock.URL
	result, err := model.ParseRequirement(context.Background(), "我们是120人的制造公司，需要财务模块")
	if err != nil {
		t.Fatal(err)
	}
	if result.EmployeeCount == nil || *result.EmployeeCount != 120 || result.LicenseCount != nil || !result.NeedUserConfirmation {
		t.Fatalf("unexpected requirement: %+v", result)
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
