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

使用 `theme.css` 中的 `--ui-*` 变量定义颜色、文字、间距和圆角。主题只用黑、白和灰阶。黑色实底用于主要操作，卡片保留白底和浅灰边框。用文字及图标区分待处理、完成、失败等业务状态。

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
