# PartnerLedger Go API

接口覆盖三种组织身份、产品目录、DeepSeek 需求解析、三端项目和询价，以及代理商的商机台账与拜访草稿。正式项目可办理两套报价、改版、客户确认、代理商下单、厂商供货、实施、整改和验收。

## 连接与启动

使用 Go 1.25 或更新版本。在本目录运行，下列命令以 PowerShell 为例。

1. 按 [.env.example](.env.example) 将配置注入当前进程环境。MySQL 使用团队专用的 `goapp` 库，Redis 使用团队专用实例。密钥和密码从交付文档或密钥保管工具读取。
2. 先检查连通性与认证，再执行 SQL 迁移。迁移只在显式运行 `migrate` 时执行。

   ```powershell
   go run ./cmd/partnerledger check
   go run ./cmd/partnerledger migrate
   ```

   `migrate` 会应用正式业务和表格工作台的两个 MySQL 迁移序列。`/readyz` 同时检查两套迁移与 Redis。只维护表格迁移时可运行 `tables-migrate`。

3. 创建厂家、代理商和客户开发账号。每次先在当前进程设置 `PARTNERLEDGER_BOOTSTRAP_PASSWORD`（12–72 字节），按需替换示例名称与用户名。记录命令输出的组织 ID。

   ```powershell
   go run ./cmd/partnerledger bootstrap-org -kind MANUFACTURER -name "演示厂家" -username vendor_demo
   go run ./cmd/partnerledger bootstrap-org -kind RESELLER -name "演示经销商" -username dealer_demo
   go run ./cmd/partnerledger bootstrap-org -kind CUSTOMER -name "演示采购方" -username customer_demo
   Remove-Item Env:PARTNERLEDGER_BOOTSTRAP_PASSWORD
   ```

4. 维护者记录已获授权的代理关系。把示例 ID 替换为上一步取得的实际组织 ID。

   ```powershell
   go run ./cmd/partnerledger link-customer -customer 31 -reseller 24
   ```

5. 启动 API 和检查就绪状态。该版本对应数据库迁移 `000009`。迁移新增渠道财务申请、服务费额度和工单及其操作记录。旧版服务检查其对应迁移版本，回退时使用与旧版对应的开发库备份。

   ```powershell
   go run ./cmd/partnerledger serve
   # 在另一终端检查 MySQL 迁移版本及 Redis
   Invoke-RestMethod http://127.0.0.1:8080/readyz
   ```

服务默认监听回环地址。前端开发时由 Vite 将 `/api` 代理到 `http://127.0.0.1:8080`。启动后参照 [OpenAPI](openapi.yaml) 调用接口，登录返回的 token 用在 `Authorization: Bearer <token>` 请求头中。AI 解析使用后端的 `DEEPSEEK_API_KEY` 和可配置的 `PARTNERLEDGER_DEEPSEEK_MODEL`。联调请使用脱敏的需求文本。

厂商通过 `/api/products/{id}/knowledge` 为本组织产品维护模块、部署方式、授权席位范围和资料摘录。代理商确认需求字段后，通过 `/api/ai/match-products` 核对已发布产品，查看逐项结论和厂商录入的来源。员工人数与授权席位分别记录。授权数未知或产品规则缺失时，匹配结果提示人工核对。

代理商成员可为待询价项目在 `/api/channel/v1/projects/{id}/inquiry-draft` 保存草稿。同一请求编号和内容可安全重试。草稿仅创建成员可读取，并与项目版本绑定。经办人打开项目核对后，通过 `submitInquiry` 项目操作正式提交。

三端项目通过 `GET /api/channel/v1/organization-links` 查询当前身份的可选关联方，再使用 `/api/channel/v1/projects` 建项和办理。重试同一写入须带回原 `requestId` 和请求体。收到 `revision_conflict` 时先重新加载详情。操作按钮以项目的 `actions` 为准。

代理商使用 `/api/channel/v1/opportunities` 管理团队商机与伙伴，通过 `/api/channel/v1/visits` 保存当前成员的拜访草稿。用两个不同经办组织以及同一组织的两个成员分别检查团队隔离和草稿归属。可用前述命令开通两个 `RESELLER` 组织、厂家和客户，并把示例客户授权给相应代理商。联调数据和请求约定见 [商机与拜访交班](../docs/opportunity-handoff.md)。

## 创建一组三端测试账号

在已执行迁移的测试库中，可以用一条命令完成上面的开户和关联。先为当前进程设置 `PARTNERLEDGER_MYSQL_DSN` 与自选的 `PARTNERLEDGER_BOOTSTRAP_PASSWORD`。三个账号使用本次输入的同一个测试密码。

```powershell
go run ./cmd/partnerledger bootstrap-test-workspaces
Remove-Item Env:PARTNERLEDGER_BOOTSTRAP_PASSWORD
```

- 代理商 `test_dealer` 从 `/login/dealer` 登录。
- 软件厂商 `test_vendor` 从 `/login/vendor` 登录。
- 企业客户 `test_customer` 从 `/login/customer` 登录。

命令会关联本组客户与代理商，厂商在发起询价时选择。多人共用一个测试库时，可传 `-prefix qa1`，生成 `qa1_dealer`、`qa1_vendor`、`qa1_customer`。用户名冲突或关联失败时整组回滚。

## 厂商直购与佣金

厂商在「订购价格与佣金」里为已发布产品设置直购单价和分佣比例。代理商可生成归属本组织的推广码，也可为已关联客户创建待确认订单。客户确认后向厂商付款，厂商在核对凭证后登记实收，系统按下单时锁定的比例生成应结佣金。厂商完成转账后登记结算凭证，代理商可以在待结算时催办。当前采用站外付款和转账凭证登记。

直购 API 使用 `/api/retail/v1`。推广订购页通过公开的 `GET /store/:code` 查询上架产品。其余接口要求登录，按经办组织授权。`GET /orders?order_id=...` 用于打开指定订单；`GET /orders?view=commission` 同时返回所属组织的待结和已结佣金总额（分，以十进制字符串返回）以及已催办未结算的笔数。汇总覆盖该组织的全部佣金记录。创建订单时提交唯一 `request_id` 和已确认价格 `expected_unit_cents`，响应未知时使用相同编号和参数重试。厂商登记收款和结算时使用凭证编号，代理商对同一订单每 24 小时可以登记一次催办。

### 代理商客户经营

正式运营接口使用 `/api/retail/v1/operations`。经营看板读取本组织的客户数、直购订单、未来 30 天到期数和待结佣金。客户档案汇总已有渠道关联和直购订单的客户，代理商可维护自己的联系人、备注及潜在客户。代客下单仍需正式渠道关联。

厂商在直购配置勾选年度授权后，新订单会锁定授权周期。厂商登记收款时生成一年后的到期日，按北京时间的日历计算。已有订单没有历史授权周期字段，需留存原业务信息供后续核对，迁移不会自动给它们设定到期日。代理商可按订单记录最近一次续费跟进。客户档案与续费数据位于 MySQL，Redis 保存登录会话。

### 财务台账和服务工单

在 `/api/retail/v1/operations` 下，代理商可提交预存入账、订单开票和服务费报销申请，厂商只办理本组织收到的申请。开票必须引用已收款的本代理商直购订单，同一订单只受理一份。预存和报销使用已有直购往来的厂商与凭证号。服务费额度由厂商按代理商设置，初始为零，待办理和已完成的申请都占用额度。金额上限为每次申请或每个合作额度 100 万元，按整数分计算。厂商在线下办理后登记凭证。

工单可关联直购订单，也可直接选择已有直购往来的厂商。代理商建单，厂商提交处理结果，代理商可关闭或补充后重开。表单提交使用工单版本，重试相同操作可核对之前的结果。工单历史和财务申请保存在 MySQL，只向对应组织返回。

## 验证

`go test ./...` 默认执行模型适配器测试。使用已迁移的开发库时，可按以下方式验证 MySQL、Redis、产品、三端权限、商机与拜访。测试会清理本次创建的账号、商品、项目、拜访、商机和会话。

```powershell
$env:PARTNERLEDGER_INTEGRATION_TEST = '1'
go test -count=1 ./...
```

要同时用 DeepSeek 验证一条脱敏样例，另设 `PARTNERLEDGER_LIVE_MODEL_TEST=1` 并注入有效的 `DEEPSEEK_API_KEY`。该测试会发起一次真实 API 调用。
