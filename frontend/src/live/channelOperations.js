import { readRetailCatalog, retailApi } from './retailRequest.js'

export const operations = retailApi + '/operations'

const total = (items, field) => items.reduce((sum, row) => sum + BigInt(row[field]), 0n)

export async function readChannelOperations(read, view, role) {
  const catalog = name => readRetailCatalog(read, operations + '/' + name)
  const orders = () => readRetailCatalog(read, retailApi + '/orders', 'before')
  if (view === 'business-overview') return { totals: await read(operations + '/overview') }
  if (view === 'customers' || view === 'renewals') return { [view]: await catalog(view) }

  const dealer = role === 'dealer'
  const partnerField = dealer ? 'manufacturer_id' : 'reseller_id'
  if (view === 'service-tickets' || view === 'vendor-tickets') {
    const [tickets, partners, orderRows] = await Promise.all([catalog('tickets'), catalog('partners'), dealer ? orders() : []])
    const names = new Map(partners.map(row => [row.id, row.name]))
    return { partners, orders: orderRows, tickets: tickets.reverse().map(row => ({ ...row, partner: names.get(row[partnerField]) })) }
  }

  const needsOrders = ['funds-transactions', 'funds-invoices', 'vendor-finance'].includes(view)
  const [requests, partners, orderRows, commission] = await Promise.all([
    catalog('finance'), catalog('partners'), needsOrders ? orders() : [],
    view === 'funds-overview' ? read(retailApi + '/orders?view=commission') : null,
  ])
  const names = new Map(partners.map(row => [row.id, row.name]))
  const [topups, invoices, claims] = ['topups', 'invoices', 'claims'].map(kind => requests
    .filter(row => row.kind === kind).map(row => ({ ...row, partner: names.get(row[partnerField]) })))
  const payouts = orderRows.filter(row => row.payout_ref && row.commission?.settled_at).map(row => ({
    order_id: row.id, amount_cents: row.commission.cents, reference: row.payout_ref, handled_at: row.commission.settled_at,
    partner: row.manufacturer,
  }))
  const available = total(partners, 'limit_cents') - total(partners, 'pending_cents') - total(partners, 'paid_cents')
  return {
    partners, orders: orderRows, topups, invoices, claims, payouts,
    totals: {
      topup_balance_cents: total(topups.filter(row => row.status === 'credited'), 'amount_cents'),
      pending_commission_cents: commission?.summary.settlement_cents ?? '0',
      settled_commission_cents: commission?.summary.settled_cents ?? '0',
      service_credit_cents: total(partners, 'limit_cents'),
      claimed_service_cents: total(partners, 'pending_cents'),
      paid_service_cents: total(partners, 'paid_cents'),
      available_service_cents: available,
    },
  }
}
