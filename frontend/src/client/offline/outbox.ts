// The offline outbox (FR-34 to FR-37). Writes made without a connection are
// queued here, persisted in IndexedDB, and replayed in order once the server
// is reachable. Every queued write carries an Idempotency-Key, so a replay
// after a lost response is never applied twice.

import {computed, ref, watch} from 'vue'
import type {QueryClient} from '@tanstack/vue-query'

import {
	focusSessionsFinish,
	habitsCheckin,
	patchTasksRead,
	tasksCreate,
	tasksDelete,
	tasksRead,
} from '@/client/generated'
import type {Task, TaskWritable} from '@/client/generated'
import {kvGet, kvSet} from './kv'
import {isNetworkError, newIdempotencyKey, online} from './network'
import {deleteStillApplies, resolveTaskUpdate, type FieldConflict} from './conflicts'

export type OutboxEntry =
	| {kind: 'task.create', key: string, createdAt: string, tempId: number, projectId: number, body: TaskWritable}
	| {kind: 'task.update', key: string, createdAt: string, taskId: number, changes: Record<string, unknown>, base: Record<string, unknown>, baseUpdated: string, title: string}
	| {kind: 'task.delete', key: string, createdAt: string, taskId: number, baseUpdated: string, title: string}
	| {kind: 'habit.checkin', key: string, createdAt: string, habitId: number, date: string}
	| {kind: 'focus.finish', key: string, createdAt: string, sessionId: number, status: 'completed' | 'abandoned'}

export interface OutboxConflict {
	key: string
	taskId: number
	title: string
	fields: FieldConflict[]
}

export interface OutboxNotice {
	key: string
	kind: 'delete-skipped' | 'rejected'
	title: string
	detail?: string
}

const STORAGE_KEY = 'outbox'

const entries = ref<OutboxEntry[]>([])
const conflicts = ref<OutboxConflict[]>([])
const notices = ref<OutboxNotice[]>([])
const needsSignIn = ref(false)
const syncing = ref(false)
let loaded: Promise<void> | null = null

export const pendingCount = computed(() => entries.value.length)
export const outboxConflicts = computed(() => conflicts.value)
export const outboxNotices = computed(() => notices.value)
export const outboxNeedsSignIn = computed(() => needsSignIn.value)
export const outboxSyncing = computed(() => syncing.value)

interface StoredOutbox {
	entries: OutboxEntry[]
	conflicts: OutboxConflict[]
}

function persist() {
	return kvSet(STORAGE_KEY, JSON.parse(JSON.stringify({entries: entries.value, conflicts: conflicts.value})))
}

export function loadOutbox(): Promise<void> {
	if (!loaded) {
		loaded = kvGet<StoredOutbox>(STORAGE_KEY).then(stored => {
			entries.value = [...(stored?.entries ?? []), ...entries.value]
			conflicts.value = stored?.conflicts ?? []
		})
	}
	return loaded
}

export async function enqueue(entry: OutboxEntry) {
	await loadOutbox()
	entries.value = [...entries.value, entry]
	await persist()
}

export function newEntryMeta() {
	return {key: newIdempotencyKey(), createdAt: new Date().toISOString()}
}

let nextTempId = -Date.now()
/** Ids for tasks created offline; negative so they never clash with real ones. */
export function newTempId(): number {
	nextTempId -= 1
	return nextTempId
}

function idempotency(key: string) {
	return {headers: {'Idempotency-Key': key}}
}

function status(cause: unknown): number | undefined {
	return typeof cause === 'object' && cause !== null && 'status' in cause ? Number((cause as {status: unknown}).status) : undefined
}

/** Rewrites later entries that point at a task created offline. */
function remapTaskId(tempId: number, realId: number) {
	entries.value = entries.value.map(entry => {
		if ((entry.kind === 'task.update' || entry.kind === 'task.delete') && entry.taskId === tempId) {
			return {...entry, taskId: realId}
		}
		return entry
	})
}

async function replayOne(entry: OutboxEntry): Promise<void> {
	switch (entry.kind) {
		case 'task.create': {
			const {data} = await tasksCreate({path: {project: entry.projectId}, body: entry.body, ...idempotency(entry.key)})
			if (data.id) remapTaskId(entry.tempId, data.id)
			return
		}
		case 'task.update': {
			if (entry.taskId < 0) return // created offline and never synced; nothing to update
			const {data: server} = await tasksRead({path: {task: entry.taskId}})
			const {apply, conflicts: fields} = resolveTaskUpdate(entry.changes, entry.base, entry.baseUpdated, server as Record<string, unknown>)
			if (Object.keys(apply).length > 0) {
				await patchTasksRead({
					path: {task: entry.taskId},
					body: Object.entries(apply).map(([field, value]) => ({op: 'add' as const, path: `/${field}`, value})),
					...idempotency(entry.key),
				})
			}
			if (fields.length > 0) {
				conflicts.value = [...conflicts.value, {key: entry.key, taskId: entry.taskId, title: entry.title, fields}]
			}
			return
		}
		case 'task.delete': {
			if (entry.taskId < 0) return
			const {data: server} = await tasksRead({path: {task: entry.taskId}})
			if (!deleteStillApplies(entry.baseUpdated, server)) {
				notices.value = [...notices.value, {key: entry.key, kind: 'delete-skipped', title: entry.title}]
				return
			}
			await tasksDelete({path: {task: entry.taskId}, ...idempotency(entry.key)})
			return
		}
		case 'habit.checkin':
			await habitsCheckin({path: {id: entry.habitId}, body: {date: entry.date}, ...idempotency(entry.key)})
			return
		case 'focus.finish':
			await focusSessionsFinish({path: {id: entry.sessionId}, body: {status: entry.status}, ...idempotency(entry.key)})
	}
}

function entryTitle(entry: OutboxEntry): string {
	if (entry.kind === 'task.update' || entry.kind === 'task.delete') return entry.title
	if (entry.kind === 'task.create') return entry.body.title ?? ''
	return ''
}

let onSynced: (() => void) | null = null

/**
 * Sends queued writes in order. Stops at the first network failure (to try
 * again later) or when the sign-in expired, which keeps the queue for after
 * the user signs in again (FR-37). A write the server rejects is dropped and
 * reported.
 */
export async function syncOutbox(): Promise<void> {
	await loadOutbox()
	if (syncing.value || entries.value.length === 0) return
	syncing.value = true
	let synced = false
	try {
		while (entries.value.length > 0) {
			const entry = entries.value[0]
			try {
				await replayOne(entry)
			} catch (cause) {
				if (isNetworkError(cause)) return
				if (status(cause) === 401) {
					needsSignIn.value = true
					return
				}
				const code = status(cause)
				// 404 or 409: the thing is gone or already in that state; nothing to keep.
				if (code !== 404 && code !== 409) {
					notices.value = [...notices.value, {
						key: entry.key,
						kind: 'rejected',
						title: entryTitle(entry),
						detail: cause instanceof Error ? cause.message : String(cause),
					}]
				}
			}
			entries.value = entries.value.slice(1)
			synced = true
			await persist()
		}
		needsSignIn.value = false
	} finally {
		syncing.value = false
		if (synced) onSynced?.()
	}
}

/** The user picked a side for every field of a conflict. */
export async function resolveConflict(key: string, keep: Record<string, 'mine' | 'theirs'>) {
	const conflict = conflicts.value.find(item => item.key === key)
	if (!conflict) return
	const mine = conflict.fields.filter(field => keep[field.field] === 'mine')
	if (mine.length > 0) {
		await patchTasksRead({
			path: {task: conflict.taskId},
			body: mine.map(field => ({op: 'add' as const, path: `/${field.field}`, value: field.mine})),
			...idempotency(`${key}-resolved`),
		})
	}
	conflicts.value = conflicts.value.filter(item => item.key !== key)
	await persist()
	onSynced?.()
}

export function dismissNotice(key: string) {
	notices.value = notices.value.filter(notice => notice.key !== key)
}

/** Clears everything, for logout: another user must not replay these. */
export async function clearOutbox() {
	entries.value = []
	conflicts.value = []
	notices.value = []
	needsSignIn.value = false
	await persist()
}

let started = false

/** Starts replaying on reconnect, on start and every 30 s while writes wait. */
export function startOutbox(client: QueryClient, isSignedIn: () => boolean) {
	if (started) return
	started = true
	onSynced = () => { client.invalidateQueries() }
	const trySync = () => {
		if (online.value && isSignedIn()) syncOutbox().catch(() => undefined)
	}
	loadOutbox().then(trySync)
	watch(online, value => { if (value) trySync() })
	watch(isSignedIn, value => { if (value) trySync() })
	setInterval(() => {
		if (entries.value.length > 0) trySync()
	}, 30_000)
}

/** Finds the freshest cached copy of a task, for the base of an offline edit. */
export function cachedTask(client: QueryClient, id: number): Task | undefined {
	for (const [, data] of client.getQueriesData<unknown>({queryKey: ['tasks']})) {
		const found = findTask(data, id)
		if (found) return found
	}
	return undefined
}

function findTask(data: unknown, id: number): Task | undefined {
	if (!data || typeof data !== 'object') return undefined
	if (Array.isArray(data)) {
		for (const item of data) {
			const found = findTask(item, id)
			if (found) return found
		}
		return undefined
	}
	const record = data as Record<string, unknown>
	if (record.id === id && 'title' in record) return record as Task
	if (Array.isArray(record.items)) return findTask(record.items, id)
	if (Array.isArray(record.buckets)) {
		for (const bucket of record.buckets as {tasks?: unknown}[]) {
			const found = findTask(bucket.tasks, id)
			if (found) return found
		}
	}
	return undefined
}
