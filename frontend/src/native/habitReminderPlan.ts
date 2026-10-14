// Plans the local habit reminders for the Android app. Web Push does not
// reach a WebView, so the app schedules the next days' reminders itself and
// replans whenever the habit list changes.
//
// No framework imports, so it can be unit-tested in isolation.

export interface ReminderHabit {
	id: number
	title: string
	schedule_kind: 'daily' | 'weekdays' | 'times_per_week'
	weekdays: number
	reminder_time: string
	archived: boolean
	done_today: boolean
	due_today: boolean
}

export interface PlannedReminder {
	id: number
	habitId: number
	title: string
	at: Date
}

export const HABIT_REMINDER_ID_BASE = 2_000_000
export const HABIT_REMINDER_DAYS = 7

/** A stable notification id per habit and day offset. */
export function habitReminderId(habitId: number, dayOffset: number): number {
	return HABIT_REMINDER_ID_BASE + habitId * HABIT_REMINDER_DAYS + dayOffset
}

function parseTime(hhmm: string): [number, number] | null {
	const match = /^(\d{2}):(\d{2})$/.exec(hhmm)
	if (!match) return null
	const hours = Number(match[1])
	const minutes = Number(match[2])
	if (hours > 23 || minutes > 59) return null
	return [hours, minutes]
}

function dueOn(habit: ReminderHabit, day: Date): boolean {
	if (habit.schedule_kind === 'weekdays') {
		// Bit 0 is Sunday, matching Date.getDay().
		return (habit.weekdays & (1 << day.getDay())) !== 0
	}
	// Daily habits are due every day. A times-per-week habit may be done on
	// any day, so it is reminded daily until the server says otherwise today.
	return true
}

/**
 * The reminders for the next days, in device local time. Today counts only
 * when the habit is due and not done yet, and only for a time still ahead.
 */
export function planHabitReminders(habits: ReminderHabit[], now: Date, days = HABIT_REMINDER_DAYS): PlannedReminder[] {
	const planned: PlannedReminder[] = []
	for (const habit of habits) {
		if (habit.archived) continue
		const time = parseTime(habit.reminder_time)
		if (!time) continue

		for (let offset = 0; offset < days; offset++) {
			const at = new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset, time[0], time[1], 0, 0)
			if (at.getTime() <= now.getTime()) continue
			if (offset === 0 ? (!habit.due_today || habit.done_today) : !dueOn(habit, at)) continue
			planned.push({id: habitReminderId(habit.id, offset), habitId: habit.id, title: habit.title, at})
		}
	}
	return planned
}
