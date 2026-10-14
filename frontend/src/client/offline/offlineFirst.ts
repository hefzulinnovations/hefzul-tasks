import {isNetworkError, newIdempotencyKey, online} from './network'

/**
 * Runs a write online with an Idempotency-Key, or queues it when the device is
 * offline or the request never reached the server. The queued copy reuses the
 * key, so a request that did arrive (only its answer got lost) is not applied
 * twice on replay.
 */
export async function offlineFirst<T>(
	attempt: (headers: {headers: Record<string, string>}) => Promise<T>,
	queue: (key: string) => Promise<T>,
): Promise<T> {
	const key = newIdempotencyKey()
	if (!online.value) return queue(key)
	try {
		return await attempt({headers: {'Idempotency-Key': key}})
	} catch (cause) {
		if (isNetworkError(cause)) return queue(key)
		throw cause
	}
}
