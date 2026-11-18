# PartnerLedger

渠智链 AI 是面向 ERP 软件厂家和经销商的询报协作项目。

- [团队分工与开发计划](团队分工与开发计划.md)
- [Vue 前端启动与双角色原型](frontend/README.md)
- [Go 后端启动说明](backend/README.md)
- [首批 OpenAPI 契约](backend/openapi.yaml)

默认入口使用 Go 后端的账号、组织和产品目录，经销商可调用需求解析。`/demo` 提供独立的 `PL-004` 双角色原型，询报价和交付演示使用浏览器里的模拟数据。开发服务器会将 `/api` 请求转发到本地 Go 服务。
