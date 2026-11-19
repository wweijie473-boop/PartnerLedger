export function statusTone(stage) {
  if (['completed', 'settled', 'won', 'confirmed', 'ordered'].includes(stage)) return 'success'
  if (['customer_info', 'vendor_info', 'channel_revision', 'sales_revision', 'rectification', 'expired', 'acceptance'].includes(stage)) return 'warning'
  if (['inquired', 'channel_quoted', 'sales_quoted', 'supplied', 'shipped', 'channel_wait', 'introduced', 'active'].includes(stage)) return 'info'
  return 'neutral'
}
