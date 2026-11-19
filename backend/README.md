# PartnerLedger Go API

接口覆盖三种组织身份、产品目录、DeepSeek 需求解析、三端项目和询价，以及代理商的商机台账与拜访草稿。三端可协作建项、询价、补资料与标记缺口。报价、订单和交付按团队计划接续。

## 连接与启动

使用 Go 1.25 或更新版本。在本目录运行，下列命令以 PowerShell 为例。

1. 按 [.env.example](.env.example) 将配置注入当前进程环境。MySQL 使用团队专用的 `goapp` 库，Redis 使用团队专用实例。密钥和密码从交付文档或密钥保管工具读取。
2. 先检查连通性与认证，再执行 SQL 迁移。迁移只在显式运行 `migrate` 时执行。

   ```powershell
   go run ./cmd/partnerledger check
   go run ./cmd/partnerledger migrate
   ```

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

5. 启动 API 和检查就绪状态。该版本对应数据库迁移 `000004`。此迁移新增代理商表，保留既有账号、产品和项目数据。若必须回退到按 `000003` 检查就绪的旧版服务，使用与旧版对应的数据库备份。迁移命令只提供向前迁移。

   ```powershell
   go run ./cmd/partnerledger serve
   # 在另一终端检查 MySQL 迁移版本及 Redis
   Invoke-RestMethod http://127.0.0.1:8080/readyz
   ```

服务默认监听回环地址。前端开发时由 Vite 将 `/api` 代理到 `http://127.0.0.1:8080`。启动后参照 [OpenAPI](openapi.yaml) 调用接口，登录返回的 token 用在 `Authorization: Bearer <token>` 请求头中。AI 解析使用后端的 `DEEPSEEK_API_KEY` 和可配置的 `PARTNERLEDGER_DEEPSEEK_MODEL`。联调请使用脱敏的需求文本。

三端项目通过 `GET /api/channel/v1/organization-links` 查询当前身份的可选关联方，再使用 `/api/channel/v1/projects` 建项和办理。重试同一写入须带回原 `requestId` 和请求体。收到 `revision_conflict` 时先重新加载详情。操作按钮以项目的 `actions` 为准。

代理商使用 `/api/channel/v1/opportunities` 管理团队商机与伙伴，通过 `/api/channel/v1/visits` 保存当前成员的拜访草稿。用两个不同经办组织以及同一组织的两个成员分别检查团队隔离和草稿归属。可用前述命令开通两个 `RESELLER` 组织、厂家和客户，并把示例客户授权给相应代理商。联调数据和请求约定见 [商机与拜访交班](../docs/opportunity-handoff.md)。

## 验证

`go test ./...` 默认执行模型适配器测试。使用已迁移的开发库时，可按以下方式验证 MySQL、Redis、产品、三端权限、商机与拜访。测试会清理本次创建的账号、商品、项目、拜访、商机和会话。

```powershell
$env:PARTNERLEDGER_INTEGRATION_TEST = '1'
go test -count=1 ./...
```

要同时用 DeepSeek 验证一条脱敏样例，另设 `PARTNERLEDGER_LIVE_MODEL_TEST=1` 并注入有效的 `DEEPSEEK_API_KEY`。该测试会发起一次真实 API 调用。
