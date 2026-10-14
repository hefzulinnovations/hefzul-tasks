// Keeps the data the app needs offline (FR-33) in IndexedDB: projects, tasks,
// habits, countdowns, focus state, labels and the signed-in account. The cache
// is restored before the app mounts and saved a second after it changes.

import {dehydrate, hydrate, type QueryClient} from '@tanstack/vue-query'
import {kvDelete, kvGet, kvSet} from './kv'

const STORAGE_KEY = 'query-cache'
// Bump when cached shapes change incompatibly; an old cache is then dropped.
const CACHE_VERSION = 1
const MAX_AGE_MS = 14 * 24 * 60 * 60 * 1000
const PERSISTED_ROOTS = new Set(['tasks', 'projects', 'habits', 'countdowns', 'focus', 'labels', 'account', 'kanban'])

interface StoredCache {
	version: number
	savedAt: number
	state: ReturnType<typeof dehydrate>
}

let restoring: Promise<void> | null = null

/**
 * Puts the cache from disk into the query client, once. Every caller gets the
 * same promise, so whoever needs the cached data first starts the restore.
 */
export function restoreQueryCache(client: QueryClient): Promise<void> {
	if (!restoring) {
		restoring = (async () => {
			try {
				const stored = await kvGet<StoredCache>(STORAGE_KEY)
				if (!stored || stored.version !== CACHE_VERSION || Date.now() - stored.savedAt > MAX_AGE_MS) return
				hydrate(client, stored.state)
			} catch {
				// A broken cache only costs a refetch.
			}
		})()
	}
	return restoring
}

export function persistQueryCache(client: QueryClient): () => void {
	let timer: ReturnType<typeof setTimeout> | null = null
	const save = () => {
		timer = null
		const state = dehydrate(client, {
			// A query that failed offline still holds its last good data; keep that.
			shouldDehydrateQuery: query => query.state.data !== undefined
				&& PERSISTED_ROOTS.has(String(query.queryKey[0])),
		})
		const clean = {
			...state,
			queries: state.queries.map(query => ({
				...query,
				// Invalidated, so a restored query shows at once but still refetches
				// when online instead of counting as fresh for the rest of its staleTime.
				state: {...query.state, status: 'success' as const, error: null, fetchStatus: 'idle' as const, isInvalidated: true},
			})),
		}
		kvSet(STORAGE_KEY, JSON.parse(JSON.stringify({version: CACHE_VERSION, savedAt: Date.now(), state: clean}))).catch(() => undefined)
	}
	const unsubscribe = client.getQueryCache().subscribe(event => {
		if (event.type !== 'updated' && event.type !== 'removed') return
		if (timer === null) timer = setTimeout(save, 1000)
	})
	return () => {
		unsubscribe()
		if (timer !== null) clearTimeout(timer)
	}
}

export async function clearQueryCache(): Promise<void> {
	await kvDelete(STORAGE_KEY)
}
