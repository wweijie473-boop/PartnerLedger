import { mkdirSync } from 'node:fs'
import { dirname } from 'node:path'
import { DatabaseSync } from 'node:sqlite'

export function openRetailDatabase(filename) {
  if (filename !== ':memory:') mkdirSync(dirname(filename), { recursive: true })
  const db = new DatabaseSync(filename)
  db.exec(`PRAGMA busy_timeout = 5000;
    PRAGMA foreign_keys = ON;
    CREATE TABLE IF NOT EXISTS demo_sessions (
      code TEXT PRIMARY KEY NOT NULL, dealer_key TEXT UNIQUE NOT NULL, vendor_key TEXT UNIQUE NOT NULL,
      promotion_active INTEGER NOT NULL, state TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS demo_checkout_links (
      token TEXT PRIMARY KEY NOT NULL, session_code TEXT NOT NULL REFERENCES demo_sessions(code), order_id TEXT NOT NULL
    );`)
  const byActor = db.prepare('SELECT state FROM demo_sessions WHERE dealer_key = ? OR vendor_key = ?')
  const byPromotion = db.prepare('SELECT state FROM demo_sessions WHERE code = ? AND promotion_active = 1')
  const byCheckout = db.prepare(`SELECT state, order_id FROM demo_checkout_links
    JOIN demo_sessions ON code = session_code WHERE token = ?`)
  const save = db.prepare(`INSERT INTO demo_sessions (code, dealer_key, vendor_key, promotion_active, state)
    VALUES (?, ?, ?, ?, ?) ON CONFLICT(code) DO UPDATE SET promotion_active = excluded.promotion_active, state = excluded.state`)
  const addCheckout = db.prepare('INSERT INTO demo_checkout_links (token, session_code, order_id) VALUES (?, ?, ?)')
  const sessionOf = row => row && JSON.parse(row.state)
  return {
    actor(key) {
      const session = sessionOf(byActor.get(key, key))
      return session && { session, role: session.dealerKey === key ? 'dealer' : 'vendor' }
    },
    promotion: code => sessionOf(byPromotion.get(code)),
    checkout(key) {
      const entry = byCheckout.get(key)
      if (!entry) return undefined
      const session = sessionOf(entry)
      return { session, row: session.orders.find(row => row.id === entry.order_id) }
    },
    save(session) {
      save.run(session.code, session.dealerKey, session.vendorKey, Number(session.promotionActive), JSON.stringify(session))
    },
    addCheckout: (key, session, row) => addCheckout.run(key, session.code, row.id),
    write(operation) {
      db.exec('BEGIN IMMEDIATE')
      try {
        const result = operation()
        db.exec('COMMIT')
        return result
      } catch (error) {
        db.exec('ROLLBACK')
        throw error
      }
    },
    close: () => db.close(),
  }
}
