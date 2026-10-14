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

	it('keeps stored queries this session did not load', async () => {
		vi.useFakeTimers()
		const {persistQueryCache} = await freshModule()
		const first = new QueryClient()
		const stopFirst = persistQueryCache(first)
		first.setQueryData(['habits', 'list', false], ['Read'])
		first.setQueryData(['tasks', 1], {id: 1, title: 'Old title'})
		await vi.advanceTimersByTimeAsync(1000)
		stopFirst()

		// The next start is online: nothing restored, only the task is loaded again.
		const second = new QueryClient()
		const stopSecond = persistQueryCache(second)
		second.setQueryData(['tasks', 1], {id: 1, title: 'New title'})
		await vi.advanceTimersByTimeAsync(1000)
		stopSecond()
		vi.useRealTimers()

		const {restoreQueryCache} = await freshModule()
		const restored = new QueryClient()
		await restoreQueryCache(restored)
		expect(restored.getQueryData(['tasks', 1])).toEqual({id: 1, title: 'New title'})
		expect(restored.getQueryData(['habits', 'list', false])).toEqual(['Read'])
	})
})
