import {queryOptions, useMutation} from '@tanstack/vue-query'
import {
	pushPublicKey,
	pushSubscriptionsCreate,
	pushSubscriptionsDelete,
	pushSubscriptionsList,
	pushTest,
} from '@/client/generated'
import {contextMutationOptions} from './contextMutation'

export const pushKeys = {
	all: ['push'] as const,
	subscriptions: ['push', 'subscriptions'] as const,
}

export function pushSubscriptionsQuery() {
	return queryOptions({
		queryKey: pushKeys.subscriptions,
		queryFn: async ({signal}) => (await pushSubscriptionsList({signal})).data.items ?? [],
	})
}

export function pushSupported(): boolean {
	return typeof window !== 'undefined'
		&& 'serviceWorker' in navigator
		&& 'PushManager' in window
		&& 'Notification' in window
}

function base64UrlToBytes(value: string): Uint8Array<ArrayBuffer> {
	const padded = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
	const raw = atob(padded)
	const bytes = new Uint8Array(new ArrayBuffer(raw.length))
	for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
	return bytes
}

function bytesToBase64Url(buffer: ArrayBuffer | null): string {
	if (!buffer) return ''
	let raw = ''
	for (const byte of new Uint8Array(buffer)) raw += String.fromCharCode(byte)
	return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

/** The browser's current push subscription for this server, if any. */
export async function currentBrowserSubscription(): Promise<PushSubscription | null> {
	if (!pushSupported()) return null
	const registration = await navigator.serviceWorker.getRegistration()
	return registration ? registration.pushManager.getSubscription() : null
}

function deviceName(): string {
	const ua = navigator.userAgent
	const browser = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : 'Browser'
	const os = /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : /Windows/.test(ua) ? 'Windows' : /Mac OS/.test(ua) ? 'macOS' : /Linux/.test(ua) ? 'Linux' : ''
	return os ? `${browser} on ${os}` : browser
}

export function useEnablePushMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({includeContent}: {includeContent: boolean}) => {
			const permission = await Notification.requestPermission()
			if (permission !== 'granted') throw new Error('Notifications are blocked for this site in the browser settings.')
			const registration = await navigator.serviceWorker.ready
			const {data} = await pushPublicKey()
			let subscription = await registration.pushManager.getSubscription()
			if (!subscription) {
				subscription = await registration.pushManager.subscribe({
					userVisibleOnly: true,
					applicationServerKey: base64UrlToBytes(data.public_key ?? ''),
				})
			}
			await pushSubscriptionsCreate({body: {
				endpoint: subscription.endpoint,
				p256dh: bytesToBase64Url(subscription.getKey('p256dh')),
				auth: bytesToBase64Url(subscription.getKey('auth')),
				include_content: includeContent,
				device: deviceName(),
			}})
		},
		onSettled: (_input, client) => client.invalidateQueries({queryKey: pushKeys.all}),
	}))
}

export function useDisablePushMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({endpoint}: {endpoint?: string} = {}) => {
			const subscription = await currentBrowserSubscription()
			const target = endpoint ?? subscription?.endpoint
			if (!target) return
			await pushSubscriptionsDelete({body: {endpoint: target}}).catch(() => undefined)
			if (subscription && subscription.endpoint === target) await subscription.unsubscribe()
		},
		onSettled: (_input, client) => client.invalidateQueries({queryKey: pushKeys.all}),
	}))
}

export function useTestPushMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async () => (await pushTest()).data.delivered ?? 0,
	}))
}
