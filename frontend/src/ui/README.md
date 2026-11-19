# PartnerLedger UI 组件库

运行前端后从工作台底部打开「组件库」，或访问 `/ui` 查看示例。适合在 Vite 应用内直接复用。静态部署时应让 `/ui` 返回同一个 `index.html`。

| 组件 | 用法 |
| --- | --- |
| `UiIcon` / `UiIconTile` | 用 `name` 引用 `icons.js` 中的业务图标。默认 18px。独立表达含义时给 `UiIcon` 加 `label`，带文字的按钮内使用默认装饰图标。 |
| `UiButton` | `primary`、`secondary`、`text`、`danger` 四种。支持 `icon`、`end-icon`、`compact`、`loading`、`disabled`。表单提交显式写 `type="submit"`。 |
| `UiCard` | 普通卡片或 `muted`。用 `title` 与 `description` 设置标题说明，用 `actions` 和 `footer` 插槽放操作。表格用 `flush` 贴齐卡片边缘。 |
| `UiMetricCard` | `label`、`value`、`description`；可选图标。只有可以跳转或筛选时才加 `interactive` 和点击事件。 |
| `UiBadge` | `neutral`、`brand`、`info`、`success`、`warning`、`danger`。配合状态文字使用，`dot` 显示对应的状态图标或圆点。 |
| `UiField` | 设置 `label`、`hint`、`error` 和 `required`，把插槽给出的 `control` 绑定到原生 `input`、`select` 或 `textarea`。业务校验由表单负责。 |
| `UiState` | `empty`、`loading`、`denied`、`error`。可在失败状态插入重试按钮。 |
| `UiProgress` | 用 `value` 传入 0–100 的百分比，`label` 提供进度说明。 |
| `UiProjectProgress` | 传入 `role`（dealer/vendor/customer）、当前身份可见的 `stage` 与 `label`。未知状态不显示进度；不从上游内部状态推断客户进度。 |

使用 `theme.css` 中的 `--ui-*` 变量定义颜色、文字、间距和圆角。蓝色用于主要操作和选中项，白色卡片放在浅蓝灰页面上。成功、警告、失败使用语义颜色，并配合文字及图标。项目状态标签通过 `statusTone.js` 映射，`UiProjectProgress` 根据当前角色可见的状态显示协作阶段。正文采用 14px／22px，关键标签至少 14px，辅助编号为 12px。手机控件触控高度至少 44px。

```vue
<script setup>
import { ref } from 'vue'
import { UiButton, UiCard, UiField } from './ui/index.js'

const keyword = ref('')
</script>

<template>
  <UiCard title="产品筛选" muted>
    <UiField label="产品关键词" v-slot="{ control }">
      <input v-bind="control" v-model="keyword" />
    </UiField>
    <template #footer>
      <UiButton @click="keyword = ''">清空</UiButton>
    </template>
  </UiCard>
</template>
```
