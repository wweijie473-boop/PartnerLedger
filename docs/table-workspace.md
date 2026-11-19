# 表格工作台

启动现有 PartnerLedger Go 服务和 Vue 前端后，使用组织账号登录 `/tables`。维护者先在开发数据库执行表格迁移。

```powershell
cd backend
go run ./cmd/partnerledger migrate
go run ./cmd/partnerledger tables-migrate
go run ./cmd/partnerledger serve
```

另一个终端在 `frontend` 运行 `npm ci` 和 `npm run dev`，打开终端地址的 `/tables`。表格迁移使用 `pl_sheet_schema_migrations`，表格数据使用 `pl_sheet_*`。

## 使用

- 从地址栏跳到 A1、AA100000 等单元格。双击或按 F2 编辑，Enter 和 Tab 可以提交并移动，Shift 配合方向键扩选，Delete 清空，Ctrl/Cmd+Z 撤销。
- 可从 Excel 粘贴矩形区域。每次最多 250 格。一个批次内任一格不通过类型校验时，整批保留待处理。
- 用工具栏「查找」或 Ctrl/Cmd+F 查找整张表，用 Ctrl/Cmd+H 打开替换。支持区分大小写、整格匹配和定位结果。替换先预览改动并勾选本页单元格，再作为一个批次保存；可撤销。默认跳过公式，勾选「包含公式原文」可检索公式文本。
- 编辑完成约 450 毫秒后尝试保存。在同一标签页刷新时，未同步的已提交单元格可从恢复草稿继续。出现版本冲突时，可刷新并显式重放自己的改动，或丢弃草稿。
- 通用与数值列支持同表 A1 引用、四则运算、括号以及 SUM、AVERAGE、MIN、MAX、COUNT。支持每个范围最多 256 格的计算。公式保留输入原文，页面显示计算结果。
- 表所有者可加行、追加列、重命名表和列，选择仅自己、组织内只读或组织内可编辑。CSV 可以用下载按钮导出。浏览器表格复制时保留本应用内部的公式，外部纯文本和 CSV 中可能触发公式执行的内容会被转为文本。

## 业务页面嵌入

引用 [DataGrid](../frontend/src/tables/DataGrid.vue)，为它提供一个数据源。下面使用表格 API 数据源。业务模块也可以实现同样三个方法，把读取和写入转成自己的业务接口。

```vue
<script setup>
import DataGrid from './tables/DataGrid.vue'
import { createTableRepository } from './tables/httpSource.js'

const props = defineProps({ request: Function, sheetId: String, accountId: String })
const source = createTableRepository(props.request).source(props.sheetId)
</script>

<template>
  <DataGrid :key="sheetId" :source="source" :recovery-key="'draft:table:' + accountId + ':' + sheetId" />
</template>
```

| 方法 | 约定 |
| --- | --- |
| `getSheet()` | 返回 `{id,name,rowCount,columns,revision,canEdit,isOwner}`。列顺序定义 A1 坐标，切换数据源时重新挂载组件。 |
| `getWindow({row,col,height,width}, signal)` | 返回 `{revision,cells:[{row,col,input}]}`。坐标从 0 起算，空值省略，版本与返回的格子来自同一个一致性快照。 |
| `saveChanges({requestId,revision,cells})` | 原子应用一批改动，返回 `{revision}`。重复请求键和相同内容返回同一保存结果，过期版本返回 HTTP 409。 |

需要只读展示时传 `read-only`。保存后可通过引用的 `flush()` 等待待提交批次。业务适配器在自己的后端执行权限、价格和业务状态校验，并以业务记录作为存储来源。

提供可选的 `search(query, signal)` 时，组件还会开放查找和替换。返回值遵循下方查找接口，实际替换通过同一个 `saveChanges` 提交。

## HTTP 调用

接口前缀为 `/api/tables/v1/sheets`。使用现有的 Bearer 会话。JSON 响应沿用 `{code,message,data}`。金额和公式输入以字符串传输。

| 调用 | 用途 |
| --- | --- |
| `GET /` | 分页读取可见表格。用 `after=nextCursor` 继续翻页。 |
| `POST /` | 创建表格，传 `requestId`、`name`、`rowCount`、`visibility` 和 `columns: [{label,kind}]`。重复创建键按组织和账号去重。 |
| `GET /{id}` | 读取表格元数据和操作权限。 |
| `GET /{id}/window?row=0&col=0&height=40&width=12` | 按矩形读取原始单元格，最多读取 2,048 格。 |
| `POST /{id}/search` | 全表查找。传 `text` 和可选 `matchCase`、`wholeCell`、`includeFormulas`、`replacement`。返回 `revision`、`matches`、已扫描格数 `scanned` 和 `nextCursor`。翻页带回 `after=nextCursor` 和原 `revision`。 |
| `POST /{id}/changes` | 写入最多 250 个坐标。空字符串清除单元格。所有者可以在同一请求中传完整 `settings` 更新元数据。 |
| `GET /{id}/csv?start=0&count=10000` | 按行范围导出 CSV，每次最多 10,000 行或 8 MiB 原始单元格内容。 |

```json
{
  "requestId": "e13495e1-125d-47ad-8422-06a6cb425f31",
  "revision": 3,
  "cells": [
    {"row": 0, "col": 1, "input": "20"},
    {"row": 0, "col": 2, "input": "=B1*1200"}
  ]
}
```

列类型为 `auto`、`text`、`number`、`date`、`checkbox`。可见范围为 `private`、`org_read`、`org_edit`。表的最大可寻址范围是 100,000 行 × 128 列，空单元格不占存储记录。已存在的列类型和坐标保留，扩容时追加行列。413 代表请求过大，422 在 `data.index` 给出无效单元格的数组位置。读取不可见表格返回 404，能看表但无权修改时返回 403。

查找按行列顺序分页，每次扫描最多 2,048 个非空单元格，最多返回 60 个结果。空结果页仍可能有 `nextCursor`，工作台会继续扫描直到找到一页匹配或到达表尾。`replacement` 为空字符串表示清除匹配内容，省略时只查找。替换预览含 `proposed`，不符合列类型或长度限制的结果标记为 `invalid`。用结果的 `revision` 调用 `changes` 保存所选格子。

## 测试

前端运行 `npm test` 和 `npm run build`。在专用开发库应用表格迁移后，为 Go 测试设置 `PARTNERLEDGER_MYSQL_DSN` 和 `PARTNERLEDGER_TABLES_INTEGRATION_TEST=1`，再运行 `go test ./internal/tables`。实库测试覆盖跨组织读取和导出、只读共享、批次原子性、并发重复请求和表末尾窗口读取。
