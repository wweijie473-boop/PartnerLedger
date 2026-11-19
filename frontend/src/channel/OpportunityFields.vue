<script setup>
import { categoryLabels, systemLabels } from './opportunities.js'
defineProps({ form: {type:Object, required:true}, requireCategory:Boolean })
</script>
<template>
  <div class="op-fields">
    <label>承接类型<select v-model="form.category" :required="requireCategory"><option value="">待澄清</option><option v-for="(label,key) in categoryLabels" :key="key" :value="key">{{ label }}</option></select></label>
    <label>涉及系统<select v-model="form.source"><option value="">待澄清</option><option v-for="(label,key) in systemLabels" :key="key" :value="key">{{ label }}</option></select></label>
    <label>对接系统<select v-model="form.target"><option value="">未确认或不涉及</option><option v-for="(label,key) in systemLabels" :key="key" :value="key">{{ label }}</option></select></label>
  </div>
</template>
