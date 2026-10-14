import {getFullBaseUrl} from './helpers/getFullBaseUrl'

declare let self: ServiceWorkerGlobalScope & {
	__precacheManifest?: Parameters<typeof precacheAndRoute>[0],
}
declare const workbox: {
	core: typeof import('workbox-core'),
	routing: typeof import('workbox-routing'),
	strategies: typeof import('workbox-strategies'),
	precaching: typeof import('workbox-precaching'),
	setConfig(options: {modulePathPrefix: string}): void,
}
declare const __WORKBOX_VERSION__: string

const fullBaseUrl = getFullBaseUrl()
const workboxVersion = __WORKBOX_VERSION__

importScripts(`${fullBaseUrl}workbox-${workboxVersion}/workbox-sw.js`)
workbox.setConfig({
	modulePathPrefix: `${fullBaseUrl}workbox-${workboxVersion}`,
})

// Page loads go to the network first; the server's own index page (with the
// API URL it injects) is kept, so the app also opens offline on any route.
const PAGE_CACHE = 'hefzul-pages'
const pageKey = fullBaseUrl
workbox.routing.registerRoute(
	new workbox.routing.NavigationRoute(async ({request}) => {
		try {
			const response = await fetch(request)
			if (response.ok && response.headers.get('Content-Type')?.includes('text/html')) {
				const cache = await caches.open(PAGE_CACHE)
				await cache.put(pageKey, response.clone())
			}
			return response
		} catch {
			const cache = await caches.open(PAGE_CACHE)
			return (await cache.match(pageKey))
				?? (await workbox.precaching.matchPrecache(`${fullBaseUrl}index.html`))
				?? Response.error()
		}
	}, {
		denylist: [/\/api\//, /\/dav\//, /\/feeds\//, /\/\.well-known\//],
	}),
)

import { precacheAndRoute } from 'workbox-precaching'
precacheAndRoute(self.__WB_MANIFEST)

// Cache assets
workbox.routing.registerRoute(
	// This regexp matches all files in precache-manifest
	new RegExp('.+\\.(css|json|js|svg|woff2|png|html|txt|wav)$'),
	new workbox.strategies.StaleWhileRevalidate(),
)

// Construct pattern with full base URL
const apiRoutePattern = new RegExp(`${fullBaseUrl.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}api\\/v1\\/.*$`)
// Always send api requests through the network and bypass the browser's HTTP cache
workbox.routing.registerRoute(
	apiRoutePattern,
	new workbox.strategies.NetworkOnly({
		fetchOptions: {
			cache: 'no-store',
		},
	}),
)

// This code listens for the user's confirmation to update the app.
self.addEventListener('message', (e) => {
	if (!e.data) {
		return
	}

	switch (e.data) {
		case 'skipWaiting':
			self.skipWaiting()
			break
		default:
			// NOOP
			break
	}
})

// Web Push from the Hefzul Tasks server. Without the content opt-in the
// payload carries only a generic text and the URL to open.
interface PushPayload {
	title?: string
	body?: string
	url?: string
	tag?: string
	kind?: string
}

self.addEventListener('push', (event) => {
	let payload: PushPayload
	try {
		payload = event.data?.json() ?? {}
	} catch {
		payload = {body: event.data?.text()}
	}
	event.waitUntil(self.registration.showNotification(payload.title || 'Hefzul Tasks', {
		body: payload.body || '',
		tag: payload.tag,
		icon: `${fullBaseUrl}images/icons/android-chrome-192x192.png`,
		badge: `${fullBaseUrl}images/icons/badge-monochrome.png`,
		data: {url: payload.url || '/'},
	}))
})

// Notification action
self.addEventListener('notificationclick', function (event) {
	const data = event.notification.data ?? {}
	event.notification.close()

	if (event.action === 'show-task' && data.taskId) {
		event.waitUntil(self.clients.openWindow(`${fullBaseUrl}tasks/${data.taskId}`))
		return
	}
	if (!data.url) return

	// Focus an open window and navigate it, or open a new one.
	const target = new URL(String(data.url).replace(/^\//, ''), fullBaseUrl).href
	event.waitUntil((async () => {
		const windows = await self.clients.matchAll({type: 'window', includeUncontrolled: true})
		const existing = windows.find(client => client.url.startsWith(fullBaseUrl))
		if (existing) {
			await existing.focus()
			await existing.navigate(target).catch(() => undefined)
			return
		}
		await self.clients.openWindow(target)
	})())
})

workbox.core.clientsClaim()
// The precaching code provided by Workbox.
self.__precacheManifest = [...(self.__precacheManifest ?? [])]
workbox.precaching.precacheAndRoute(self.__precacheManifest, {})

