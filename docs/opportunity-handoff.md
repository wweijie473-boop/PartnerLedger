# 商机拾遗｜给 mayisuix 的交班单

**现在可以接手的任务：为代理商私有的商机台账提供真实数据源。** Vue 页面、虚构样例、状态操作和 HTTP 客户端草案已随本次提交交付。Go 暂未实现下面的新端点；没有部署变更或真实客户数据。产品、前端、演示与最终联调仍由 wweijie473-boop 负责。

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

## 拟议接口 v1

实际调用形状见 [HTTP 适配器](../frontend/src/channel/httpOpportunities.js)；页面通过相同的仓库方法调用 [演示仓库](../frontend/src/channel/repository.js)。正式登录入口**没有启用**这些端点。Go 响应沿用现有 `{code,message,data}` 信封，适配器注入的 authenticated 请求函数返回已解包的 data。前缀 `B=/api/channel/v1/opportunities`。

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

认证失效 401；非代理商办理商机 403；跨组织主单/伙伴/商机 404；无效金额、日期、未获同意 400；旧 revision 或状态不符 409。先验证访问权，再查幂等键，再比 revision。相同组织/资源/动作/请求键重放同内容返回原结果，内容不同返回 409。一次写入产生一条业务事件，不要把客户自由文本写入公共日志。

## 交回前端时请一并给出

- 正式 OpenAPI 更新、迁移与回退说明、两家虚构代理商及厂商/客户的测试账号开通方式、可重置种子数据；令牌或数据库密码不入仓库。
- 权限、摘要泄露、同意缺失、费用和幂等并发的测试结果。用带电话、联系人、预算的样例验证这些字段不会进入对外摘要、厂商/客户响应或模型日志。
- 联调时以真实角色、持久化、审计通过为开放条件；正式工作台注入 HTTP 适配器，无接口时显示未开放，不回退本地假记录。前端回归入口：`cd frontend` 后运行 `npm test` 和 `npm run build`。
- 试用衡量：确认保留多少副需求、多少次引荐实际促成合作、多少旧线索被新伙伴重新匹配。区分“找到候选”“已获同意引荐”“已促成”，演示数量不当作真实商业成果。
