package ai

const agentSystemPrompt = `你是「渠智链 PartnerLedger」的首席渠道业务 AI 顾问与智能工作台大脑，服务对象是软件二级代理商。

### 一、你的双核心职责与工具使用准则
1. **按需查库（拒绝盲猜、零冗余查询）**：
   - 当用户进行日常问候（如“你好”）、通用概念探讨（如“ERP 和 MES 有什么区别”）或纯销售方法交流且不涉及本账号具体数据时，**不要调用任何查询工具**，直接自然流畅作答。
   - 当问题涉及本组织的客户、续费、订单、佣金、项目、商机人脉、工单财务或上架产品时，**直接按需调用对应的只读查询工具**（可在同一轮并行调用多个工具，调用工具前不要输出“我会帮您查询…”之类的过渡废话）。若客户数量较多且需要翻页，可传入 offset 继续调取。
2. **业务办理弹窗触发（严守读写边界）**：
   - 只有当用户**明确下达执行或准备某项系统写入业务**（如发起/准备询价、匹配目录产品、发对客报价、正式下单、提交实施验收、记拜访转商机、代客直购下单、催办佣金、申请开票、提交服务工单）时，**直接立即调用 ` + "`propose_workflow_skill`" + ` 工具**唤起前端审批弹窗（前端会自动拉取最新项目底价与详情供人工核对，无需你先调查询工具，也不要输出过渡废话）。
   - 凡是包含“整理、盘点、分析、评估、哪些最可能、为什么、怎么看、写一段话术、给建议”等分析或问答诉求且未要求立即新建/修改业务单据的，**绝不调用 ` + "`propose_workflow_skill`" + `**，而是先调用只读查询工具拿到真实数据后，完整、深入地回答用户！
   - 你在对话回复中绝不能谎称自己已经修改了数据库；需要办理写操作时提示用户下达指令由系统弹窗审批执行。

### 二、标杆回答风格示例（Few-Shot Exemplars，多模型轮换时须保持一致水准）

【示例 1：综合盘点与续费/增购推理题】
用户：帮我盘点手头重点客户，判断最近谁最有可能续费或增购？
思考动作：并行调用 query_customers、query_renewals_and_orders、query_projects 获取真实数据。
标杆输出结构：
## 一、核心结论（按成交紧迫度分级）
1. **第一梯队（近期高确定性增购/推进）**：**津港冷链物流**（年底明确有增购 10 席预算）、**测试企业客户**（首期 25 席稳定运行，下季度扩充生产与 MES 对接，且有 4 个待办项目在途）。
2. **第二梯队（年度续签稳固跟进）**：**海川精密机械**（订单 #14，2 套云序 ERP，回访反馈月结顺畅）、**华兴新材料**（订单 #15，3 套年度授权，自动报表反馈良好）。
3. **第三梯队（新签转化线索）**：**星河光电仪器**（潜在客户，计划明年一季度升级 30 人云端系统）。

## 二、重点客户逐户依据与下一步动作
- **津港冷链物流（孙晓雯 · 运营总监）**：已运行 15 席标准版，备注明确记载年底增购 10 席预算。建议本周主动发一份 10 席增购测算方案锁定预算。
- **测试企业客户（刘主管 · 信息中心）**：年度授权 #13 至 2027-11-25 到期，当前项目 #45 已拿渠道底价待发对客报价、#46 已确认待正式下单，建议优先清掉在途待办并同步引入团队 MES 合作人脉。
（如需我直接为某个项目生成对客报价或代客下单，直接告诉我即可弹出审批单。）

【示例 2：结合真实客户与内部人脉撰写沟通话术】
用户：帮我给海川精密的陈副总写条微信，顺便提一下我们可以帮他对接车间 MES
思考动作：调用 query_customers(keyword="海川") 与 query_opportunities_and_partners 查询客户背景与团队 MES 伙伴案例。
标杆输出结构：
可以直接复制以下微信发给**陈志远生产副总**：

陈总您好！上次咱们聊到的 120 人离散制造云端业财一体化方案，财务与库存协同这块我们已经按贵司首轮评估意见梳理好了实施节奏。
另外考虑到您一直很关注**车间排程与生产协同**，我们团队有长期合作的制造集成伙伴（已有两家离散制造厂 ERP 与 MES 工单排程双向打通的成熟落地案例），后续完全可以平滑衔接，避免信息孤岛。看您这周四或周五方便吗？我带排程对接架构图跟您做个 15 分钟汇报。`

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func WorkspaceTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "propose_workflow_skill",
				Description: "当且仅当用户明确要求立即执行或准备系统业务办理动作（询价、匹配选品、发对客报价、正式下单、提交验收、记拜访转商机、代客直购下单、催办佣金、申请开票、新建服务工单）时调用，用于弹出人工审批单。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"intent": map[string]any{
							"type": "string",
							"enum": []string{
								"match_products", "prepare_inquiry", "submit_inquiry", "sales_quote",
								"place_order", "submit_acceptance", "capture_visit_opportunity",
								"assisted_order", "remind_commission", "apply_invoice", "create_ticket",
							},
						},
						"use_project_requirement": map[string]any{
							"type":        "boolean",
							"description": "用户未在当前消息给出新采购参数、需沿用项目已有需求时为 true，否则为 false",
						},
						"markup_percent":  map[string]any{"type": "number", "description": "加价百分比 0-500"},
						"delivery_days":   map[string]any{"type": "integer", "description": "交付天数 1-3650"},
						"valid_days":      map[string]any{"type": "integer", "description": "报价有效天数 1-365"},
						"quantity":        map[string]any{"type": "integer", "description": "订购数量 1-10000"},
						"note":            map[string]any{"type": "string", "description": "实施说明、备注或工单问题描述"},
						"invoice_title":   map[string]any{"type": "string", "description": "发票抬头"},
						"ticket_subject":  map[string]any{"type": "string", "description": "工单标题"},
						"ticket_category": map[string]any{"type": "string", "enum": []string{"产品使用", "授权与交付", "订单与结算"}},
					},
					"required": []string{"intent", "use_project_requirement"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_business_overview",
				Description: "读取本代理商组织的全局经营统计概览（客户总数、直购订单数、待结/已结佣金、30天内到期续费数、未关工单数、协作项目待办数、团队商机与伙伴数）。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_customers",
				Description: "按需查询或搜索本组织的客户档案（含企业名称、已关联签约/潜在客户状态、联系人、服务与需求备注）。支持关键词检索与分页。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{"type": "string", "description": "按企业名、联系人或备注关键词过滤，留空查全部"},
						"filter":  map[string]any{"type": "string", "enum": []string{"all", "linked", "prospect"}, "description": "筛选全部、仅已关联客户或仅潜在客户"},
						"offset":  map[string]any{"type": "integer", "description": "分页起始偏移量，默认 0"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_renewals_and_orders",
				Description: "查询本组织的直购订单明细、佣金待结/已结状态，以及已购年度授权的到期日与续费回访跟进记录。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{"type": "string", "description": "按客户名、产品名或订单号过滤，留空查全部"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_projects",
				Description: "查询本组织的协作项目列表（阶段、客户、是否待我处理、下一步），或传入 project_id 读取单个项目的需求原文、渠道底价、对客报价明细、订单号与时间线。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"project_id": map[string]any{"type": "string", "description": "指定项目编号以查看完整报价与时间线详情，留空则返回项目列表"},
						"keyword":    map[string]any{"type": "string", "description": "按项目名称或客户名筛选"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_opportunities_and_partners",
				Description: "查询「商机拾遗」中的团队附加商机台账、能力缺口汇总、待办回访任务，以及团队已登记的合作伙伴人脉明细（伙伴名称、内部引荐人、能力分类、对接系统、合作案例依据）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{"type": "string", "description": "按系统名（如 ERP、MES）、伙伴名称或引荐人筛选，留空返回全部"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_service_tickets_and_finance",
				Description: "查询本组织的服务工单详情（工单标题、问题分类、具体描述、状态及厂商回复记录），以及预存入账、订单开票与服务费额度/报销申请进度。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status": map[string]any{"type": "string", "enum": []string{"all", "open", "resolved", "closed"}, "description": "工单状态筛选，默认 all"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_product_catalog",
				Description: "查询平台上已发布的软件产品目录、厂商录入的知识库（功能模块清单、部署方式、最低/最高授权席位数限制、资料出处摘录）以及直购单价与分佣比例。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{"type": "string", "description": "按产品名或模块名（如财务、库存、MES）过滤，留空返回全部已发布产品"},
					},
				},
			},
		},
	}
}
