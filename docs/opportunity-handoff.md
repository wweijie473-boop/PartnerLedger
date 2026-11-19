# 商机拾遗｜给 mayisuix 的交班单

**Go 已提供商机台账、伙伴档案、跟进动作和拜访草稿接口。** 后端联调使用迁移 `000004` 和 [现行 OpenAPI](../backend/openapi.yaml)。正式前端需要接入这些接口。产品、前端、演示与最终联调由 wweijie473-boop 负责。

## 先看一笔业务（约三分钟）

1. 在 `frontend` 下运行 `npm ci`、`npm run dev`，进入 `/experience/dealer?view=opportunities`。打开示例“仓储财务一体化”的副需求：能看到 ERP → MES 的历史合作依据、内部引荐人和仍需核实的版本与排期。
2. 另开一个代理商项目，在「记录走访与补充需求」写“ERP 已定，客户还需要 IDE 接口，原厂没有”。整理草稿并人工确认。此时保留“IDE 接口”的歧义，不应直接断定为插件、连接器或定制开发；主项目原状态、报价、订单保持不变。
3. 向客户澄清后，例设为“系统对接服务 / ERP / WMS”。初始无相符案例，到「团队伙伴」登记同能力的团队、内部引荐人及经授权的交付案例；保存后回到可回访的旧商机。能力登记和有合作记录的候选须清楚区分。无候选也是正常结果。
4. 查看对外摘要（应只有能力/系统类型）；自行联系，不自动发微信。拿到双方同意后，登记引荐、约定介绍费及条件、下次跟进日期。检查台账「今天需跟进」。登记成功或未促成；成功且有约定费用后，才能手工登记已结清。可经授权把成交作为后续匹配依据；不能称为已交付验收。
5. 厂商演示端另有入口：打开待首次报价的询价 →「标记不支持的附加需求」。主询价仍可报价；代理商打开同一项目时可看到原厂说明，并以此作为商机草稿。客户不看到厂商说明；厂商、客户都看不到引荐与介绍费。

## 伙伴按此顺序接入

| 顺序 | mayisuix 的交付 | 可以开始前端联调的条件 |
| --- | --- | --- |
| 1. 项目与数据边界 | 按[三端协作提案](channel-contract.md)先提供真实项目 ID、经办组织和项目查询。为商机、伙伴能力、合作依据、引荐跟进事件建增量迁移；项目可有多笔副需求，主单与商机独立版本。补上厂商 `flagGap`。 | 同角色两家代理商读不到彼此记录；厂商仅能标记自己收到的未处理缺口，客户无该字段。 |
| 2. 捕捉与推荐 | 完成四个接口，用已确认的能力类别、源/目标系统做可追溯匹配。优先同系统且有获准使用的案例；返回依据、引荐人及局限，排除明确冲突。新增伙伴时反查仍开放的需求。 | 同一组样例能得到“待澄清 / 无人选 / 仅登记待核实 / 有合作记录”四种结果，不使用虚构分数。 |
| 3. 引荐与结果 | 校验双方同意、费用与跟进日期；写状态、事件、幂等结果的同一事务。正式实现保留同意确认人/时间/来源、金额 DECIMAL、版本及审计。 | 重复点击、超时重放、两个标签使用旧版本，不多记引荐/介绍费；无权限不泄露主单和人脉。 |
| 4. 可选 AI 增强 | 授权的拜访记录仅作候选抽取，模型输出需求片段、字段及追问，禁止自动保存/联系/结算。脱敏后再按项目约定调用模型；模型不可用可手工录入。 | 关键词不清楚时保留未知；推荐给出真实证据引用，无案例不能称已验证。前端演示目前是本地规则。 |

## 商机接口 v1

正式界面可注入现有 [HTTP 适配器](../frontend/src/channel/httpOpportunities.js)。演示使用 [本地仓库](../frontend/src/channel/repository.js)。Go 响应沿用 `{code,message,data}` 信封，适配器的 authenticated 请求函数返回已解包的 data。前缀 `B=/api/channel/v1/opportunities`。

| 端点 / 方法 | 请求体样例 | 成功 data |
| --- | --- | --- |
| `GET B/desk` / opportunityDesk | 无；从 Bearer 取得代理商组织 | `{items:[Opportunity],partners:[Partner],gaps:[{category,label,count}],reverseMatches:[{partnerId,opportunityId}]}`；只限本组织经办项目。首版为整体快照；大数据量应另行设计分页与聚合。 |
| `POST B` / captureOpportunity | `{"requestId":"<uuid>","projectId":"42","payload":{"note":"客户还需仓储对接","category":"integration","source":"erp","target":"wms"}}` | Opportunity；新记录 `revision:1,stage:"open"`，允许类型或系统待澄清。关联项目由服务端鉴权。 |
| `POST B/partners` / addPartner | `{"requestId":"<uuid>","payload":{"name":"示例团队","owner":"内部经办人","category":"integration","source":"erp","target":"wms","evidence":"经授权的合作案例简述"}}` | Partner：`{id,name,owner,category,source,target,evidence}`；保存后重新 GET desk 得到可回访列表，演示无联系人电话字段。生产应将内部成员和案例绑定可审计 ID。 |
| `POST B/{id}/actions/{action}` / actOpportunity | `{"requestId":"<uuid>","revision":1,"payload":{...}}` | 更新后的 Opportunity。响应不包含其他组织的资料；已知密钥、余额、支付或发票均不在范围内。 |

Opportunity 示例（仅代理商内部可读；示例编号非数据库主键）：

```json
{
  "id":"GAP-11","projectId":"42","projectTitle":"ERP采购","note":"客户还要 MES 工单接口",
  "category":"integration","source":"erp","target":"mes","revision":1,"stage":"open",
  "partnerId":"","followUp":"","feeAmount":"","feeTerms":"","outcome":"",
  "history":[{"at":"2026-11-19T08:00:00Z","label":"确认记录附加需求"}],"questions":[],
  "matches":[{"partnerId":"PPL-1","name":"示例团队","owner":"王经理","evidence":"某工单同步项目验收记录","reason":"系统对接服务 · ERP · MES","caveat":"版本、档期和报价仍需核实。"}],
  "brief":"有一项企业软件配套需求。能力：系统对接服务。涉及系统：ERP → MES。请确认支持的版本、交付方式和是否可承接。"
}
```

能力枚举：integration / connector / plugin / migration / training / other；系统枚举：erp / mes / oa / ide / wms / crm / other，空串表示未确认。不要从“IDE 接口”推导供需双方。对外 brief 由白名单模板生成，**不拼接自由文本**：不得包含走访原文、单位、预算、人名、电话、佣金、项目编号。前端仅复制摘要；不读微信、不读取或上传通讯录。

| action | 前置条件 | payload / 结果 |
| --- | --- | --- |
| clarify | open | `{category,source,target}`；更新匹配，缺字段则列出 questions。 |
| introduce | open；同组织候选、已核实能力、双方同意 | `{partnerId,consent:true,followUp:"2026-12-01",feeAmount:"1500.00",feeTerms:"客户验收后"}` → introduced。金额可不约定；正金额必须附条件。本版 consent 代表经办人手工确认两项。 |
| follow | introduced，或有待结清款的 won | `{followUp:"2026-12-03"}`；变更跟进日期，不变阶段。台账显示今日及逾期项，不主动发送通知。 |
| win / lose | win：introduced；lose：open 或 introduced | `{outcome:"结果摘要",remember:false}` → won / lost。win 且 remember 为 true 才追加获授权的成交依据，只取编号与能力类别。 |
| settle | won 且约定金额为正 | `{}` → settled。仅登记人工确认的结果，不发生资金交易。 |

认证失效返回 401，非代理商办理返回 403，跨组织主单、伙伴或商机返回 404。参数无效或缺少同意返回 400。旧版本、状态冲突、同键不同内容和同项目重复附加描述返回 409。写入先验证权限，再检查幂等键和 revision。用原请求体和原 `requestId` 重放可得到原结果。商机动作和拜访写入的重放按成员隔离，伙伴登记按组织隔离。商机动作在同一事务写入状态与事件。

## 正式前端接入

演示复现见 [三端演示脚本](demo-walkthrough.md)。正式「记拜访」当前使用本机存储。接入服务时，使用服务端记录的 ID 和 revision。原始草稿仅允许同组织的创建成员读取或修改。确认产生的独立商机按经办组织可见。

商机增量沿用上文路径及 `requestId`、`revision` 约定。

- Opportunity 增加 `knownPartnerIds`、`contactReviews`、`rejections`、`followReason`，旧数据缺省为空。contactReviews 以 partnerId 为键，内容为 `{checks:{version,interfaceAccess,region,availability},note}`，每项为 unknown / confirmed / blocked。只有经办代理商内部成员可见这些字段。
- 新动作 `contactReview` 输入 `{partnerId,checks,note}`，open 状态可记录联系前核对；有 blocked 项时暂不可登记引荐。`rejectPartner` 输入 `{partnerId,reason}`，保留排除原因并从本商机当前候选中移除此伙伴，商机继续开放。
- 新动作 `acknowledge` 输入空对象，记录当前成员查看过的候选。演示按团队记录 knownPartnerIds，正式实现应按成员保存已读状态，避免一人查看导致其他人漏掉新伙伴。
- `follow` 支持 open 状态，输入 `{followUp,followReason}`，事项为 customer / partner / fee。引荐成功后的默认事项为伙伴回访；已促成但有待核对介绍费时显示计费条件。GET desk 增加 `tasks:[{opportunityId,projectTitle,reason,action,date}]`，只返回当前组织可见内容，关闭或结清后相应事项消除。
- 介绍费只记录约定与人工结清。引荐的服务端审计保存确认经办人、时间、手工确认来源及本次约定。

### 拜访草稿接口

前缀 `V=/api/channel/v1/visits`。除创建时传 `revision:0`，所有写入都带服务端给出的当前 `revision` 和 UUID 格式的 `requestId`。

| 操作 | 请求与响应 |
| --- | --- |
| `GET V` | 返回当前成员的 `{items:[Visit]}`，按最近更新排序。 |
| `GET V/{id}` | 返回当前成员可查看的 `Visit`。 |
| `POST V` / `PUT V/{id}` | 发送 `{requestId,revision,payload:{projectId,text,main,additional,reviewed}}`。保存草稿时 `projectId` 可为空。创建返回 201，更新返回 200。 |
| `POST V/{id}/confirm` | 人工复核后发送 `{requestId,revision,confirmed:true,createOpportunity:true}`。可选 `category,source,target`，未知时留空。必须先保存已复核草稿并关联真实项目。返回含 `opportunityId` 的新版 `Visit`。仅记录复核时将 `createOpportunity` 设为 false。 |
| `DELETE V/{id}` | 发送 `{requestId,revision}`，成功返回 `{id,revision,deleted:true}`。草稿移出列表，保留审计和已确认商机。 |

`Visit` 返回草稿字段和 `id,revision,updatedAt,confirmed,opportunityId`。相同项目与描述已有商机时，确认拜访复用已有记录。同一次确认保存草稿状态、商机和审计。主采购事项留在拜访草稿，项目需求经主单流程更新。

三端验收应核对同一项目编号。客户需求和对客资料由客户与代理商查看；厂商处理代理商提交的摘要；人脉、排除原因、合作费用、联系清单和拜访原文均只在经办代理商内部流转。模型调用仅使用获授权的内容，待办和提醒由真实记录与状态生成。

- 正式 OpenAPI 更新、迁移与回退说明、两家虚构代理商及厂商/客户的测试账号开通方式、可重置种子数据；令牌或数据库密码不入仓库。
- 权限、摘要泄露、同意缺失、费用和幂等并发的测试结果。用带电话、联系人、预算的样例验证这些字段不会进入对外摘要、厂商/客户响应或模型日志。
- 联调时以真实角色、持久化、审计通过为开放条件；正式工作台注入 HTTP 适配器，无接口时显示未开放，不回退本地假记录。前端回归入口：`cd frontend` 后运行 `npm test` 和 `npm run build`。
- 试用衡量：确认保留多少副需求、多少次引荐实际促成合作、多少旧线索被新伙伴重新匹配。区分“找到候选”“已获同意引荐”“已促成”，演示数量不当作真实商业成果。
