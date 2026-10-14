// Pomodoro timer arithmetic. The server stores when a session started and how
// long it was paused; every client derives the time left from those, so the
// count stays right after the app is closed or the phone sleeps.
//
// No framework imports, so it can be unit-tested in isolation.

export interface TimedSession {
	kind: 'focus' | 'short_break' | 'long_break'
	planned_seconds: number
	paused_seconds: number
	started_at: string
	paused_at: string | null
}

function toSeconds(iso: string | null): number | null {
	if (!iso) return null
	const ms = new Date(iso).getTime()
	return Number.isNaN(ms) ? null : ms / 1000
}

/** Seconds spent in the session so far, pauses excluded. */
export function elapsedSeconds(session: TimedSession, nowSeconds: number): number {
	const started = toSeconds(session.started_at)
	if (started === null) return 0
	const end = toSeconds(session.paused_at) ?? nowSeconds
	return Math.max(0, Math.floor(end - started - session.paused_seconds))
}

/** Seconds left until the session's planned end; never negative. */
export function remainingSeconds(session: TimedSession, nowSeconds: number): number {
	return Math.max(0, session.planned_seconds - elapsedSeconds(session, nowSeconds))
}

/** Share of the session done, from 0 to 1. */
export function progress(session: TimedSession, nowSeconds: number): number {
	if (session.planned_seconds <= 0) return 1
	return Math.min(1, elapsedSeconds(session, nowSeconds) / session.planned_seconds)
}

/** 25:00, or 1:05:00 for an hour or more. */
export function formatClock(totalSeconds: number): string {
	const seconds = Math.max(0, Math.floor(totalSeconds))
	const h = Math.floor(seconds / 3600)
	const m = Math.floor((seconds % 3600) / 60)
	const s = seconds % 60
	const mmss = `${String(m).padStart(h > 0 ? 2 : 1, '0')}:${String(s).padStart(2, '0')}`
	return h > 0 ? `${h}:${mmss}` : mmss.padStart(5, '0')
}

/** 1 h 05 min, 25 min or 0 min, for stats. */
export function formatDuration(totalSeconds: number): string {
	const minutes = Math.round(Math.max(0, totalSeconds) / 60)
	const h = Math.floor(minutes / 60)
	const m = minutes % 60
	if (h === 0) return `${m} min`
	return `${h} h ${String(m).padStart(2, '0')} min`
}

/**
 * The break that follows a completed focus session: every Nth completed focus
 * session (counted over the day) earns a long break.
 */
export function nextBreakKind(completedFocusToday: number, sessionsBeforeLongBreak: number): 'short_break' | 'long_break' {
	const n = Math.max(1, sessionsBeforeLongBreak)
	return completedFocusToday > 0 && completedFocusToday % n === 0 ? 'long_break' : 'short_break'
}
