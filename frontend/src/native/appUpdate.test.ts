import {describe, expect, it} from 'vitest'

import {isNewerRelease, parseRelease} from './appUpdate'

const root = 'https://tasks.example.com'

describe('parseRelease', () => {
	it('resolves a relative url against the server', () => {
		expect(parseRelease({versionCode: 12, versionName: '2.7.0-12', url: '/downloads/android/hefzul-tasks.apk'}, root)).toEqual({
			versionCode: 12,
			versionName: '2.7.0-12',
			url: 'https://tasks.example.com/downloads/android/hefzul-tasks.apk',
		})
	})

	it('keeps a server under a sub path', () => {
		expect(parseRelease({versionCode: 3, url: 'hefzul.apk'}, 'https://example.com/tasks')?.url)
			.toBe('https://example.com/tasks/hefzul.apk')
	})

	it('rejects incomplete or unsafe data', () => {
		expect(parseRelease(null, root)).toBeNull()
		expect(parseRelease({versionCode: 'x', url: '/a.apk'}, root)).toBeNull()
		expect(parseRelease({versionCode: 4}, root)).toBeNull()
		expect(parseRelease({versionCode: 4, url: 'javascript:alert(1)'}, root)).toBeNull()
	})
})

describe('isNewerRelease', () => {
	const release = {versionCode: 12, versionName: '12', url: `${root}/a.apk`}

	it('compares build numbers', () => {
		expect(isNewerRelease(release, '11')).toBe(true)
		expect(isNewerRelease(release, '12')).toBe(false)
		expect(isNewerRelease(release, 'dev')).toBe(false)
		expect(isNewerRelease(null, '1')).toBe(false)
	})
})
