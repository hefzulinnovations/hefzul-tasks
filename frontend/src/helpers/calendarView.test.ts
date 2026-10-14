import {describe, expect, it} from 'vitest'

import {
	isAllDay,
	layoutDay,
	onDay,
	moveChanges,
	slotTime,
	stepAnchor,
	taskSpan,
	visibleDays,
} from './calendarView'

const local = (y: number, m: number, d: number, h = 0, min = 0) => new Date(y, m - 1, d, h, min)
const iso = (date: Date) => date.toISOString()

describe('calendar view', () => {
	it('shows whole weeks around a month, honouring the week start', () => {
		const days = visibleDays('month', local(2026, 10, 14), 6)
		expect(days[0].getDay()).toBe(6)
		expect(days.length % 7).toBe(0)
		expect(days.some(d => d.getDate() === 1 && d.getMonth() === 9)).toBe(true)
		expect(days.some(d => d.getDate() === 31 && d.getMonth() === 9)).toBe(true)
	})

	it('shows the week of the anchor and a single day', () => {
		const week = visibleDays('week', local(2026, 10, 14), 1)
		expect(week).toHaveLength(7)
		expect(week[0]).toEqual(local(2026, 10, 12))
		expect(visibleDays('day', local(2026, 10, 14, 15), 1)).toEqual([local(2026, 10, 14)])
	})

	it('steps by day, week and month', () => {
		expect(stepAnchor('day', local(2026, 10, 31), 1)).toEqual(local(2026, 11, 1))
		expect(stepAnchor('week', local(2026, 10, 14), -1)).toEqual(local(2026, 10, 7))
		expect(stepAnchor('month', local(2026, 1, 31), 1)).toEqual(local(2026, 2, 1))
	})

	it('treats the zero date as unset and midnight as all day', () => {
		expect(taskSpan({id: 1, due_date: '0001-01-01T00:00:00Z'})).toBeNull()
		expect(isAllDay({id: 1, due_date: iso(local(2026, 10, 14))})).toBe(true)
		expect(isAllDay({id: 1, due_date: iso(local(2026, 10, 14, 9, 30))})).toBe(false)
	})

	it('places overlapping tasks side by side', () => {
		const day = local(2026, 10, 14)
		const placed = layoutDay([
			{id: 1, due_date: iso(local(2026, 10, 14, 9))},
			{id: 2, due_date: iso(local(2026, 10, 14, 9, 15))},
			{id: 3, due_date: iso(local(2026, 10, 14, 11))},
			{id: 4, due_date: iso(local(2026, 10, 15, 9))},
		], day)
		expect(placed.map(p => p.task.id)).toEqual([1, 2, 3])
		expect(placed[0]).toMatchObject({top: 540, column: 0, columns: 2})
		expect(placed[1]).toMatchObject({column: 1, columns: 2})
		expect(placed[2]).toMatchObject({top: 660, column: 0, columns: 1})
	})

	it('keeps a late due date on its own day', () => {
		const lateTask = {id: 1, due_date: iso(local(2026, 10, 14, 23, 59))}
		expect(onDay(lateTask, local(2026, 10, 14))).toBe(true)
		expect(onDay(lateTask, local(2026, 10, 15))).toBe(false)
	})

	it('clips a multi-day span to the day', () => {
		const placed = layoutDay([
			{id: 1, start_date: iso(local(2026, 10, 13, 22)), end_date: iso(local(2026, 10, 14, 2))},
		], local(2026, 10, 14))
		expect(placed[0]).toMatchObject({top: 0, height: 120})
	})

	it('snaps drops to 30 minute slots', () => {
		expect(slotTime(local(2026, 10, 14), 9 * 60 + 44)).toEqual(local(2026, 10, 14, 9, 30))
		expect(slotTime(local(2026, 10, 14), 24 * 60 + 5)).toEqual(local(2026, 10, 14, 23, 30))
	})

	it('moves the due date, or the whole span keeping its length', () => {
		const target = local(2026, 10, 20, 14)
		expect(moveChanges({id: 1, due_date: iso(local(2026, 10, 14, 9))}, target, false))
			.toEqual({due_date: iso(target)})
		expect(moveChanges({id: 1, due_date: iso(local(2026, 10, 14, 9, 45))}, local(2026, 10, 20), true))
			.toEqual({due_date: iso(local(2026, 10, 20, 9, 45))})
		expect(moveChanges({
			id: 1,
			start_date: iso(local(2026, 10, 14, 9)),
			end_date: iso(local(2026, 10, 14, 11)),
		}, target, false)).toEqual({
			start_date: iso(target),
			end_date: iso(local(2026, 10, 20, 16)),
		})
	})
})
