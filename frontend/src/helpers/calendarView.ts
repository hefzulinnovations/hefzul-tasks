// Calendar view arithmetic: visible ranges, where tasks sit in a time grid and
// how overlapping tasks share a column. Dates are local (browser) time.
// No framework imports, so it can be unit-tested in isolation.

export type CalendarMode = 'day' | 'week' | 'month'

export const SLOT_MINUTES = 30
export const DEFAULT_DURATION_MINUTES = 30
const DAY_MS = 24 * 60 * 60 * 1000

export function startOfDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

export function addDays(date: Date, days: number): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days, date.getHours(), date.getMinutes())
}

export function startOfWeek(date: Date, weekStart: number): Date {
	const day = startOfDay(date)
	return addDays(day, -((day.getDay() - weekStart + 7) % 7))
}

export function sameDay(a: Date, b: Date): boolean {
	return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

/** The days shown for a mode; a month shows whole weeks around it. */
export function visibleDays(mode: CalendarMode, anchor: Date, weekStart: number): Date[] {
	if (mode === 'day') return [startOfDay(anchor)]
	if (mode === 'week') {
		const first = startOfWeek(anchor, weekStart)
		return Array.from({length: 7}, (_, i) => addDays(first, i))
	}
	const firstOfMonth = new Date(anchor.getFullYear(), anchor.getMonth(), 1)
	const lastOfMonth = new Date(anchor.getFullYear(), anchor.getMonth() + 1, 0)
	const first = startOfWeek(firstOfMonth, weekStart)
	const last = addDays(startOfWeek(lastOfMonth, weekStart), 6)
	const days: Date[] = []
	for (let d = first; d <= last; d = addDays(d, 1)) days.push(d)
	return days
}

/** Moves the anchor one page forward (1) or back (-1). */
export function stepAnchor(mode: CalendarMode, anchor: Date, direction: 1 | -1): Date {
	if (mode === 'day') return addDays(anchor, direction)
	if (mode === 'week') return addDays(anchor, 7 * direction)
	return new Date(anchor.getFullYear(), anchor.getMonth() + direction, 1)
}

export interface CalendarTaskLike {
	id: number
	due_date?: string | null
	start_date?: string | null
	end_date?: string | null
}

/** The API stores "no date" as the year-1 zero date, so anything before 1970 counts as unset. */
export function parseTaskDate(value: string | null | undefined): Date | null {
	if (!value) return null
	const date = new Date(value)
	if (Number.isNaN(date.getTime()) || date.getFullYear() < 1970) return null
	return date
}

/** When a task sits in the calendar: its start/end span if set, otherwise its due time. */
export function taskSpan(task: CalendarTaskLike): {start: Date, end: Date} | null {
	const start = parseTaskDate(task.start_date)
	const end = parseTaskDate(task.end_date)
	if (start && end && end > start) return {start, end}
	const due = parseTaskDate(task.due_date)
	if (!due) return null
	return {start: due, end: new Date(due.getTime() + DEFAULT_DURATION_MINUTES * 60_000)}
}

/** A task due at exactly midnight has no time of day; it goes in the all-day row. */
export function isAllDay(task: CalendarTaskLike): boolean {
	const span = taskSpan(task)
	if (!span) return false
	if (parseTaskDate(task.start_date) && parseTaskDate(task.end_date)) {
		return span.end.getTime() - span.start.getTime() >= DAY_MS
	}
	return span.start.getHours() === 0 && span.start.getMinutes() === 0
}

export interface PlacedTask<T> {
	task: T
	/** Minutes from midnight, clipped to the day. */
	top: number
	height: number
	column: number
	columns: number
}

/**
 * Lays out one day's timed tasks: each gets its minutes from midnight and a
 * column, so overlapping tasks sit side by side.
 */
/**
 * Whether a task belongs on the day: a start/end span on every day it covers,
 * a task with only a due date on its due day (even when its default block
 * runs past midnight).
 */
export function onDay(task: CalendarTaskLike, day: Date): boolean {
	const span = taskSpan(task)
	if (!span) return false
	const dayStart = startOfDay(day).getTime()
	const dayEnd = dayStart + DAY_MS
	const start = parseTaskDate(task.start_date)
	const end = parseTaskDate(task.end_date)
	if (start && end && end > start) {
		return span.start.getTime() < dayEnd && span.end.getTime() > dayStart
	}
	return span.start.getTime() >= dayStart && span.start.getTime() < dayEnd
}

export function layoutDay<T extends CalendarTaskLike>(tasks: T[], day: Date): PlacedTask<T>[] {
	const dayStart = startOfDay(day).getTime()
	const items = tasks
		.map(task => ({task, span: taskSpan(task)}))
		.filter((item): item is {task: T, span: {start: Date, end: Date}} =>
			item.span !== null && onDay(item.task, day))
		.map(({task, span}) => {
			const top = Math.max(0, (span.start.getTime() - dayStart) / 60_000)
			const bottom = Math.min(24 * 60, (span.end.getTime() - dayStart) / 60_000)
			return {task, top, height: Math.max(SLOT_MINUTES / 2, bottom - top), column: 0, columns: 1}
		})
		.sort((a, b) => a.top - b.top || b.height - a.height || a.task.id - b.task.id)

	// Group tasks that overlap transitively; inside a group, take the first free column.
	let group: typeof items = []
	let groupEnd = -1
	const flush = () => {
		const columns = Math.max(1, ...group.map(item => item.column + 1))
		for (const item of group) item.columns = columns
		group = []
	}
	for (const item of items) {
		if (group.length > 0 && item.top >= groupEnd) flush()
		const taken = new Set(group.filter(other => other.top + other.height > item.top).map(other => other.column))
		let column = 0
		while (taken.has(column)) column++
		item.column = column
		group.push(item)
		groupEnd = Math.max(groupEnd, item.top + item.height)
	}
	flush()
	return items
}

/** The time a drop at minutesFromMidnight lands on, snapped to the slot grid. */
export function slotTime(day: Date, minutesFromMidnight: number): Date {
	const snapped = Math.min(24 * 60 - SLOT_MINUTES, Math.max(0, Math.floor(minutesFromMidnight / SLOT_MINUTES) * SLOT_MINUTES))
	const d = startOfDay(day)
	d.setMinutes(snapped)
	return d
}

/**
 * The fields to send when a task moves to a new start time. A task with a
 * start/end span keeps its length; otherwise only the due date moves.
 * Moving to a month cell keeps the task's time of day.
 */
export function moveChanges(task: CalendarTaskLike, target: Date, keepTimeOfDay: boolean): Record<string, string> {
	const span = taskSpan(task)
	let start = target
	if (keepTimeOfDay && span) {
		start = new Date(target.getFullYear(), target.getMonth(), target.getDate(), span.start.getHours(), span.start.getMinutes())
	}
	const start_date = parseTaskDate(task.start_date)
	const end_date = parseTaskDate(task.end_date)
	if (start_date && end_date && end_date > start_date) {
		const length = end_date.getTime() - start_date.getTime()
		const changes: Record<string, string> = {
			start_date: start.toISOString(),
			end_date: new Date(start.getTime() + length).toISOString(),
		}
		const due = parseTaskDate(task.due_date)
		if (due) changes.due_date = new Date(due.getTime() + (start.getTime() - start_date.getTime())).toISOString()
		return changes
	}
	return {due_date: start.toISOString()}
}
