package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"
)

type ToolExecutor func(ctx context.Context, name string, args json.RawMessage) (result string, linkKey string, statusLabel string, err error)

type StreamCallbacks struct {
	OnStatus   func(label string)
	OnRoute    func(route WorkspaceRoute)
	OnRawRoute func(raw json.RawMessage)
	OnDelta    func(delta string)
	OnDone     func(links []string, model string)
}

type AgentStreamConfig struct {
	SystemPrompt string
	ActorHeader  string
	Tools        []ToolDefinition
	AllowedLinks map[string]bool
	RawRouteMode bool
}

type streamedToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func (d *DeepSeek) StreamWorkspace(ctx context.Context, actorHeader string, history []ChatMessage, exec ToolExecutor, cb StreamCallbacks) error {
	return d.StreamWorkspaceWithConfig(ctx, AgentStreamConfig{
		SystemPrompt: agentSystemPrompt,
		ActorHeader:  actorHeader,
		Tools:        WorkspaceTools(),
		RawRouteMode: false,
	}, history, exec, cb)
}

func (d *DeepSeek) StreamWorkspaceWithConfig(ctx context.Context, cfg AgentStreamConfig, history []ChatMessage, exec ToolExecutor, cb StreamCallbacks) error {
	if d.apiKey == "" || len(d.models) == 0 {
		return ErrUnavailable
	}
	if len(history) == 0 || len(history) > 16 {
		return ErrResponse
	}
	for i := range history {
		role := strings.TrimSpace(history[i].Role)
		content := strings.TrimSpace(history[i].Content)
		if (role != "user" && role != "assistant") || content == "" || utf8.RuneCountInString(content) > 4000 {
			return ErrResponse
		}
		history[i].Role = role
		history[i].Content = content
	}
	if history[len(history)-1].Role != "user" {
		return ErrResponse
	}

	candidates := d.orderedCandidates()
	sawUnavailable := false
	for index, model := range candidates {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
		}
		emittedAnyDelta := false
		outcome, err := d.runStreamingAgentWithModel(ctx, model, cfg, history, exec, StreamCallbacks{
			OnStatus:   cb.OnStatus,
			OnRoute:    cb.OnRoute,
			OnRawRoute: cb.OnRawRoute,
			OnDelta: func(delta string) {
				emittedAnyDelta = true
				if cb.OnDelta != nil {
					cb.OnDelta(delta)
				}
			},
			OnDone: cb.OnDone,
		})
		if outcome == outcomeSuccess {
			d.clearCooldown(model)
			return nil
		}
		if outcome == outcomeAuthFatal || emittedAnyDelta {
			return err
		}
		switch outcome {
		case outcomeQuotaOrModelMissing:
			sawUnavailable = true
			d.markCooldown(model, quotaCooldown)
		case outcomeTransientUnavailable:
			sawUnavailable = true
			d.markCooldown(model, transientCooldown)
		case outcomeInvalidOutput:
			d.markCooldown(model, transientCooldown)
		}
		if index+1 < len(candidates) {
			log.Printf("ai stream failover: %s failed (%v), switching to %s", model, err, candidates[index+1])
		}
	}
	if sawUnavailable {
		return ErrUnavailable
	}
	return ErrResponse
}

func (d *DeepSeek) runStreamingAgentWithModel(
	ctx context.Context,
	model string,
	cfg AgentStreamConfig,
	history []ChatMessage,
	exec ToolExecutor,
	cb StreamCallbacks,
) (callOutcome, error) {
	sys := cfg.SystemPrompt
	if sys == "" {
		sys = agentSystemPrompt
	}
	if strings.TrimSpace(cfg.ActorHeader) != "" {
		sys += "\n\n【当前会话身份】" + strings.TrimSpace(cfg.ActorHeader)
	}
	messages := make([]map[string]any, 0, len(history)+6)
	messages = append(messages, map[string]any{"role": "system", "content": sys})
	for _, msg := range history {
		messages = append(messages, map[string]any{"role": msg.Role, "content": msg.Content})
	}

	tools := cfg.Tools
	if tools == nil {
		tools = WorkspaceTools()
	}
	collectedLinks := make([]string, 0, 3)
	emittedInPriorRound := false

	const maxToolRounds = 3
	for round := 0; round <= maxToolRounds; round++ {
		allowTools := round < maxToolRounds && exec != nil
		needParagraphBreak := emittedInPriorRound
		roundDelta := func(chunk string) {
			if needParagraphBreak {
				needParagraphBreak = false
				if !strings.HasPrefix(chunk, "\n") && cb.OnDelta != nil {
					cb.OnDelta("\n\n")
				}
			}
			if cb.OnDelta != nil {
				cb.OnDelta(chunk)
			}
		}
		toolCalls, emittedText, outcome, err := d.streamSingleRound(ctx, model, messages, tools, allowTools, true, roundDelta)
		if outcome != outcomeSuccess {
			return outcome, err
		}
		if emittedText {
			emittedInPriorRound = true
		}

		if len(toolCalls) == 0 {
			if !emittedText {
				return outcomeInvalidOutput, ErrResponse
			}
			if cb.OnDone != nil {
				links := filterAllowedChatLinks(collectedLinks)
				if cfg.AllowedLinks != nil {
					links = filterCustomLinks(collectedLinks, cfg.AllowedLinks)
				}
				cb.OnDone(links, model)
			}
			return outcomeSuccess, nil
		}

		// Check if the LLM invoked propose_workflow_skill to open an approval dialog
		for _, tc := range toolCalls {
			if tc.Name == "propose_workflow_skill" {
				if cfg.RawRouteMode {
					rawArgs := strings.TrimSpace(tc.Arguments.String())
					var check struct {
						Intent string `json:"intent"`
					}
					if json.Unmarshal([]byte(rawArgs), &check) != nil || strings.TrimSpace(check.Intent) == "" {
						return outcomeInvalidOutput, ErrResponse
					}
					if cb.OnRawRoute != nil {
						cb.OnRawRoute(json.RawMessage(rawArgs))
					}
					return outcomeSuccess, nil
				}
				route, parseErr := parseProposedWorkflowSkill(tc.Arguments.String())
				if parseErr != nil {
					return outcomeInvalidOutput, parseErr
				}
				if cb.OnRoute != nil {
					cb.OnRoute(route)
				}
				return outcomeSuccess, nil
			}
		}

		assistantToolCalls := make([]map[string]any, 0, len(toolCalls))
		for _, tc := range toolCalls {
			args := strings.TrimSpace(tc.Arguments.String())
			if args == "" {
				args = "{}"
			}
			assistantToolCalls = append(assistantToolCalls, map[string]any{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]any{
					"name":      tc.Name,
					"arguments": args,
				},
			})
		}
		messages = append(messages, map[string]any{
			"role":       "assistant",
			"content":    "",
			"tool_calls": assistantToolCalls,
		})

		for _, tc := range toolCalls {
			argsRaw := strings.TrimSpace(tc.Arguments.String())
			if argsRaw == "" {
				argsRaw = "{}"
			}
			if preHint := defaultToolStatusHint(tc.Name); preHint != "" && cb.OnStatus != nil {
				cb.OnStatus(preHint)
			}
			resultText, linkKey, statusLabel, toolErr := exec(ctx, tc.Name, json.RawMessage(argsRaw))
			if statusLabel != "" && statusLabel != defaultToolStatusHint(tc.Name) && cb.OnStatus != nil {
				cb.OnStatus(statusLabel)
			}
			if linkKey != "" {
				collectedLinks = append(collectedLinks, linkKey)
			}
			if toolErr != nil {
				resultText = "工具查询暂时失败：" + toolErr.Error()
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      resultText,
			})
		}
		if cb.OnStatus != nil {
			cb.OnStatus("正在结合查询数据生成回答")
		}
	}
	return outcomeInvalidOutput, ErrResponse
}

func defaultToolStatusHint(name string) string {
	switch name {
	case "get_business_overview":
		return "正在读取全局经营统计概览"
	case "query_customers":
		return "正在按需检索客户档案"
	case "query_renewals_and_orders":
		return "正在查询直购订单与续费回访台账"
	case "query_projects":
		return "正在查询协作项目与报价进展"
	case "query_opportunities_and_partners":
		return "正在查询商机拾遗与团队合作人脉"
	case "query_service_tickets_and_finance":
		return "正在查询服务工单与财务申请记录"
	case "query_product_catalog":
		return "正在查询已发布软件目录与产品知识库"
	case "vendor_overview":
		return "正在读取厂商渠道运营总览"
	case "vendor_query_inquiries":
		return "正在查询渠道询价与供货进展"
	case "vendor_query_products":
		return "正在读取本厂软件目录与知识库"
	case "vendor_query_orders_and_settlements":
		return "正在查询直购收款与佣金结算明细"
	case "vendor_query_finance_and_tickets":
		return "正在查询渠道财务申请与售后工单"
	case "customer_query_projects":
		return "正在查询本企业采购项目与对客报价"
	case "customer_query_orders_and_dealers":
		return "正在查询直购订单与合作代理商"
	default:
		return ""
	}
}

func filterCustomLinks(links []string, allowed map[string]bool) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, 2)
	for _, item := range links {
		key := strings.TrimSpace(item)
		if allowed[key] && !seen[key] {
			seen[key] = true
			out = append(out, key)
			if len(out) == 2 {
				break
			}
		}
	}
	return out
}

func parseProposedWorkflowSkill(rawArgs string) (WorkspaceRoute, error) {
	var raw struct {
		Intent                string   `json:"intent"`
		UseProjectRequirement *bool    `json:"use_project_requirement"`
		MarkupPercent         *float64 `json:"markup_percent"`
		DeliveryDays          *int     `json:"delivery_days"`
		ValidDays             *int     `json:"valid_days"`
		Quantity              *int     `json:"quantity"`
		Note                  string   `json:"note"`
		InvoiceTitle          string   `json:"invoice_title"`
		TicketSubject         string   `json:"ticket_subject"`
		TicketCategory        string   `json:"ticket_category"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawArgs)), &raw); err != nil {
		return WorkspaceRoute{}, ErrResponse
	}
	useProj := false
	if raw.UseProjectRequirement != nil {
		useProj = *raw.UseProjectRequirement
	}
	slots, err := sanitizeWorkspaceSlots(&WorkspaceSlots{
		MarkupPercent:  raw.MarkupPercent,
		DeliveryDays:   raw.DeliveryDays,
		ValidDays:      raw.ValidDays,
		Quantity:       raw.Quantity,
		Note:           raw.Note,
		InvoiceTitle:   raw.InvoiceTitle,
		TicketSubject:  raw.TicketSubject,
		TicketCategory: raw.TicketCategory,
	})
	if err != nil {
		return WorkspaceRoute{}, err
	}
	switch raw.Intent {
	case "match_products", "prepare_inquiry", "submit_inquiry", "sales_quote", "place_order",
		"submit_acceptance", "capture_visit_opportunity", "assisted_order", "remind_commission",
		"apply_invoice", "create_ticket":
		return WorkspaceRoute{
			Intent:                raw.Intent,
			UseProjectRequirement: useProj,
			Slots:                 slots,
		}, nil
	default:
		return WorkspaceRoute{}, ErrResponse
	}
}

func (d *DeepSeek) streamSingleRound(
	ctx context.Context,
	model string,
	messages []map[string]any,
	tools []ToolDefinition,
	allowTools bool,
	disableThinking bool,
	onDelta func(string),
) ([]*streamedToolCall, bool, callOutcome, error) {
	payload := struct {
		Model      string            `json:"model"`
		Messages   []map[string]any  `json:"messages"`
		Tools      []ToolDefinition  `json:"tools,omitempty"`
		ToolChoice string            `json:"tool_choice,omitempty"`
		Stream     bool              `json:"stream"`
		MaxTokens  int               `json:"max_tokens"`
		Thinking   map[string]string `json:"thinking,omitempty"`
	}{
		Model:     model,
		Messages:  messages,
		Stream:    true,
		MaxTokens: 2400,
	}
	if allowTools && len(tools) > 0 {
		payload.Tools = tools
		payload.ToolChoice = "auto"
	}
	if disableThinking {
		payload.Thinking = map[string]string{"type": "disabled"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false, outcomeInvalidOutput, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, false, outcomeInvalidOutput, err
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Use a dedicated transport call without the short 25s whole-body timeout so long streams don't cut off mid-sentence
	streamClient := *d.client
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, false, outcomeTransientUnavailable, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rawErr, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		lowerErr := bytes.ToLower(rawErr)
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, false, outcomeAuthFatal, ErrResponse
		case resp.StatusCode == http.StatusPaymentRequired || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound:
			return nil, false, outcomeQuotaOrModelMissing, ErrUnavailable
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			return nil, false, outcomeTransientUnavailable, ErrUnavailable
		case resp.StatusCode == http.StatusBadRequest && disableThinking && bytes.Contains(lowerErr, []byte("thinking")):
			return d.streamSingleRound(ctx, model, messages, tools, allowTools, false, onDelta)
		default:
			return nil, false, outcomeInvalidOutput, ErrResponse
		}
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 2<<20))
	scanner.Buffer(make([]byte, 0, 64<<10), 256<<10)

	toolMap := make(map[int]*streamedToolCall)
	toolOrder := make([]int, 0, 4)
	emittedText := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if dataStr == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(dataStr), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		for _, tc := range delta.ToolCalls {
			entry, exists := toolMap[tc.Index]
			if !exists {
				entry = &streamedToolCall{ID: tc.ID, Name: tc.Function.Name}
				toolMap[tc.Index] = entry
				toolOrder = append(toolOrder, tc.Index)
			} else {
				if tc.ID != "" && entry.ID == "" {
					entry.ID = tc.ID
				}
				if tc.Function.Name != "" && entry.Name == "" {
					entry.Name = tc.Function.Name
				}
			}
			if tc.Function.Arguments != "" {
				entry.Arguments.WriteString(tc.Function.Arguments)
			}
		}
		if delta.Content != "" && len(toolOrder) == 0 {
			emittedText = true
			if onDelta != nil {
				onDelta(delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, emittedText, outcomeTransientUnavailable, ErrUnavailable
	}

	calls := make([]*streamedToolCall, 0, len(toolOrder))
	for i, idx := range toolOrder {
		entry := toolMap[idx]
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("call_%d", i+1)
		}
		calls = append(calls, entry)
	}
	return calls, emittedText, outcomeSuccess, nil
}
