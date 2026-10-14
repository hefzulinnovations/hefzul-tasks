// The Android app asks the server which APK is current. The server publishes
// it as a static file next to the APK (see deploy/README.md).
export const LATEST_RELEASE_PATH = '/downloads/android/latest.json'

export interface AndroidRelease {
	versionCode: number
	versionName: string
	url: string
}

export function parseRelease(data: unknown, serverRoot: string): AndroidRelease | null {
	if (data === null || typeof data !== 'object') return null
	const record = data as Record<string, unknown>
	const versionCode = Number(record.versionCode)
	if (!Number.isInteger(versionCode) || versionCode <= 0 || typeof record.url !== 'string' || record.url === '') {
		return null
	}

	let url: URL
	try {
		// A relative url is relative to the server, not to the app.
		url = new URL(record.url, serverRoot.replace(/\/*$/, '/'))
	} catch {
		return null
	}
	if (url.protocol !== 'https:' && url.protocol !== 'http:') return null

	return {
		versionCode,
		versionName: typeof record.versionName === 'string' ? record.versionName : String(versionCode),
		url: url.toString(),
	}
}

/** Whether the server offers a newer build than the installed one. */
export function isNewerRelease(release: AndroidRelease | null, installedBuild: string | number): release is AndroidRelease {
	const installed = Number(installedBuild)
	return release !== null && Number.isFinite(installed) && release.versionCode > installed
}
