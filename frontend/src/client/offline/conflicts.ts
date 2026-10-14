// Per-field conflict resolution for task edits made offline (FR-36).
// No framework imports, so it can be unit-tested in isolation.

export interface FieldConflict {
	field: string
	mine: unknown
	theirs: unknown
}

export interface UpdateResolution {
	/** Fields to send: edited offline and not changed on the server meanwhile. */
	apply: Record<string, unknown>
	/** Fields both sides changed to different values; the user keeps one. */
	conflicts: FieldConflict[]
}

const DATE_FIELDS = new Set(['due_date', 'start_date', 'end_date', 'done_at', 'updated'])

function normalize(field: string, value: unknown): unknown {
	if (value === undefined || value === null || value === '') return null
	if (DATE_FIELDS.has(field) && typeof value === 'string') {
		const ms = new Date(value).getTime()
		if (Number.isNaN(ms)) return value
		// The API's year-1 zero date means "no date".
		return new Date(ms).getUTCFullYear() < 1970 ? null : ms
	}
	if (typeof value === 'object') return JSON.stringify(value)
	return value
}

export function sameValue(field: string, a: unknown, b: unknown): boolean {
	return normalize(field, a) === normalize(field, b)
}

/**
 * Last write wins per field, using the task's updated time: a field the
 * server did not change since the offline edit takes the offline value; a
 * field both sides changed to different values is a conflict.
 */
export function resolveTaskUpdate(
	changes: Record<string, unknown>,
	base: Record<string, unknown>,
	baseUpdated: string,
	server: Record<string, unknown>,
): UpdateResolution {
	const apply: Record<string, unknown> = {}
	const conflicts: FieldConflict[] = []
	const serverChanged = !sameValue('updated', server.updated, baseUpdated)
	for (const [field, mine] of Object.entries(changes)) {
		const theirs = server[field]
		if (!serverChanged || sameValue(field, theirs, base[field])) {
			apply[field] = mine
		} else if (!sameValue(field, theirs, mine)) {
			conflicts.push({field, mine, theirs})
		}
	}
	return {apply, conflicts}
}

/** An offline delete goes through only if nobody changed the task since. */
export function deleteStillApplies(baseUpdated: string, server: {updated?: unknown}): boolean {
	return sameValue('updated', server.updated, baseUpdated)
}
