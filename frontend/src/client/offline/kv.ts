// A tiny IndexedDB key-value store for the offline cache and outbox. Falls
// back to memory where IndexedDB is unavailable (private windows, tests), so
// the app still works, just without surviving a reload.

const DB_NAME = 'hefzul-offline'
const STORE = 'kv'

let dbPromise: Promise<IDBDatabase | null> | null = null
const memory = new Map<string, unknown>()

function openDb(): Promise<IDBDatabase | null> {
	if (dbPromise) return dbPromise
	dbPromise = new Promise(resolve => {
		try {
			if (typeof indexedDB === 'undefined') {
				resolve(null)
				return
			}
			const request = indexedDB.open(DB_NAME, 1)
			request.onupgradeneeded = () => request.result.createObjectStore(STORE)
			request.onsuccess = () => resolve(request.result)
			request.onerror = () => resolve(null)
			request.onblocked = () => resolve(null)
		} catch {
			resolve(null)
		}
	})
	return dbPromise
}

function run<T>(mode: IDBTransactionMode, fn: (store: IDBObjectStore) => IDBRequest<T>): Promise<T | undefined> {
	return openDb().then(db => new Promise(resolve => {
		if (!db) {
			resolve(undefined)
			return
		}
		try {
			const request = fn(db.transaction(STORE, mode).objectStore(STORE))
			request.onsuccess = () => resolve(request.result)
			request.onerror = () => resolve(undefined)
		} catch {
			resolve(undefined)
		}
	}))
}

export async function kvGet<T>(key: string): Promise<T | undefined> {
	const db = await openDb()
	if (!db) return memory.get(key) as T | undefined
	return run<T>('readonly', store => store.get(key) as IDBRequest<T>)
}

export async function kvSet(key: string, value: unknown): Promise<void> {
	const db = await openDb()
	if (!db) {
		memory.set(key, value)
		return
	}
	await run('readwrite', store => store.put(value, key))
}

export async function kvDelete(key: string): Promise<void> {
	const db = await openDb()
	if (!db) {
		memory.delete(key)
		return
	}
	await run('readwrite', store => store.delete(key))
}
