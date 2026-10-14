import {afterEach, describe, expect, it, vi} from 'vitest'
import {QueryClient} from '@tanstack/vue-query'

const store = new Map<string, unknown>()

vi.mock('./kv', () => ({
	kvGet: vi.fn(async (key: string) => store.get(key)),
	kvSet: vi.fn(async (key: string, value: unknown) => {
		store.set(key, value)
	}),
	kvDelete: vi.fn(async (key: string) => {
		store.delete(key)
	}),
}))

async function freshModule() {
	vi.resetModules()
	return import('./persist')
}

describe('query cache persistence', () => {
	afterEach(() => {
		store.clear()
		vi.useRealTimers()
	})

	it('restores saved data, but as stale so it refetches when online', async () => {
		vi.useFakeTimers()
		const {persistQueryCache} = await freshModule()
		const saving = new QueryClient({defaultOptions: {queries: {staleTime: 60_000}}})
		const stop = persistQueryCache(saving)
		saving.setQueryData(['tasks', 1], {id: 1, title: 'Old title'})
		saving.setQueryData(['unrelated'], 'not persisted')
		await vi.advanceTimersByTimeAsync(1000)
		stop()
		vi.useRealTimers()

		const {restoreQueryCache} = await freshModule()
		const restored = new QueryClient({defaultOptions: {queries: {staleTime: 60_000}}})
		await restoreQueryCache(restored)

		const query = restored.getQueryCache().find({queryKey: ['tasks', 1]})
		expect(query?.state.data).toEqual({id: 1, title: 'Old title'})
		expect(query?.isStaleByTime(60_000)).toBe(true)
		expect(restored.getQueryData(['unrelated'])).toBeUndefined()
	})
})
