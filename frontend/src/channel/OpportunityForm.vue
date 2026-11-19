<script setup>
import { computed, reactive, ref } from 'vue'
import { UiButton } from '../ui/index.js'
import { suggestOpportunity } from './opportunities.js'
import OpportunityFields from './OpportunityFields.vue'
const props = defineProps({mode:String,item:Object,partner:Object,projects:Array,projectId:String,busy:Boolean,error:String})
const emit = defineEmits(['save','cancel'])
const isCapture = props.mode === 'capture'
const isClarify = props.mode === 'clarify'
const form = reactive({
  projectId: props.projectId || '', note:'', category:isClarify ? props.item?.category || '' : '',
  source:isClarify ? props.item?.source || '' : '', target:isClarify ? props.item?.target || '' : '',
  name:'',owner:'',evidence:'',followUp:'',feeTerms:'',feeAmount:'',outcome:'', consentCustomer:false,consentPartner:false,
})
const extracted = ref(false)
const confirmed = ref(false)
const remember = ref(false)
const rawNote = ref(isCapture ? props.projects?.find(project => project.id === props.projectId)?.inquiry?.gapNote || '' : '')
const suggestion = computed(() => suggestOpportunity(rawNote.value))
function review() {
  const result = suggestion.value
  Object.assign(form,{note:result.suggestedNote, category:result.category,source:result.source,target:result.target})
  extracted.value = true; confirmed.value = false
}
const buttons = {capture:'确认保存商机',partner:'保存伙伴能力',clarify:'保存澄清结果',introduce:'登记已获同意的引荐',follow:'保存跟进日期',win:'确认合作成功',lose:'关闭此条商机',settle:'确认已结清'}
function submit(){
  if(props.busy) return
  if(isCapture){
    if(!extracted.value || !confirmed.value) return
    emit('save',{projectId:form.projectId, note:form.note.trim(),category:form.category,source:form.source,target:form.target})
  } else if(isClarify) emit('save',{category:form.category,source:form.source,target:form.target})
  else if(props.mode === 'partner') emit('save',{name:form.name,owner:form.owner,category:form.category,source:form.source,target:form.target,evidence:form.evidence})
  else if(props.mode === 'introduce') emit('save',{partnerId:props.partner.partnerId,followUp:form.followUp,feeTerms:form.feeTerms,feeAmount:form.feeAmount,consent:form.consentCustomer && form.consentPartner})
  else if(props.mode === 'settle') emit('save',{})
  else if(props.mode === 'follow') emit('save',{followUp:form.followUp})
  else emit('save',{outcome:form.outcome.trim(),remember:remember.value})
}
</script>
<template>
  <form class="op-form" @submit.prevent="submit">
    <template v-if="isCapture">
      <label>关联项目<select v-model="form.projectId" required><option disabled value="">请选择一笔项目</option><option v-for="record in projects" :key="record.id" :value="record.id">{{ record.title }} · {{ record.id }}</option></select></label>
      <label>走访记录<textarea v-model="rawNote" required rows="3" maxlength="1000" placeholder="例：ERP 已定，客户还需要 IDE 接口，原厂暂不支持。" @input="extracted = false; confirmed = false"></textarea></label>
      <UiButton v-if="!extracted" :disabled="!rawNote.trim() || !form.projectId" @click="review">整理成商机草稿</UiButton>
      <template v-else>
        <label>保留为商机的需求<textarea v-model="form.note" required maxlength="1000" rows="2"></textarea></label>
        <OpportunityFields :form="form" />
        <div v-if="suggestion.questions?.length" class="op-inset"><strong>下次可向客户确认</strong><ul><li v-for="q in suggestion.questions" :key="q">{{ q }}</li></ul></div>
        <label class="op-check"><input v-model="confirmed" type="checkbox" required>我已核对这是一条需要单独跟进的需求。</label>
      </template>
    </template>
    <template v-else-if="isClarify">
      <p class="op-muted">{{ item.note }}</p>
      <OpportunityFields :form="form" require-category />
      <p class="op-muted">信息不足时继续留空。系统只按核对的业务能力推荐伙伴。</p>
    </template>
    <template v-else-if="mode === 'partner'">
      <div class="op-fields"><label>伙伴名称或代称<input v-model="form.name" required maxlength="60" placeholder="例如：北城实施团队"></label>
        <label>团队内谁能引荐<input v-model="form.owner" required maxlength="60" placeholder="例如：王经理"></label></div>
      <OpportunityFields :form="form" require-category />
      <label>合作依据（选填）<textarea v-model="form.evidence" rows="2" maxlength="500" placeholder="已授权的案例简述；没有合作记录就留空。"></textarea></label>
      <p class="op-muted">只记录已获授权的业务能力和团队引荐人，不录入电话或私人通讯录。</p>
    </template>
    <template v-else-if="mode === 'introduce'">
      <div class="op-inset"><strong>{{ partner.name }}</strong><p>通过 {{ partner.owner }} 引荐。请先核实当前版本、排期和交付范围。</p></div>
      <label>下次跟进日期<input v-model="form.followUp" required type="date"></label>
      <div class="op-fields"><label>约定介绍费（元，选填）<input v-model="form.feeAmount" type="text" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,2})?" placeholder="例如 1500.00"></label>
      <label>计费条件<input v-model="form.feeTerms" :required="Number(form.feeAmount) > 0" maxlength="300" placeholder="例如：接口验收后一次结清"></label></div>
      <label class="op-check"><input v-model="form.consentCustomer" type="checkbox" required>客户已同意此次引荐。</label>
      <label class="op-check"><input v-model="form.consentPartner" type="checkbox" required>伙伴已确认可以承接，并同意此次引荐。</label>
    </template>
    <template v-else-if="mode === 'settle'"><p>请与经办人核对已实际收到介绍费 <strong>¥{{ item.feeAmount }}</strong>。这里只登记结果，不执行收付款。</p></template>
    <label v-else-if="mode === 'follow'">下次跟进日期<input v-model="form.followUp" required type="date"></label>
    <template v-else><label>{{ mode === 'win' ? '合作结果' : '关闭原因' }}<textarea v-model="form.outcome" rows="3" required maxlength="500" :placeholder="mode === 'win' ? '记录实际承接的事项。' : '例如：客户决定暂缓该需求。'"></textarea></label>
      <label v-if="mode === 'win'" class="op-check"><input type="checkbox" v-model="remember">已获授权，将此次成交作为团队案例。仅保存商机编号及能力类型。</label>
    </template>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <div class="modal-actions"><UiButton :disabled="busy" @click="$emit('cancel')">取消</UiButton><UiButton v-if="!isCapture || extracted" type="submit" variant="primary" :loading="busy">{{ buttons[mode] }}</UiButton></div>
  </form>
</template>
