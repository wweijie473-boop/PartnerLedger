# 三端项目协作接口提案 · v1

**状态：Go 服务已实现客户身份、组织绑定、项目查询和创建，以及询价、撤回、缺口、补资料操作。** 现行接口以 [OpenAPI](../backend/openapi.yaml) 为准。报价、订单、交付和商机流程仍为后续接口提案。正式 Vue 工作台的对接见 [Issue #1](https://github.com/wweijie473-boop/PartnerLedger/issues/1)。三端演示使用本地样例。

## 身份和可见范围

继续使用 `POST /api/auth/login`，请求为 `{"username":"采购账号","password":"<输入值>"}`，成功返回 `data: {access_token,token_type:"Bearer",expires_in,user}`。客户的 `GET /api/user/profile` 和 `GET /api/org/current` 返回同一组织，例如 `{"id":"31","name":"示例采购企业","kind":"CUSTOMER"}`。维护者开通组织和账号并登记代理关系，步骤见 [后端启动说明](../backend/README.md)。

| 调用方 | 查询范围与明细 |
| --- | --- |
| 客户 | 自己组织的项目、原始需求、补充资料、对客报价及历史、订单的销售报价版本、公开交付和验收记录。不返回询价摘要、渠道价、渠道版本编号或上游私有事件。 |
| 代理商 | 自己经办且客户已授权的项目；可见两套报价及历史，按目标对象撰写询价、补充说明与对客方案。 |
| 厂商 | 仅已提交至本厂的询价及相应订单。以产品/询价为标题，只返回提交的摘要、渠道报价及版本、授权交接及最终完结状态；不返回原始客户名称、内部预算、对客售价和整改意见。 |

列表、详情、订单引用、动作响应和时间线必须做同样的**服务端字段投影**；隐藏页面按钮不能代替权限校验。客户看到「代理商跟进中」而不是渠道改价，厂商看到「渠道报价已发送」而不是客户确认售价。供货说明会发给三方，请勿写入密钥、内部折扣等。多个同角色组织也须分别隔离。

## 资源、请求和响应

项目接口前缀 `P = /api/channel/v1/projects`。所有请求用 Bearer。沿用 Go 信封 `{"code":200,"message":"success","data":...}`，以下示例仅写 `data` 或其中标出的字段。项目 ID、报价版本 ID 均作为不透明字符串处理。客户和代理商可调用 `GET /api/channel/v1/organization-links`，返回授权关联的 `items: [{id,name,kind}]`，用于建项时选择关联组织。

| 操作 | 请求样例 | 响应样例 / 约束 |
| --- | --- | --- |
| 列项目 | `GET P?limit=50`；下一页 `GET P?limit=50&after=opaque` | `{"items":[ProjectView],"next_cursor":null}`；支持三方，仅本组织记录；本版每项为完整角色视图，便于列表、待办和详情切换。 |
| 查项目 | `GET P/PRJ-1042` | 当前身份的 `ProjectView`；不存在及跨组织均返回 404。 |
| 新项目 | `POST P {"title":"ERP 升级","requirement":"20 席财务授权","dueDate":"2027-06-01","requestId":"<UUID>"}` | 客户或代理商。成功返回 `ProjectView`，例如 `id:"PRJ-1042",revision:1,stage:"requested"`。客户组织由会话推导；代理商代录时传已关联的 `customerId`。客户建项须绑定经办代理商；示例为唯一绑定，多个绑定时须选择并校验 `resellerId`，不得按客户名推断归属。 |
| 办理动作 | `POST P/PRJ-1042/actions/submitInquiry {"requestId":"<UUID>","revision":1,"payload":{"product":"42","summary":"20 席财务授权，含实施"}}` | 返回办理人视角的最新完整 `ProjectView`，状态和版本随事务更新。下表枚举各动作及响应关键字段。 |

示例：客户刚收到销售报价时的详情（询价、渠道报价键均不存在）：

```json
{
  "id":"PRJ-1042", "title":"ERP 升级", "customerName":"示例采购企业",
  "requirement":"20 席财务授权", "product":"42", "dueDate":"2027-06-01",
  "revision":4, "stage":"sales_quoted", "stageLabel":"待客户审阅", "isTodo":true,
  "nextStep":"确认此版报价", "actions":["confirmSales","reviseSales"],
  "salesQuotes":[{"id":"SQ-1042-V1","version":1,"status":"active",
    "lines":[{"label":"软件授权","quantity":20,"unitPrice":"1500.00"}],
    "total":"30000.00","validUntil":"2027-05-20","deliveryDays":10,"note":"含实施服务"}],
  "timeline":[{"at":"2027-05-02T02:00:00Z","label":"客户提交需求","detail":"20 席财务授权"},
              {"at":"2027-05-04T08:30:00Z","label":"代理商发送对客报价","detail":"SQ-1042-V1"}]
}
```

## 动作与状态机

下表的输入是动作信封中的 `payload`；输出摘自成功后的完整视图。源状态表示服务端实际业务状态，客户端只使用服务端给当前身份的 `actions`。每个普通动作成功只增加一次 `revision`。

| 动作 / 办理者 | 允许的源状态 | 输入样例 | 响应关键字段或投影结果 |
| --- | --- | --- | --- |
| `submitInquiry` 代理商 | requested | `{"product":"42","summary":"20席，含实施"}` | `stage:"inquired",inquiry:{id:"RFQ-1042",summary:"20席，含实施",processed:false}`；从已发布产品确定厂商。 |
| `withdrawInquiry` 代理商 | inquired 且未处理 | `{}` | `stage:"requested",inquiry:null`；厂商不再能读此项目。 |
| `flagGap` 厂商 | inquired 且还未标记缺口 | `{"note":"可提供 ERP，暂不提供车间接口"}` | 保持 inquired，记录 `inquiry.gapNote` 并标为已处理；仅上游双方可见。代理商可人工转记[独立商机](opportunity-handoff.md)，不自动创建、不影响主单报价。 |
| `requestInfo` 厂商 / 代理商 | 厂商：inquired；代理商：requested 或 vendor_info | `{"note":"需确认授权数"}` | 厂商发问 → vendor_info；代理商向客户发问 → customer_info。只向相关双方记录问题。 |
| `replyInfo` 客户 / 代理商 | customer_info / vendor_info | `{"note":"首期20席"}` | 客户回复 → requested（已有询价则回 vendor_info）；代理商独立整理回复厂商 → inquired。 |
| `channelQuote` 厂商 | inquired、channel_quoted、channel_revision；未下单的商务阶段渠道价过期也可重发 | 示例 Q，单价 1200.00 | `channelQuotes[-1].id:"CQ-1042-V1",stage:"channel_wait"`（厂商视图）；代理商视图为 channel_quoted。 |
| `reviseChannel` 代理商 | channel_quoted、sales_revision、sales_quoted；confirmed 时渠道价过期 | `{"note":"请单列迁移费用"}` | `stage:"channel_revision"`，旧版不可再用于下单。 |
| `salesQuote` 代理商 | channel_quoted、sales_revision、sales_quoted；confirmed 且对客价过期。需有效渠道价 | 示例 Q，独立填写单价 1500.00 | `salesQuotes[-1].id:"SQ-1042-V1",stage:"sales_quoted"`；保留历史。 |
| `reviseSales` 客户 | sales_quoted（含过期） | `{"note":"请调整培训安排"}` | `stage:"sales_revision"`，代理商收到待办。 |
| `confirmSales` 客户 | sales_quoted 且当前对客版本有效 | `{"quoteId":"SQ-1042-V1"}` | `stage:"confirmed",salesQuotes[-1].status:"confirmed"`；此时没有订单。 |
| `placeOrder` 代理商 | confirmed，渠道/对客最新版本均有效 | `{"quoteId":"SQ-1042-V1"}` | `stage:"ordered",order:{id:"SO-1042",salesQuoteId:"SQ-1042-V1",channelQuoteId:"CQ-1042-V1"}`；按角色裁剪订单引用。 |
| `supply` 厂商 | ordered | `{"note":"20席授权已交接"}` | `stage:"shipped",supplyNote:"20席授权已交接"`（厂商视图）；代理商视图 supplied。 |
| `implement` 代理商 | supplied 或 rectification | `{"note":"部署及培训完成，请验收"}` | `stage:"acceptance",implementationNote:"部署及培训完成，请验收"`。 |
| `rectify` 客户 | acceptance | `{"note":"补充培训签到记录"}` | `stage:"rectification"`，写入客户与代理商可见时间线。 |
| `accept` 客户 | acceptance | `{"note":"通过"}`（说明选填） | `stage:"completed",actions:[]`；厂商收到完结状态，不收到验收意见。 |

示例 Q：`{"lines":[{"label":"软件授权","quantity":20,"unitPrice":"1200.00"},{"label":"实施","quantity":1,"unitPrice":"3000.00"}],"validUntil":"2027-05-20","deliveryDays":10,"note":"含上线支持"}`。币种 CNY，线价与合计用十进制字符串传输，服务端 DECIMAL 或整数分计算；总额由服务端汇总。有效期为含当天的中国标准时间日期，时间线用带时区时间戳。两套报价独立编号，版本状态为 active / superseded / confirmed / expired / ordered。上游版本改动后，对客旧版失效，须重新发出并由客户再次确认。

## 冲突、重试与逐步联调

- 相同组织、操作、资源及 `requestId` 重放同一请求须返回同一业务结果，不追加事件；同键不同内容返回 409。服务端**先检查会话和资源访问权，再检查幂等记录，再比较 revision**。订单加唯一约束；即使用新请求键对同一已确认版本重试，也只能得到原订单。订单、状态、事件、幂等结果在同一事务中写入。
- 过期、并发旧 revision、旧报价版本或不允许的状态返回 `409 {"code":409,"message":"revision_conflict","data":null}`（或 `quote_expired`、`invalid_transition`）；参数错误 400，失效会话 401，无角色办理权限 403，越组织读取或写入 404。超时时先读取项目确认结果；重放必须使用原请求键。冲突后展示最新记录，要求经办人重新核对，不自动覆盖。
- [演示仓库](../frontend/src/channel/repository.js) 提供 list/get/create/act、本地产品与订阅、重置。[HTTP 项目适配器](../frontend/src/channel/httpProjects.js) 已提供同名的四个业务方法，注入正式工作台的 authenticated 请求函数即可联调；写入额外支持传入原 requestId。适配器不自动切换到本地样例。组织权限、操作列表和时间线投影可对照 [演示状态机](../frontend/src/channel/domain.js)，权限边界最终以服务端为准。
- 接入顺序：客户身份及绑定 → 项目与询价 → 两种报价 → 订单及履约验收。正式工作台按就绪环节开放，目录和 AI 沿用已上线端点；不要在正式入口提供演示身份切换和重置。对每个角色至少用两个不同组织验证跨组织 404、列表隔离、金额字段不可见；再验收版本过期、超时重放、重复下单和退回整改。
