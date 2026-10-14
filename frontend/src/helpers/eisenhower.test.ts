import {describe, expect, it} from 'vitest'

import {
	QUADRANTS,
	URGENT_WITHIN_DAYS,
	getQuadrant,
	isImportant,
	isUrgent,
	quadrantChanges,
	type MatrixTask,
	type Quadrant,
} from './eisenhower'

const NOW = new Date('2026-10-14T10:00:00Z')
const DAY_MS = 24 * 60 * 60 * 1000
const ZERO_DATE = '0001-01-01T00:00:00Z'

function inDays(days: number): string {
	return new Date(NOW.getTime() + days * DAY_MS).toISOString()
}

function apply(task: MatrixTask, changes: ReturnType<typeof quadrantChanges>): MatrixTask {
	return {...task, ...changes}
}

describe('eisenhower matrix', () => {
	it('treats priority 3 and above as important', () => {
		expect(isImportant({priority: 0})).toBe(false)
		expect(isImportant({priority: 2})).toBe(false)
		expect(isImportant({priority: 3})).toBe(true)
		expect(isImportant({priority: 5})).toBe(true)
		expect(isImportant({})).toBe(false)
	})

	it('treats overdue and soon-due tasks as urgent', () => {
		expect(isUrgent({due_date: inDays(-3)}, NOW)).toBe(true)
		expect(isUrgent({due_date: inDays(1)}, NOW)).toBe(true)
		expect(isUrgent({due_date: inDays(URGENT_WITHIN_DAYS)}, NOW)).toBe(true)
		expect(isUrgent({due_date: new Date(NOW.getTime() + URGENT_WITHIN_DAYS * DAY_MS + 1).toISOString()}, NOW)).toBe(false)
		expect(isUrgent({due_date: inDays(10)}, NOW)).toBe(false)
	})

	it('treats a missing or zero due date as not urgent', () => {
		expect(isUrgent({}, NOW)).toBe(false)
		expect(isUrgent({due_date: null}, NOW)).toBe(false)
		expect(isUrgent({due_date: ''}, NOW)).toBe(false)
		expect(isUrgent({due_date: ZERO_DATE}, NOW)).toBe(false)
		expect(isUrgent({due_date: 'not a date'}, NOW)).toBe(false)
	})

	it('assigns the four quadrants', () => {
		expect(getQuadrant({priority: 4, due_date: inDays(1)}, NOW)).toBe('do')
		expect(getQuadrant({priority: 4, due_date: inDays(20)}, NOW)).toBe('schedule')
		expect(getQuadrant({priority: 4}, NOW)).toBe('schedule')
		expect(getQuadrant({priority: 1, due_date: inDays(1)}, NOW)).toBe('delegate')
		expect(getQuadrant({priority: 0, due_date: ZERO_DATE}, NOW)).toBe('eliminate')
	})

	it('returns no changes when the task is already in the target quadrant', () => {
		expect(quadrantChanges({priority: 4, due_date: inDays(1)}, 'do', NOW)).toEqual({})
		expect(quadrantChanges({priority: 0}, 'eliminate', NOW)).toEqual({})
	})

	it('only changes the field that differs', () => {
		const importantNotUrgent = {priority: 4, due_date: inDays(20)}
		expect(Object.keys(quadrantChanges(importantNotUrgent, 'do', NOW))).toEqual(['due_date'])

		const urgentNotImportant = {priority: 1, due_date: inDays(1)}
		expect(Object.keys(quadrantChanges(urgentNotImportant, 'do', NOW))).toEqual(['priority'])
	})

	it('lands every task in the target quadrant, and a second move is a no-op', () => {
		const starts: MatrixTask[] = [
			{priority: 5, due_date: inDays(-1)},
			{priority: 3, due_date: inDays(30)},
			{priority: 0, due_date: inDays(1)},
			{priority: 0, due_date: ZERO_DATE},
			{},
		]
		for (const start of starts) {
			for (const target of QUADRANTS as readonly Quadrant[]) {
				const moved = apply(start, quadrantChanges(start, target, NOW))
				expect(getQuadrant(moved, NOW)).toBe(target)
				expect(quadrantChanges(moved, target, NOW)).toEqual({})
			}
		}
	})

	it('demotes to medium priority rather than clearing it', () => {
		expect(quadrantChanges({priority: 5, due_date: inDays(20)}, 'eliminate', NOW).priority).toBe(2)
	})
})
