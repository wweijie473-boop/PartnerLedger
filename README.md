# PartnerLedger

渠智链 AI 是以软件二级代理商为核心的渠道协作平台：客户提出采购需求，代理商组织询价与对客方案，软件厂商负责供给，三方按权限跟进同一个项目直至验收。

- [团队分工与开发计划](团队分工与开发计划.md)
- [启动前端与体验三个工作台](frontend/README.md)
- [Go 后端启动说明](backend/README.md)
- [首批 OpenAPI 契约](backend/openapi.yaml)
- [三端项目协作接口提案与联调样例](docs/channel-contract.md)
- [商机拾遗 · 给后端伙伴的交班文档](docs/opportunity-handoff.md)

默认入口是公开门户。代理商及厂商可使用组织账号访问真实服务。三个工作台另有免账号的本地演示，包含可操作的 ERP 采购样例；正式客户身份、询报价、订单与验收接口仍待后端实现。

**体验商机拾遗：** 启动前端后访问 `/experience/dealer?view=opportunities`。打开预置的 ERP → MES 需求，可看到团队里谁能引荐、过去做过什么、还需要核实什么。登记引荐和跟进结果不会改变主订单。该模块当前只使用本地虚构数据。
