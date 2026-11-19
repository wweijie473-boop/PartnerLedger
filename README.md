# PartnerLedger

渠智链 AI 是以软件二级代理商为核心的渠道协作平台：客户提出采购需求，代理商组织询价与对客方案，软件厂商负责供给，三方按权限跟进同一个项目直至验收。

- [团队分工与开发计划](团队分工与开发计划.md)
- [启动前端与体验三个工作台](frontend/README.md)
- [Go 后端启动说明](backend/README.md)
- [表格工作台与嵌入接口](docs/table-workspace.md)
- [首批 OpenAPI 契约](backend/openapi.yaml)
- [三端项目协作接口提案与联调样例](docs/channel-contract.md)
- [商机拾遗 · 给后端伙伴的交班文档](docs/opportunity-handoff.md)
- [三端协作与商机拾遗演示脚本](docs/demo-walkthrough.md)
- [蓝色企业界面改版与验收交接](docs/interface-redesign-handoff.md)

默认入口是公开门户。三种角色均可使用组织账号登录，共同办理建项、询价、撤回、补充资料和能力缺口。正式代理商工作台已接通团队商机、伙伴、引荐跟进和个人拜访草稿同步。三个工作台另有免账号的本地演示，包含 ERP 采购样例；正式报价、订单与验收仍待后续接入。

**体验商机拾遗：** 启动前端后访问 `/experience/dealer?view=opportunities`。打开预置的 ERP → MES 需求，可看到团队里谁能引荐、过去做过什么、还需要核实什么。登记引荐和跟进结果不会改变主订单。该模块当前只使用本地虚构数据。
