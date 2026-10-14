// Eisenhower Matrix logic: maps a task onto one of four quadrants and computes
// the field changes needed to move it to another quadrant.
//
// Vikunja has no "urgent"/"important" fields, so both are derived from fields
// every task already has:
//   - important = priority is HIGH (3) or above
//   - urgent    = due date is overdue or falls within the next URGENT_WITHIN_DAYS days
//
// This file intentionally has no framework or path-alias imports so it can be
// unit-tested in isolation.

export type Quadrant = 'do' | 'schedule' | 'delegate' | 'eliminate'

export const QUADRANTS: readonly Quadrant[] = ['do', 'schedule', 'delegate', 'eliminate']

// Matches PRIORITIES.HIGH and PRIORITIES.MEDIUM in constants/priorities.ts.
export const IMPORTANT_MIN_PRIORITY = 3
export const DEMOTED_PRIORITY = 2

export const URGENT_WITHIN_DAYS = 2
// When a task is dragged into a "not urgent" quadrant, its due date moves this far out.
export const NOT_URGENT_DUE_AFTER_DAYS = 7

const DAY_MS = 24 * 60 * 60 * 1000

export interface MatrixTask {
	priority?: number | null
	due_date?: string | null
}

export interface MatrixChanges {
	priority?: number
	due_date?: string
}

/** The API stores "no due date" as the year-1 zero date, so anything before 1970 counts as unset. */
function parseDueDate(due: string | null | undefined): Date | null {
	if (!due) return null
	const date = new Date(due)
	if (Number.isNaN(date.getTime()) || date.getFullYear() < 1970) return null
	return date
}

export function isImportant(task: MatrixTask): boolean {
	return (task.priority ?? 0) >= IMPORTANT_MIN_PRIORITY
}

export function isUrgent(task: MatrixTask, now: Date = new Date()): boolean {
	const due = parseDueDate(task.due_date)
	if (due === null) return false
	return due.getTime() <= now.getTime() + URGENT_WITHIN_DAYS * DAY_MS
}

export function getQuadrant(task: MatrixTask, now: Date = new Date()): Quadrant {
	const important = isImportant(task)
	const urgent = isUrgent(task, now)
	if (important && urgent) return 'do'
	if (important) return 'schedule'
	if (urgent) return 'delegate'
	return 'eliminate'
}

function endOfDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 0, 0)
}

/**
 * Returns only the fields that must change for the task to land in the target
 * quadrant. An empty object means the task is already there.
 *
 * Moving into an urgent quadrant sets the due date to the end of today; moving
 * into a not-urgent one pushes it NOT_URGENT_DUE_AFTER_DAYS out. Both overwrite
 * an existing due date, which is what makes a drag change the quadrant.
 */
export function quadrantChanges(task: MatrixTask, target: Quadrant, now: Date = new Date()): MatrixChanges {
	const changes: MatrixChanges = {}
	const wantImportant = target === 'do' || target === 'schedule'
	const wantUrgent = target === 'do' || target === 'delegate'

	if (wantImportant !== isImportant(task)) {
		changes.priority = wantImportant ? IMPORTANT_MIN_PRIORITY : DEMOTED_PRIORITY
	}

	if (wantUrgent !== isUrgent(task, now)) {
		const due = wantUrgent
			? endOfDay(now)
			: endOfDay(new Date(now.getTime() + NOT_URGENT_DUE_AFTER_DAYS * DAY_MS))
		changes.due_date = due.toISOString()
	}

	return changes
}
