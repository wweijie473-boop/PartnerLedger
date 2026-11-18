# PartnerLedger Go API

首批接口覆盖登录和退出、当前组织、产品目录及 DeepSeek 需求解析。组织和首个账号由维护者通过命令创建。厂家可维护本组织产品，经销商可查看已发布产品并解析 ERP 需求。询价、报价、订单和交付接口待按团队计划实现。

## 连接与启动

使用 Go 1.25 或更新版本。在本目录运行，下列命令以 PowerShell 为例。

1. 按 [.env.example](.env.example) 将配置注入当前进程环境。MySQL 使用团队专用的 `goapp` 库，Redis 使用团队专用实例。密钥和密码从交付文档或密钥保管工具读取。
2. 先检查连通性与认证，再执行 SQL 迁移。迁移只在显式运行 `migrate` 时执行。

   ```powershell
   go run ./cmd/partnerledger check
   go run ./cmd/partnerledger migrate
   ```

3. 创建厂家和经销商各一个开发账号。每次先在当前进程设置 `PARTNERLEDGER_BOOTSTRAP_PASSWORD`（12–72 字节），按需替换示例名称与用户名。

   ```powershell
   go run ./cmd/partnerledger bootstrap-org -kind MANUFACTURER -name "演示厂家" -username vendor_demo
   go run ./cmd/partnerledger bootstrap-org -kind RESELLER -name "演示经销商" -username dealer_demo
   Remove-Item Env:PARTNERLEDGER_BOOTSTRAP_PASSWORD
   ```

4. 启动 API 和检查就绪状态。

   ```powershell
   go run ./cmd/partnerledger serve
   # 在另一终端检查 MySQL 迁移版本及 Redis
   Invoke-RestMethod http://127.0.0.1:8080/readyz
   ```

服务默认监听回环地址。前端开发时由 Vite 将 `/api` 代理到 `http://127.0.0.1:8080`。启动后参照 [OpenAPI](openapi.yaml) 调用接口，登录返回的 token 用在 `Authorization: Bearer <token>` 请求头中。AI 解析使用后端的 `DEEPSEEK_API_KEY` 和可配置的 `PARTNERLEDGER_DEEPSEEK_MODEL`。联调请使用脱敏的需求文本。

## 验证

`go test ./...` 默认执行模型适配器测试。使用已迁移的开发库时，可按以下方式验证 MySQL、Redis 和产品权限。测试会清理本次创建的账号、商品和会话。

```powershell
$env:PARTNERLEDGER_INTEGRATION_TEST = '1'
go test -count=1 ./...
```

要同时用 DeepSeek 验证一条脱敏样例，另设 `PARTNERLEDGER_LIVE_MODEL_TEST=1` 并注入有效的 `DEEPSEEK_API_KEY`。该测试会发起一次真实 API 调用。
