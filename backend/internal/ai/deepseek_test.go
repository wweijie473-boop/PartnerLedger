package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
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
		content := "```json\n{\"enterprise_type\":\"制造业\",\"employee_count\":120,\"required_modules\":[\"财务\"],\"deployment_mode\":\"\",\"license_count\":null,\"version\":\" v5.1 \",\"missing_fields\":[\"license_count\"],\"follow_up_questions\":[\"需要多少账号？\"]}\n```"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}},
		})
	}))
	defer mock.Close()
	model := NewProvider(mock.URL, "test-key", "test-model")
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
	model := NewProvider(mock.URL, "test-key", "test-model")
	_, err := model.ParseRequirement(context.Background(), "示例")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected model unavailable, got %v", err)
	}
}

func TestModelPoolAutomaticallyFailsOverWhenFreeQuotaExhausted(t *testing.T) {
	var mu sync.Mutex
	calls := make([]string, 0)
	rateLimitSecond := true

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		calls = append(calls, payload.Model)
		shouldRateLimit := rateLimitSecond
		if payload.Model == "model-b" && rateLimitSecond {
			rateLimitSecond = false
		}
		mu.Unlock()

		switch payload.Model {
		case "model-a":
			// Simulate Volcengine Ark Safe Experience Mode 403 SetLimitExceeded when free quota hits 0.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"SetLimitExceeded","message":"free quota exhausted"}}`))
		case "model-b":
			if shouldRateLimit {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"finish_reason": "stop",
					"message":       map[string]string{"content": `{"intent":"sales_quote","use_project_requirement":true}`},
				}},
			})
		case "model-c":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"finish_reason": "stop",
					"message":       map[string]string{"content": `{"intent":"prepare_inquiry","use_project_requirement":true}`},
				}},
			})
		}
	}))
	defer mock.Close()

	provider := NewProvider(mock.URL, "test-key", "model-a, model-b, model-c")

	// Call 1: model-a 403 (quota exhausted) -> model-b 429 -> model-c 200!
	first, err := provider.RouteWorkspace(context.Background(), "帮我整理当前项目需求")
	if err != nil || first.Intent != "prepare_inquiry" {
		t.Fatalf("expected failover to model-c, got %+v, err=%v", first, err)
	}

	// Call 2: model-a & model-b are in cooldown, so model-c is tried FIRST directly!
	second, err := provider.RouteWorkspace(context.Background(), "帮我整理当前项目需求")
	if err != nil || second.Intent != "prepare_inquiry" {
		t.Fatalf("expected sticky healthy model-c, got %+v, err=%v", second, err)
	}

	mu.Lock()
	defer mu.Unlock()
	expected := []string{"model-a", "model-b", "model-c", "model-c"}
	if len(calls) != len(expected) {
		t.Fatalf("expected call sequence %v, got %v", expected, calls)
	}
	for i := range expected {
		if calls[i] != expected[i] {
			t.Fatalf("call %d: expected %s, got %s (all: %v)", i, expected[i], calls[i], calls)
		}
	}
}

func TestUnauthorizedStopsWithoutCascadingAllModels(t *testing.T) {
	count := 0
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mock.Close()

	provider := NewProvider(mock.URL, "bad-key", "model-a,model-b,model-c")
	_, err := provider.RouteWorkspace(context.Background(), "查看项目进展")
	if !errors.Is(err, ErrResponse) || count != 1 {
		t.Fatalf("expected immediate stop on 401 (count=1), got count=%d err=%v", count, err)
	}
}

func TestArkPrefixDefaultsToArkEndpointAndPool(t *testing.T) {
	provider := NewDeepSeek("ark-placeholder-key", "deepseek-flash")
	if provider.endpoint != defaultArkEndpoint || len(provider.models) < 3 || len(provider.audioModels) < 2 {
		t.Fatalf("expected Ark defaults, got endpoint=%s models=%v audioModels=%v", provider.endpoint, provider.models, provider.audioModels)
	}
}

func TestTranscribeAudioSkipsNonAudioModelAndFailsOver(t *testing.T) {
	var seen []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		seen = append(seen, payload.Model)
		if payload.Model == "text-only-model" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"audio input is not supported by this model"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"finish_reason": "stop",
				"message":       map[string]string{"content": `{"text":"给财务共享中心加价25%发对客报价"}`},
			}},
		})
	}))
	defer mock.Close()

	provider := NewProvider(mock.URL, "test-key", "text-only-model,audio-capable-model")
	text, err := provider.TranscribeAudio(context.Background(), "UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAIA+AAACABAAZGF0YQAAAAA=", "wav")
	if err != nil || text != "给财务共享中心加价25%发对客报价" {
		t.Fatalf("unexpected transcription: %q, err=%v", text, err)
	}
	if len(seen) != 2 || seen[0] != "text-only-model" || seen[1] != "audio-capable-model" {
		t.Fatalf("unexpected audio failover sequence: %v", seen)
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
			model := NewProvider(mock.URL, "test-key", "test-model")
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

func TestLiveArkFailoverAndExtraction(t *testing.T) {
	if os.Getenv("PARTNERLEDGER_LIVE_MODEL_TEST") != "1" {
		t.Skip("set PARTNERLEDGER_LIVE_MODEL_TEST=1 to run live model verification")
	}
	key := os.Getenv("DEEPSEEK_API_KEY")
	endpoint := os.Getenv("PARTNERLEDGER_DEEPSEEK_ENDPOINT")
	if key == "" {
		t.Fatal("DEEPSEEK_API_KEY required")
	}
	// Put an unavailable/unopened model FIRST to prove live automatic failover to the real pool!
	pool := "unopened-quota-zero-model-test," + os.Getenv("PARTNERLEDGER_DEEPSEEK_MODEL")
	provider := NewProvider(endpoint, key, pool)

	route, err := provider.RouteWorkspace(context.Background(), "给财务共享中心项目按渠道底价加价 28%，12 天交付，生成并发送对客报价")
	if err != nil || route.Intent != "sales_quote" {
		t.Fatalf("live route failed: %+v, err=%v", route, err)
	}
	t.Logf("Live RouteWorkspace succeeded after automatic failover: intent=%s slots=%+v", route.Intent, route.Slots)

	req, err := provider.ParseRequirement(context.Background(), "我们是一家120人的制造公司，需要ERP财务和库存模块，希望私有化部署，授权数量待确认。")
	if err != nil || req.EmployeeCount == nil || *req.EmployeeCount != 120 || req.LicenseCount != nil || len(req.RequiredModules) < 2 {
		t.Fatalf("live extraction failed: %+v, err=%v", req, err)
	}
	t.Logf("Live ParseRequirement succeeded on cached healthy model: %+v", req)
}
