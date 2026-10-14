import {describe, expect, it} from 'vitest'

import {habitReminderId, planHabitReminders, type ReminderHabit} from './habitReminderPlan'

function habit(overrides: Partial<ReminderHabit> = {}): ReminderHabit {
	return {
		id: 3,
		title: 'Read',
		schedule_kind: 'daily',
		weekdays: 0,
		reminder_time: '20:30',
		archived: false,
		done_today: false,
		due_today: true,
		...overrides,
	}
}

// Wednesday 14 October 2026, 09:00 local time.
const now = new Date(2026, 9, 14, 9, 0, 0)

describe('planHabitReminders', () => {
	it('plans a daily habit for today and the next six days', () => {
		const planned = planHabitReminders([habit()], now)
		expect(planned).toHaveLength(7)
		expect(planned[0].at).toEqual(new Date(2026, 9, 14, 20, 30))
		expect(planned[6].at).toEqual(new Date(2026, 9, 20, 20, 30))
		expect(planned.map(p => p.id)).toEqual([0, 1, 2, 3, 4, 5, 6].map(d => habitReminderId(3, d)))
	})

	it('skips today when the habit is done or not due', () => {
		expect(planHabitReminders([habit({done_today: true})], now)[0].at).toEqual(new Date(2026, 9, 15, 20, 30))
		expect(planHabitReminders([habit({due_today: false})], now)).toHaveLength(6)
	})

	it('skips today when the time has passed', () => {
		const late = new Date(2026, 9, 14, 21, 0)
		expect(planHabitReminders([habit()], late)[0].at).toEqual(new Date(2026, 9, 15, 20, 30))
	})

	it('only plans the chosen days of a weekdays habit', () => {
		// Monday (2) and Friday (32).
		const planned = planHabitReminders([habit({schedule_kind: 'weekdays', weekdays: 2 | 32, due_today: false})], now)
		expect(planned.map(p => p.at.getDay())).toEqual([5, 1])
	})

	it('ignores archived habits and habits without a valid time', () => {
		expect(planHabitReminders([
			habit({archived: true}),
			habit({reminder_time: ''}),
			habit({reminder_time: '25:00'}),
		], now)).toEqual([])
	})
})
