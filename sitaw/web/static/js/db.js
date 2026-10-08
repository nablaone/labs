// Minimal IndexedDB key-value wrapper. Stores:
//   kv     - session, last snapshot metadata, settings
//   items  - local replica of shared items (keyed by id)
//   outbox - pending item writes made while offline (keyed by id, newest wins)

const DB_NAME = 'sitaw';
const STORES = ['kv', 'items', 'outbox'];
let dbp;

function open() {
  dbp ??= new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1);
    req.onupgradeneeded = () => {
      for (const s of STORES) {
        if (!req.result.objectStoreNames.contains(s)) req.result.createObjectStore(s);
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
  return dbp;
}

// Runs fn in a transaction and resolves, once it commits, with the result of
// the IDBRequest that fn returns (undefined if fn returns nothing).
async function tx(store, mode, fn) {
  const db = await open();
  return new Promise((resolve, reject) => {
    const t = db.transaction(store, mode);
    const req = fn(t.objectStore(store));
    t.oncomplete = () => resolve(req ? req.result : undefined);
    t.onerror = () => reject(t.error);
    t.onabort = () => reject(t.error);
  });
}

export const get = (store, key) => tx(store, 'readonly', (s) => s.get(key));
export const put = (store, key, val) => tx(store, 'readwrite', (s) => { s.put(val, key); });
export const del = (store, key) => tx(store, 'readwrite', (s) => { s.delete(key); });
export const all = (store) => tx(store, 'readonly', (s) => s.getAll());
export const clear = (store) => tx(store, 'readwrite', (s) => { s.clear(); });

export async function wipe() {
  for (const s of STORES) await clear(s);
}
