package ai

const vendorSystemPrompt = `你是「渠智链 PartnerLedger」的软件厂商（ISV 原厂）智能运营与渠道管理顾问，服务对象是软件厂商团队。

### 一、你的双核心职责与权限准则
1. **按需查库（零冗余查询、严守原厂数据边界）**：
   - 日常寒暄或通用 SaaS/ERP 产品运营交流时，**不要调用任何查询工具**，直接自然作答。
   - 当问题涉及本厂商收到的渠道询价、自家产品目录与知识库、直购订单收款与代理商佣金结算、财务申请（预存/开票/报销）或渠道售后工单时，**直接按需调用对应的只读查询工具**（不要输出“我会帮您查询…”等过渡废话）。
2. **业务办理弹窗触发（严守人工审批边界）**：
   - 当且仅当厂商用户**明确要求执行写入操作**（录入/提取产品匹配知识库、配置产品直购单价与分佣、给渠道询价发送渠道底价、确认直购订单收款、登记代理商佣金结算、回复渠道售后工单）时，**直接立即调用 ` + "`propose_workflow_skill`" + ` 工具**弹出人工审批弹窗。
   - 凡属“盘点、分析、汇总、评估、草拟回复话术”等分析或咨询诉求，绝不调用 ` + "`propose_workflow_skill`" + `，先调用只读工具获取真实数据后深入作答。

### 二、标杆回答风格示例（Few-Shot）
用户：帮我盘点一下目前有哪些积压的渠道待办需要优先处理？
思考动作：并行调用 vendor_overview、vendor_query_inquiries、vendor_query_finance_and_tickets。
标杆输出结构：
## 一、紧急待办优先级盘点
1. **高优（直接影响成单与现金流）**：渠道询价待报价项目、直购订单待确认收款。
2. **中优（影响渠道满意度）**：渠道售后待回复工单、代理商开票与预存入账确认。

## 二、具体单据与办理建议
- 列出真实项目号、订单号、工单号及金额，提醒可直接下达指令弹出审批单一键处理。`

const customerSystemPrompt = `你是「渠智链 PartnerLedger」的企业客户数字化选型与采购验收顾问，服务对象是采购企业软件的甲方客户。

### 一、你的双核心职责与权限准则
1. **按需查库（专业客观、保护商业边界）**：
   - 日常寒暄或咨询数字化转型、ERP/MES/WMS 模块选型常识时，**不要调用查询工具**，直接客观解答。
   - 当问题涉及本企业的采购项目进度、代理商发来的对客报价方案、已购直购软件订单或已绑定代理商信息时，**直接调用对应的只读查询工具**（不要输出过渡废话）。
2. **业务办理弹窗触发（人工最终拍板）**：
   - 当且仅当企业用户**明确要求发起或确认业务操作**（把业务痛点整理立项并发起新采购项目、确认接受对客报价、确认验收项目交付、确认代理商代客直购订单）时，**直接立即调用 ` + "`propose_workflow_skill`" + ` 工具**唤起审批弹窗。
   - 若用户只是说“帮我梳理一下需求清单”或“帮我分析一下这份报价合不合理”，先查询或直接输出分析建议，不要强行弹窗。

### 二、标杆回答风格示例（Few-Shot）
用户：帮我看看目前名下几个采购项目进展到哪一步了，有没有需要我拍板的？
思考动作：调用 customer_query_projects、customer_query_orders_and_dealers。
标杆输出结构：
## 一、需您立即拍板的事项
- 清晰列出处于「待客户确认报价」「待客户验收」或「待确认代客订单」的单据编号、金额、有效期与交付天数。
## 二、推进中项目状态
- 列出代理商或厂商正在处理中的项目阶段与下一步预期，提示如需立项或确认报价可直接告知弹出审批单。`

func VendorSystemPrompt() string   { return vendorSystemPrompt }
func CustomerSystemPrompt() string { return customerSystemPrompt }

func VendorAllowedLinks() map[string]bool {
	return map[string]bool{
		"projects":                 true,
		"catalog":                  true,
		"pricing":                  true,
		"manufacturer-receipts":    true,
		"manufacturer-settlements": true,
		"vendor-finance":           true,
		"vendor-tickets":           true,
	}
}

func CustomerAllowedLinks() map[string]bool {
	return map[string]bool{
		"projects":      true,
		"retail-orders": true,
		"account":       true,
	}
}

func VendorWorkspaceTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "propose_workflow_skill",
				Description: "当且仅当厂商用户明确要求执行业务办理（提取并保存产品知识库、设置直购价格与佣金、向代理商发送渠道底价、登记直购收款、登记佣金结算、回复渠道售后工单）时调用，用于弹出人工审批窗。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"intent": map[string]any{
							"type": "string",
							"enum": []string{
								"vendor_save_knowledge",
								"vendor_set_pricing",
								"vendor_channel_quote",
								"vendor_confirm_receipt",
								"vendor_settle_commission",
								"vendor_resolve_ticket",
							},
						},
						"product_id":         map[string]any{"type": "string", "description": "目标产品编号"},
						"project_id":         map[string]any{"type": "string", "description": "目标渠道询价项目编号"},
						"order_id":           map[string]any{"type": "string", "description": "目标直购订单编号"},
						"ticket_id":          map[string]any{"type": "string", "description": "目标渠道工单编号"},
						"category":           map[string]any{"type": "string", "description": "产品分类，如 ERP 业财一体 / MES 生产制造"},
						"modules":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "支持模块列表"},
						"deployment_modes":   map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"cloud", "saas", "private", "on_premises"}}, "description": "部署方式代码列表"},
						"min_seats":          map[string]any{"type": "integer", "description": "最低授权席位数"},
						"max_seats":          map[string]any{"type": "integer", "description": "最高授权席位数"},
						"license_note":       map[string]any{"type": "string", "description": "授权规则说明"},
						"source_title":       map[string]any{"type": "string", "description": "知识库资料名称"},
						"source_reference":   map[string]any{"type": "string", "description": "资料页码或编号"},
						"source_excerpt":     map[string]any{"type": "string", "description": "产品资料关键摘录"},
						"unit_price":         map[string]any{"type": "number", "description": "单价（人民币元）"},
						"commission_percent": map[string]any{"type": "number", "description": "代理商分佣百分比 0-100"},
						"annual":             map[string]any{"type": "boolean", "description": "是否为年度订阅授权"},
						"quantity":           map[string]any{"type": "integer", "description": "报价数量"},
						"line_label":         map[string]any{"type": "string", "description": "渠道报价明细名称"},
						"delivery_days":      map[string]any{"type": "integer", "description": "预计交付天数"},
						"valid_days":         map[string]any{"type": "integer", "description": "报价有效天数"},
						"reference":          map[string]any{"type": "string", "description": "收款凭证号或佣金转账流水号"},
						"reply":              map[string]any{"type": "string", "description": "渠道工单处理结果回复或报价备注"},
					},
					"required": []string{"intent"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "vendor_overview",
				Description: "读取本软件厂商的全局渠道运营概览（待处理渠道询价数、已发布产品数、待确认收款单数、待结算佣金总额与单数、待办财务申请数、未关闭渠道工单数）。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "vendor_query_inquiries",
				Description: "查询代理商提交给本厂商的渠道询价与采购项目列表，或指定 project_id 查看询价需求摘要、能力缺口、本厂渠道底价历史与供货状态。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"project_id": map[string]any{"type": "string", "description": "可选，指定项目编号查看详情"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "vendor_query_products",
				Description: "查询本厂商名下所有软件产品、发布状态、已录入的产品匹配知识库（模块、部署方式、席位数范围、出处）及直购定价与分佣配置。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "vendor_query_orders_and_settlements",
				Description: "查询本厂商的直购/代客订单列表、待收款订单、待结算代理商佣金明细及代理商催办记录。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status": map[string]any{"type": "string", "enum": []string{"all", "payment", "settlement", "settled"}, "description": "订单状态筛选"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "vendor_query_finance_and_tickets",
				Description: "查询合作代理商提交的预存入账、订单开票、服务费报销申请、各代理商服务费额度，以及渠道售后工单与回复记录。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
	}
}

func CustomerWorkspaceTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "propose_workflow_skill",
				Description: "当且仅当企业客户明确要求执行业务操作（整理需求并发起新采购项目、确认接受代理商对客报价、确认项目实施验收、确认代客直购软件订单）时调用，用于弹出人工审批窗。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"intent": map[string]any{
							"type": "string",
							"enum": []string{
								"customer_create_project",
								"customer_confirm_quote",
								"customer_accept_delivery",
								"customer_confirm_order",
							},
						},
						"title":       map[string]any{"type": "string", "description": "新采购项目标题（如：华东生产基地 · ERP 与库存协同采购）"},
						"requirement": map[string]any{"type": "string", "description": "AI 根据用户描述梳理的结构化采购需求正文（含规模、所需模块、部署偏好、上线预期）"},
						"due_date":    map[string]any{"type": "string", "description": "期望上线或交付日期 YYYY-MM-DD"},
						"project_id":  map[string]any{"type": "string", "description": "目标采购项目编号"},
						"order_id":    map[string]any{"type": "string", "description": "目标直购软件订单编号"},
						"note":        map[string]any{"type": "string", "description": "验收意见说明"},
					},
					"required": []string{"intent"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "customer_query_projects",
				Description: "查询本企业名下所有软件采购项目的实时阶段、是否待我处理、代理商发来的对客销售报价明细与有效期、订单及实施记录。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"project_id": map[string]any{"type": "string", "description": "可选，指定项目编号查看完整对客报价行与时间线"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "customer_query_orders_and_dealers",
				Description: "查询本企业的厂商软件直购/代客订单（含待我确认订单、应付金额、年度授权信息）以及已绑定合作的代理商列表。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
	}
}
