import {computed, onScopeDispose, ref, watch} from 'vue'
import {createSharedComposable} from '@vueuse/core'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'

import {queryClient} from '@/client/queryClient'
import {
	currentFocusSessionQuery,
	focusKeys,
	focusSettingsQuery,
	focusStatsQuery,
	normalizeFocusSession,
	patchCurrentFocusSession,
	useFinishFocusSessionMutation,
	usePauseFocusSessionMutation,
	useResumeFocusSessionMutation,
	useStartFocusSessionMutation,
	type FocusKind,
	type FocusSessionResponse,
} from '@/client/queries/focus'
import type {FocusSession} from '@/client/generated'
import {useAuthStore} from '@/stores/auth'
import {useWebSocket} from '@/composables/useWebSocket'
import {serverNowSeconds} from '@/helpers/serverClock'
import {nextBreakKind, remainingSeconds} from '@/helpers/focusTimer'
import {playFocusSound} from '@/helpers/focusSound'

/** Today's date (YYYY-MM-DD) in the given IANA time zone, or the browser's. */
export function todayIn(timezone: string | undefined): string {
	try {
		return new Intl.DateTimeFormat('en-CA', {timeZone: timezone || undefined}).format(new Date())
	} catch {
		return new Intl.DateTimeFormat('en-CA').format(new Date())
	}
}

function notify(title: string, body: string) {
	try {
		if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return
		new Notification(title, {body, tag: 'hefzul-focus'})
	} catch {
		// Some mobile browsers only allow notifications from a service worker.
	}
}

export function requestFocusNotificationPermission() {
	try {
		if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
			Notification.requestPermission().catch(() => {})
		}
	} catch {
		// Notifications are optional.
	}
}

// One timer for the whole app: it keeps counting, finishes the session and
// rings while the user is on any page, not only on the focus page.
export const useFocusTimer = createSharedComposable(() => {
	const {t} = useI18n({useScope: 'global'})
	const authStore = useAuthStore()
	const enabled = computed(() => authStore.authenticated && !authStore.isLinkShareAuth)

	const currentQuery = useQuery(computed(() => ({...currentFocusSessionQuery(), enabled: enabled.value})))
	const settingsQuery = useQuery(computed(() => ({...focusSettingsQuery(), enabled: enabled.value})))
	const current = computed(() => currentQuery.data.value ?? null)
	const settings = computed(() => settingsQuery.data.value)

	const startMutation = useStartFocusSessionMutation()
	const pauseMutation = usePauseFocusSessionMutation()
	const resumeMutation = useResumeFocusSessionMutation()
	const finishMutation = useFinishFocusSessionMutation()

	const now = ref(serverNowSeconds())
	const tick = setInterval(() => {
		now.value = serverNowSeconds()
	}, 1000)
	onScopeDispose(() => clearInterval(tick))

	const remaining = computed(() => current.value ? remainingSeconds(current.value, now.value) : 0)
	const isRunning = computed(() => current.value?.status === 'running')
	const isPaused = computed(() => current.value?.status === 'paused')

	// The break the user is offered after a focus session; null when none is due.
	const suggestedNext = ref<FocusKind | null>(null)

	const socket = useWebSocket()
	const unsubscribe = socket.subscribe('focus.updated', message => {
		const data = message.data as FocusSession | undefined
		if (!data?.id) return
		patchCurrentFocusSession(queryClient, normalizeFocusSession(data))
		queryClient.invalidateQueries({queryKey: focusKeys.statsAll})
	})
	onScopeDispose(unsubscribe)

	async function start(kind: FocusKind, taskId = 0) {
		requestFocusNotificationPermission()
		suggestedNext.value = null
		return startMutation.mutateAsync({kind, task_id: taskId, device: 'web'})
	}

	async function pause() {
		if (current.value) await pauseMutation.mutateAsync({id: current.value.id})
	}

	async function resume() {
		if (current.value) await resumeMutation.mutateAsync({id: current.value.id})
	}

	/** Stops the session early. For a break this is "skip break". */
	async function abandon() {
		if (!current.value) return
		await finishMutation.mutateAsync({id: current.value.id, status: 'abandoned'})
		suggestedNext.value = null
	}

	const finishing = new Set<number>()

	async function complete(session: FocusSessionResponse) {
		if (finishing.has(session.id)) return
		finishing.add(session.id)
		try {
			await finishMutation.mutateAsync({id: session.id, status: 'completed'})
		} catch {
			// Another device may have finished it first; the refetch settles the state.
			return
		} finally {
			finishing.delete(session.id)
		}

		playFocusSound(settings.value?.sound ?? 'bell')
		if (session.kind !== 'focus') {
			notify(t('focus.notification.breakOverTitle'), t('focus.notification.breakOverBody'))
			suggestedNext.value = 'focus'
			return
		}

		notify(t('focus.notification.focusDoneTitle'), t('focus.notification.focusDoneBody'))
		const today = todayIn(authStore.settings.timezone)
		let completedToday = 1
		try {
			const stats = await queryClient.fetchQuery({...focusStatsQuery(today, today), staleTime: 0})
			completedToday = stats.completed_sessions ?? 1
		} catch {
			// Fall back to a short break.
		}
		const breakKind = nextBreakKind(completedToday, settings.value?.sessions_before_long_break ?? 3)
		if (settings.value?.auto_start_breaks && !current.value) {
			await start(breakKind, 0).catch(() => {})
		} else {
			suggestedNext.value = breakKind
		}
	}

	watch([remaining, current], () => {
		const session = current.value
		if (session && session.status === 'running' && remaining.value <= 0) {
			complete(session)
		}
	})

	return {
		current,
		settings,
		remaining,
		isRunning,
		isPaused,
		isLoading: currentQuery.isPending,
		suggestedNext,
		busy: computed(() => startMutation.isPending.value || pauseMutation.isPending.value
			|| resumeMutation.isPending.value || finishMutation.isPending.value),
		start,
		pause,
		resume,
		abandon,
	}
})
