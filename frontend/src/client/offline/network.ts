import {ref} from 'vue'

/** True while the browser reports a network connection. */
export const online = ref(typeof navigator === 'undefined' ? true : navigator.onLine)

if (typeof window !== 'undefined') {
	window.addEventListener('online', () => { online.value = true })
	window.addEventListener('offline', () => { online.value = false })
}

/**
 * A request that never reached the server: fetch rejects with a TypeError
 * (no ApiError, which means the server answered).
 */
// What fetch rejects with when no response arrived: Chromium, Firefox, Safari, Node.
const NETWORK_FAILURE = /failed to fetch|networkerror|load failed|fetch failed|network request failed/i

export function isNetworkError(cause: unknown): boolean {
	if (typeof navigator !== 'undefined' && navigator.onLine === false) return true
	return cause instanceof TypeError && NETWORK_FAILURE.test(cause.message)
}

export function newIdempotencyKey(): string {
	if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
	return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}
