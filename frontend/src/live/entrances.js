import { entrances as portalEntrances } from '../entrances.js'

export const entrances = Object.fromEntries(portalEntrances.map(entry => [entry.role, entry]))
export const organizationKinds = Object.freeze({ dealer: 'RESELLER', vendor: 'MANUFACTURER', customer: 'CUSTOMER' })

export function organizationRole(profile, organization) {
  if (!profile?.username || typeof profile.id !== 'string' || !/^[1-9]\d*$/.test(profile.id) || !organization?.id || !organization.name ||
      profile.organization?.id !== organization.id || profile.organization?.kind !== organization.kind) return null
  return Object.keys(organizationKinds).find(role => organizationKinds[role] === organization.kind) || null
}
