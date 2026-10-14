import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

const listeners: Record<string, (data: {url: string}) => void> = {}
const browserOpen = vi.fn()

vi.mock('@capacitor/app', () => ({
	App: {
		addListener: vi.fn((event: string, callback: (data: {url: string}) => void) => {
			listeners[event] = callback
			return Promise.resolve({remove: vi.fn()})
		}),
		getLaunchUrl: vi.fn(() => Promise.resolve(undefined)),
	},
}))

vi.mock('@capacitor/browser', () => ({
	Browser: {
		open: (options: {url: string}) => browserOpen(options),
		close: vi.fn(() => Promise.resolve()),
	},
}))

import {authorizeUrl, createNativeAuthBridge, NATIVE_CLIENT_ID, NATIVE_REDIRECT_URI} from './nativeAuth'

const tokens = {access_token: 'access', refresh_token: 'refresh', expires_in: 900}

describe('native auth bridge', () => {
	let fetchMock: ReturnType<typeof vi.fn>

	beforeEach(() => {
		localStorage.clear()
		browserOpen.mockReset()
		fetchMock = vi.fn(() => Promise.resolve(new Response(JSON.stringify(tokens), {status: 200})))
		vi.stubGlobal('fetch', fetchMock)
	})

	afterEach(() => {
		vi.unstubAllGlobals()
	})

	it('builds the authorize url on the web app', () => {
		const url = new URL(authorizeUrl('https://tasks.example.com/', 'challenge'))
		expect(url.origin + url.pathname).toBe('https://tasks.example.com/oauth/authorize')
		expect(url.searchParams.get('client_id')).toBe(NATIVE_CLIENT_ID)
		expect(url.searchParams.get('redirect_uri')).toBe(NATIVE_REDIRECT_URI)
		expect(url.searchParams.get('code_challenge_method')).toBe('S256')
	})

	it('opens the browser and exchanges the returned code with the PKCE verifier', async () => {
		const bridge = createNativeAuthBridge()
		const received = vi.fn()
		bridge.onOAuthTokens(received)

		await bridge.startOAuthLogin('https://tasks.example.com')
		const opened = new URL(browserOpen.mock.calls[0][0].url)
		const challenge = opened.searchParams.get('code_challenge')

		listeners.appUrlOpen({url: `${NATIVE_REDIRECT_URI}?code=abc`})
		await vi.waitFor(() => expect(received).toHaveBeenCalledWith(tokens))

		const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
		expect(url).toBe('https://tasks.example.com/api/v2/oauth/token')
		const body = JSON.parse(init.body as string)
		expect(body).toMatchObject({grant_type: 'authorization_code', code: 'abc', client_id: NATIVE_CLIENT_ID, redirect_uri: NATIVE_REDIRECT_URI})

		// The verifier sent must hash to the challenge the browser got.
		const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(body.code_verifier))
		const expected = btoa(String.fromCharCode(...new Uint8Array(digest))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
		expect(challenge).toBe(expected)
	})

	it('keeps tokens that arrive before the login page subscribes', async () => {
		const bridge = createNativeAuthBridge()
		await bridge.startOAuthLogin('https://tasks.example.com')
		listeners.appUrlOpen({url: `${NATIVE_REDIRECT_URI}?code=abc`})
		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
		await new Promise(resolve => setTimeout(resolve, 0))

		const received = vi.fn()
		bridge.onOAuthTokens(received)
		expect(received).toHaveBeenCalledWith(tokens)
	})

	it('reports an error from the server and a callback without a login', async () => {
		const bridge = createNativeAuthBridge()
		const errors = vi.fn()
		bridge.onOAuthError(errors)

		listeners.appUrlOpen({url: `${NATIVE_REDIRECT_URI}?code=abc`})
		await vi.waitFor(() => expect(errors).toHaveBeenCalledWith('No pending login session'))

		await bridge.startOAuthLogin('https://tasks.example.com')
		listeners.appUrlOpen({url: `${NATIVE_REDIRECT_URI}?error=access_denied`})
		await vi.waitFor(() => expect(errors).toHaveBeenCalledWith('access_denied'))
		expect(fetchMock).not.toHaveBeenCalled()
	})

	it('refreshes with the stored refresh token', async () => {
		const bridge = createNativeAuthBridge()
		await expect(bridge.refreshToken('https://tasks.example.com', 'old')).resolves.toEqual(tokens)
		const body = JSON.parse((fetchMock.mock.calls[0] as [string, RequestInit])[1].body as string)
		expect(body).toEqual({grant_type: 'refresh_token', refresh_token: 'old'})
	})

	it('ignores other urls', async () => {
		const bridge = createNativeAuthBridge()
		const errors = vi.fn()
		bridge.onOAuthError(errors)
		listeners.appUrlOpen({url: 'https://example.com/?code=abc'})
		await new Promise(resolve => setTimeout(resolve, 0))
		expect(errors).not.toHaveBeenCalled()
	})
})
