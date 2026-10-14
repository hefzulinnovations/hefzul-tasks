import {describe, expect, it} from 'vitest'

import {
	elapsedSeconds,
	formatClock,
	formatDuration,
	nextBreakKind,
	progress,
	remainingSeconds,
	type TimedSession,
} from './focusTimer'

const START = '2026-10-14T09:00:00Z'
const startSeconds = new Date(START).getTime() / 1000

function session(overrides: Partial<TimedSession> = {}): TimedSession {
	return {
		kind: 'focus',
		planned_seconds: 25 * 60,
		paused_seconds: 0,
		started_at: START,
		paused_at: null,
		...overrides,
	}
}

describe('focus timer', () => {
	it('counts down from the server start time', () => {
		expect(remainingSeconds(session(), startSeconds)).toBe(1500)
		expect(remainingSeconds(session(), startSeconds + 600)).toBe(900)
		expect(remainingSeconds(session(), startSeconds + 5000)).toBe(0)
	})

	it('excludes earlier pauses and freezes while paused', () => {
		expect(elapsedSeconds(session({paused_seconds: 120}), startSeconds + 600)).toBe(480)
		const paused = session({paused_at: '2026-10-14T09:10:00Z'})
		expect(elapsedSeconds(paused, startSeconds + 600)).toBe(600)
		expect(elapsedSeconds(paused, startSeconds + 3600)).toBe(600)
	})

	it('reports progress between 0 and 1', () => {
		expect(progress(session(), startSeconds)).toBe(0)
		expect(progress(session(), startSeconds + 750)).toBe(0.5)
		expect(progress(session(), startSeconds + 9999)).toBe(1)
	})

	it('formats the clock', () => {
		expect(formatClock(1500)).toBe('25:00')
		expect(formatClock(65)).toBe('01:05')
		expect(formatClock(0)).toBe('00:00')
		expect(formatClock(3900)).toBe('1:05:00')
	})

	it('formats durations for stats', () => {
		expect(formatDuration(0)).toBe('0 min')
		expect(formatDuration(1500)).toBe('25 min')
		expect(formatDuration(3900)).toBe('1 h 05 min')
	})

	it('gives a long break after every Nth focus session', () => {
		expect(nextBreakKind(1, 3)).toBe('short_break')
		expect(nextBreakKind(2, 3)).toBe('short_break')
		expect(nextBreakKind(3, 3)).toBe('long_break')
		expect(nextBreakKind(6, 3)).toBe('long_break')
		expect(nextBreakKind(0, 3)).toBe('short_break')
	})
})
