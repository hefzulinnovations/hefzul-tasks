import {beforeEach, describe, expect, it, vi} from 'vitest'

const sdk = vi.hoisted(() => ({
	tasksCreate: vi.fn(),
	tasksRead: vi.fn(),
	patchTasksRead: vi.fn(),
	tasksDelete: vi.fn(),
	habitsCheckin: vi.fn(),
	focusSessionsFinish: vi.fn(),
}))
vi.mock('@/client/generated', () => sdk)

async function freshOutbox() {
	vi.resetModules()
	return import('./outbox')
}

const meta = (key: string) => ({key, createdAt: '2026-10-14T09:00:00Z'})

describe('offline outbox', () => {
	beforeEach(() => {
		for (const fn of Object.values(sdk)) fn.mockReset()
	})

	it('replays in order with each entry\'s idempotency key and empties the queue', async () => {
		const outbox = await freshOutbox()
		const calls: string[] = []
		sdk.habitsCheckin.mockImplementation(async ({headers}) => { calls.push(`checkin ${headers['Idempotency-Key']}`); return {data: {}} })
		sdk.focusSessionsFinish.mockImplementation(async ({headers}) => { calls.push(`finish ${headers['Idempotency-Key']}`); return {data: {}} })

		await outbox.enqueue({kind: 'habit.checkin', ...meta('k1'), habitId: 3, date: '2026-10-14'})
		await outbox.enqueue({kind: 'focus.finish', ...meta('k2'), sessionId: 9, status: 'completed'})
		expect(outbox.pendingCount.value).toBe(2)

		await outbox.syncOutbox()

		expect(calls).toEqual(['checkin k1', 'finish k2'])
		expect(sdk.habitsCheckin).toHaveBeenCalledWith(expect.objectContaining({body: {date: '2026-10-14'}}))
		expect(outbox.pendingCount.value).toBe(0)
	})

	it('points later edits of a task created offline at its real id', async () => {
		const outbox = await freshOutbox()
		sdk.tasksCreate.mockResolvedValue({data: {id: 77}})
		sdk.tasksRead.mockResolvedValue({data: {id: 77, title: 'Draft', updated: '2026-10-14T09:00:00Z'}})
		sdk.patchTasksRead.mockResolvedValue({data: {}})

		await outbox.enqueue({kind: 'task.create', ...meta('c'), tempId: -5, projectId: 1, body: {title: 'Draft'}})
		await outbox.enqueue({kind: 'task.update', ...meta('u'), taskId: -5, changes: {title: 'Final'}, base: {title: 'Draft'}, baseUpdated: '2026-10-14T09:00:00Z', title: 'Draft'})
		await outbox.syncOutbox()

		expect(sdk.tasksRead).toHaveBeenCalledWith({path: {task: 77}})
		expect(sdk.patchTasksRead).toHaveBeenCalledWith(expect.objectContaining({
			path: {task: 77},
			body: [{op: 'add', path: '/title', value: 'Final'}],
		}))
	})

	it('holds conflicting fields for the user and sends the rest', async () => {
		const outbox = await freshOutbox()
		sdk.tasksRead.mockResolvedValue({data: {id: 1, title: 'Theirs', done: false, updated: '2026-10-14T10:00:00Z'}})
		sdk.patchTasksRead.mockResolvedValue({data: {}})

		await outbox.enqueue({kind: 'task.update', ...meta('u'), taskId: 1, changes: {title: 'Mine', done: true}, base: {title: 'Old', done: false}, baseUpdated: '2026-10-14T09:00:00Z', title: 'Old'})
		await outbox.syncOutbox()

		expect(sdk.patchTasksRead).toHaveBeenCalledWith(expect.objectContaining({body: [{op: 'add', path: '/done', value: true}]}))
		expect(outbox.outboxConflicts.value).toEqual([{key: 'u', taskId: 1, title: 'Old', fields: [{field: 'title', mine: 'Mine', theirs: 'Theirs'}]}])

		sdk.patchTasksRead.mockClear()
		await outbox.resolveConflict('u', {title: 'mine'})
		expect(sdk.patchTasksRead).toHaveBeenCalledWith(expect.objectContaining({body: [{op: 'add', path: '/title', value: 'Mine'}]}))
		expect(outbox.outboxConflicts.value).toEqual([])
	})

	it('skips an offline delete of a task someone changed since', async () => {
		const outbox = await freshOutbox()
		sdk.tasksRead.mockResolvedValue({data: {id: 1, updated: '2026-10-14T10:00:00Z'}})

		await outbox.enqueue({kind: 'task.delete', ...meta('d'), taskId: 1, baseUpdated: '2026-10-14T09:00:00Z', title: 'Groceries'})
		await outbox.syncOutbox()

		expect(sdk.tasksDelete).not.toHaveBeenCalled()
		expect(outbox.outboxNotices.value).toEqual([{key: 'd', kind: 'delete-skipped', title: 'Groceries'}])
	})

	it('keeps the queue when the network fails or the sign-in expired', async () => {
		const outbox = await freshOutbox()
		sdk.habitsCheckin.mockRejectedValueOnce(new TypeError('Failed to fetch'))
		await outbox.enqueue({kind: 'habit.checkin', ...meta('k'), habitId: 3, date: '2026-10-14'})

		await outbox.syncOutbox()
		expect(outbox.pendingCount.value).toBe(1)

		sdk.habitsCheckin.mockRejectedValueOnce(Object.assign(new Error('expired'), {status: 401}))
		await outbox.syncOutbox()
		expect(outbox.pendingCount.value).toBe(1)
		expect(outbox.outboxNeedsSignIn.value).toBe(true)

		sdk.habitsCheckin.mockResolvedValueOnce({data: {}})
		await outbox.syncOutbox()
		expect(outbox.pendingCount.value).toBe(0)
		expect(outbox.outboxNeedsSignIn.value).toBe(false)
	})

	it('drops a write the server rejects and reports it', async () => {
		const outbox = await freshOutbox()
		sdk.habitsCheckin.mockRejectedValueOnce(Object.assign(new Error('bad date'), {status: 400}))
		await outbox.enqueue({kind: 'habit.checkin', ...meta('k'), habitId: 3, date: 'nope'})

		await outbox.syncOutbox()

		expect(outbox.pendingCount.value).toBe(0)
		expect(outbox.outboxNotices.value[0]).toMatchObject({key: 'k', kind: 'rejected'})
	})
})
