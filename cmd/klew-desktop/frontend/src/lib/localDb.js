import { openDB } from 'idb'

/**
 * Open (or reuse) a named IndexedDB with a single object store keyed by `id`.
 * @template T
 * @param {string} dbName
 * @param {number} version
 * @param {string} storeName
 */
export function openLocalDb(dbName, version, storeName) {
  return openDB(dbName, version, {
    upgrade(db) {
      if (!db.objectStoreNames.contains(storeName)) {
        db.createObjectStore(storeName, { keyPath: 'id' })
      }
    },
  })
}

/**
 * @template T extends { id: string }
 * @param {string} dbName
 * @param {number} version
 * @param {string} storeName
 * @param {string} id
 * @returns {Promise<T | undefined>}
 */
export async function idbGet(dbName, version, storeName, id) {
  const db = await openLocalDb(dbName, version, storeName)
  return db.get(storeName, id)
}

/**
 * @template T extends { id: string }
 * @param {string} dbName
 * @param {number} version
 * @param {string} storeName
 * @param {T} value
 */
export async function idbPut(dbName, version, storeName, value) {
  const db = await openLocalDb(dbName, version, storeName)
  await db.put(storeName, value)
}
