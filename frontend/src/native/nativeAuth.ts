import {App} from '@capacitor/app'
import {Browser} from '@capacitor/browser'

import type {OAuthTokens, VikunjaDesktop} from '@/types/desktop'

// The Android app signs in like the desktop app: the system browser opens the
// server's authorize page and hands a code back through a custom scheme. The
// refresh cookie cannot reach the app (it runs on another origin), so the app
// keeps the OAuth refresh token instead. The server accepts any vikunja-* scheme.
export const NATIVE_CLIENT_ID = 'hefzul-android'
export const NATIVE_REDIRECT_URI = 'vikunja-hefzul://callback'

const PENDING_LOGIN_KEY = 'nativeOAuthPendingLogin'

interface PendingLogin {
	apiUrl: string
	verifier: string
}

function base64Url(bytes: Uint8Array): string {
	let binary = ''
	for (const byte of bytes) binary += String.fromCharCode(byte)
	return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

async function codeChallenge(verifier: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
	return base64Url(new Uint8Array(digest))
}

function rootUrl(apiUrl: string): string {
	return apiUrl.replace(/\/+$/, '')
}

async function postToken(apiUrl: string, body: Record<string, string>): Promise<OAuthTokens> {
	const response = await fetch(`${rootUrl(apiUrl)}/api/v2/oauth/token`, {
		method: 'POST',
		headers: {'Content-Type': 'application/json'},
		body: JSON.stringify(body),
	})
	const data = await response.json().catch(() => ({}))
	if (!response.ok) {
		throw new Error(data?.message || `HTTP ${response.status}`)
	}
	return data as OAuthTokens
}

/** The authorize URL; the page is served by the web app, not the API. */
export function authorizeUrl(apiUrl: string, challenge: string): string {
	const url = new URL(rootUrl(apiUrl))
	url.pathname = url.pathname.replace(/\/+$/, '') + '/oauth/authorize'
	url.searchParams.set('response_type', 'code')
	url.searchParams.set('client_id', NATIVE_CLIENT_ID)
	url.searchParams.set('redirect_uri', NATIVE_REDIRECT_URI)
	url.searchParams.set('code_challenge', challenge)
	url.searchParams.set('code_challenge_method', 'S256')
	return url.toString()
}

export function createNativeAuthBridge(): VikunjaDesktop {
	let onTokens: ((tokens: OAuthTokens) => void) | null = null
	let onError: ((error: string) => void) | null = null
	// A callback can arrive before the login page has subscribed (cold start).
	let pendingTokens: OAuthTokens | null = null
	let pendingError: string | null = null

	function emitTokens(tokens: OAuthTokens) {
		if (onTokens) onTokens(tokens)
		else pendingTokens = tokens
	}

	function emitError(error: string) {
		if (onError) onError(error)
		else pendingError = error
	}

	async function handleCallback(rawUrl: string) {
		if (!rawUrl.startsWith(NATIVE_REDIRECT_URI)) return
		Browser.close().catch(() => {})

		const url = new URL(rawUrl)
		const error = url.searchParams.get('error')
		const code = url.searchParams.get('code')
		let pending: PendingLogin | null
		try {
			pending = JSON.parse(localStorage.getItem(PENDING_LOGIN_KEY) ?? 'null')
		} catch {
			pending = null
		}
		localStorage.removeItem(PENDING_LOGIN_KEY)

		if (error || !code) {
			emitError(url.searchParams.get('error_description') || error || 'No authorization code')
			return
		}
		if (!pending) {
			emitError('No pending login session')
			return
		}

		try {
			emitTokens(await postToken(pending.apiUrl, {
				grant_type: 'authorization_code',
				code,
				client_id: NATIVE_CLIENT_ID,
				redirect_uri: NATIVE_REDIRECT_URI,
				code_verifier: pending.verifier,
			}))
		} catch (e) {
			emitError(e instanceof Error ? e.message : String(e))
		}
	}

	App.addListener('appUrlOpen', ({url}) => {
		handleCallback(url)
	})
	App.getLaunchUrl().then(launch => {
		if (launch?.url) handleCallback(launch.url)
	}).catch(() => {})

	return {
		isDesktop: true,
		platform: 'android',
		async startOAuthLogin(apiUrl: string) {
			const verifier = base64Url(crypto.getRandomValues(new Uint8Array(32)))
			const pending: PendingLogin = {apiUrl, verifier}
			localStorage.setItem(PENDING_LOGIN_KEY, JSON.stringify(pending))
			await Browser.open({url: authorizeUrl(apiUrl, await codeChallenge(verifier))})
		},
		onOAuthTokens(callback) {
			onTokens = callback
			if (pendingTokens) {
				const tokens = pendingTokens
				pendingTokens = null
				callback(tokens)
			}
		},
		onOAuthError(callback) {
			onError = callback
			if (pendingError) {
				const error = pendingError
				pendingError = null
				callback(error)
			}
		},
		refreshToken(apiUrl, refreshToken) {
			return postToken(apiUrl, {grant_type: 'refresh_token', refresh_token: refreshToken})
		},
		updateQuickEntryShortcut() {
			// The quick entry window only exists on desktop.
		},
	}
}
