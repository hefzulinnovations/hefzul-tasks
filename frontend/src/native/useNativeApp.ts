import {computed, onScopeDispose, watch} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'
import {App} from '@capacitor/app'
import {LocalNotifications} from '@capacitor/local-notifications'
import type {PluginListenerHandle} from '@capacitor/core'

import {habitKeys, habitsQuery} from '@/client/queries/habits'
import {queryClient} from '@/client/queryClient'
import {useFocusTimer} from '@/composables/useFocusTimer'
import {formatClock} from '@/helpers/focusTimer'
import {getApiRootUrl} from '@/helpers/apiUrl'
import {success} from '@/message'
import {useAuthStore} from '@/stores/auth'

import {isNewerRelease, LATEST_RELEASE_PATH, parseRelease} from './appUpdate'
import {HefzulFocus} from './focusPlugin'
import {HABIT_REMINDER_ID_BASE, planHabitReminders} from './habitReminderPlan'

const FOCUS_END_NOTIFICATION_ID = 1001
const FOCUS_CHANNEL = 'focus-end'
const HABIT_CHANNEL = 'habit-reminders'

let permissionAsked = false

async function ensureNotificationPermission(): Promise<boolean> {
	try {
		const status = await LocalNotifications.checkPermissions()
		if (status.display === 'granted') return true
		if (permissionAsked) return false
		permissionAsked = true
		return (await LocalNotifications.requestPermissions()).display === 'granted'
	} catch {
		return false
	}
}

async function createChannels(t: (key: string) => string) {
	try {
		await LocalNotifications.createChannel({
			id: FOCUS_CHANNEL,
			name: t('native.channels.focus'),
			importance: 5,
			vibration: true,
			visibility: 1,
		})
		await LocalNotifications.createChannel({
			id: HABIT_CHANNEL,
			name: t('native.channels.habits'),
			importance: 4,
			vibration: true,
			visibility: 1,
		})
	} catch (e) {
		console.warn('Could not create notification channels', e)
	}
}

async function checkForUpdate(t: (key: string, values?: Record<string, unknown>) => string) {
	try {
		const root = getApiRootUrl()
		const [info, response] = await Promise.all([
			App.getInfo(),
			fetch(`${root}${LATEST_RELEASE_PATH}`, {cache: 'no-store'}),
		])
		if (!response.ok) return
		const release = parseRelease(await response.json(), root)
		if (!isNewerRelease(release, info.build)) return
		success({message: t('native.updateAvailable', {version: release.versionName})}, [{
			title: t('native.updateDownload'),
			// The WebView hands other origins to the system browser, which downloads the APK.
			callback: () => window.location.assign(release.url),
		}])
	} catch {
		// No release published or the server is unreachable; try again next start.
	}
}

/**
 * Native behaviour of the Android app: the ongoing focus notification, the
 * end-of-session alarm, local habit reminders and the update check.
 */
export function useNativeApp() {
	const {t} = useI18n({useScope: 'global'})
	const authStore = useAuthStore()
	const timer = useFocusTimer()
	const enabled = computed(() => authStore.authenticated && !authStore.isLinkShareAuth)

	createChannels(t)
	checkForUpdate(t)

	const handles: Promise<PluginListenerHandle>[] = []
	onScopeDispose(() => {
		for (const handle of handles) handle.then(h => h.remove()).catch(() => {})
	})

	// Focus: the time left comes from the server's timestamps, so the
	// notification stays right after the app is killed or the phone sleeps.
	async function syncFocus() {
		const session = timer.current.value
		const running = session?.status === 'running' && timer.remaining.value > 0
		const paused = session?.status === 'paused'

		try {
			if (!session || (!running && !paused)) {
				await HefzulFocus.hide()
				await LocalNotifications.cancel({notifications: [{id: FOCUS_END_NOTIFICATION_ID}]})
				return
			}

			// Android 13+ hides the ongoing notification without this permission.
			const allowed = await ensureNotificationPermission()
			const endsAt = Date.now() + timer.remaining.value * 1000
			await HefzulFocus.show({
				title: t(`focus.kind.${session.kind}`),
				endsAt: running ? endsAt : undefined,
				paused,
				pausedText: `${t('focus.paused')} · ${t('focus.timeLeft', {time: formatClock(timer.remaining.value)})}`,
				pauseLabel: t('focus.pause'),
				resumeLabel: t('focus.resume'),
				stopLabel: t('focus.stop'),
			})

			await LocalNotifications.cancel({notifications: [{id: FOCUS_END_NOTIFICATION_ID}]})
			if (running && allowed) {
				const isFocus = session.kind === 'focus'
				await LocalNotifications.schedule({notifications: [{
					id: FOCUS_END_NOTIFICATION_ID,
					title: t(isFocus ? 'focus.notification.focusDoneTitle' : 'focus.notification.breakOverTitle'),
					body: t(isFocus ? 'focus.notification.focusDoneBody' : 'focus.notification.breakOverBody'),
					channelId: FOCUS_CHANNEL,
					schedule: {at: new Date(endsAt), allowWhileIdle: true},
					extra: {url: '/focus'},
				}]})
			}
		} catch (e) {
			console.warn('Could not update the focus notification', e)
		}
	}

	// Only the session and its status matter, not every tick of the clock.
	watch(
		() => {
			const session = timer.current.value
			return session ? `${session.id}:${session.status}:${session.paused_seconds}:${session.started_at}` : ''
		},
		syncFocus,
		{immediate: true},
	)

	handles.push(HefzulFocus.addListener('action', ({action}) => {
		const run = action === 'pause' ? timer.pause : action === 'resume' ? timer.resume : timer.abandon
		run().catch(e => console.warn('Focus action failed', e))
	}))

	// Habits: plan the next week's reminders whenever the list changes.
	const habits = useQuery(computed(() => ({...habitsQuery(false), enabled: enabled.value})))

	async function syncHabitReminders() {
		try {
			const pending = await LocalNotifications.getPending()
			const stale = pending.notifications.filter(n => n.id >= HABIT_REMINDER_ID_BASE)
			if (stale.length > 0) {
				await LocalNotifications.cancel({notifications: stale.map(n => ({id: n.id}))})
			}

			const planned = enabled.value ? planHabitReminders(habits.data.value ?? [], new Date()) : []
			if (planned.length === 0 || !await ensureNotificationPermission()) return

			await LocalNotifications.schedule({notifications: planned.map(reminder => ({
				id: reminder.id,
				title: t('native.habitReminderTitle'),
				body: reminder.title,
				channelId: HABIT_CHANNEL,
				// A few minutes late is fine for a habit, and needs no alarm permission.
				schedule: {at: reminder.at, allowWhileIdle: true},
				isExactNotification: false,
				extra: {url: '/habits'},
			}))})
		} catch (e) {
			console.warn('Could not schedule habit reminders', e)
		}
	}

	watch([habits.data, enabled], syncHabitReminders, {immediate: true})

	// Tapping a notification opens its page.
	handles.push(LocalNotifications.addListener('localNotificationActionPerformed', ({notification}) => {
		const url = notification.extra?.url
		if (typeof url === 'string' && url.startsWith('/')) {
			import('@/router').then(({default: router}) => router.push(url)).catch(() => {})
		}
	}))

	// Back in the foreground: a new day may have started, and the session
	// may have ended or changed on another device.
	handles.push(App.addListener('resume', () => {
		queryClient.invalidateQueries({queryKey: habitKeys.all})
		syncFocus()
	}))
}
