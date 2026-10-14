import {
	queryOptions,
	useMutation,
	type QueryClient,
} from '@tanstack/vue-query'
import {
	focusSessionsCurrent,
	focusSessionsFinish,
	focusSessionsPause,
	focusSessionsResume,
	focusSessionsStart,
	focusSettingsRead,
	focusSettingsUpdate,
	focusStats,
} from '@/client/generated'
import type {
	FocusSession,
	FocusSessionWritable,
	FocusSettings,
	FocusSettingsWritable,
	FocusStats,
} from '@/client/generated'
import {contextMutationOptions} from './contextMutation'

export type FocusKind = 'focus' | 'short_break' | 'long_break'
export type FocusStatus = 'running' | 'paused' | 'completed' | 'abandoned'

export interface FocusSessionResponse {
	id: number
	task_id: number
	kind: FocusKind
	status: FocusStatus
	planned_seconds: number
	actual_seconds: number
	paused_seconds: number
	started_at: string
	paused_at: string | null
	ended_at: string | null
	device: string
}

export type FocusSettingsResponse = Required<Omit<FocusSettings, '$schema'>>

export const DEFAULT_FOCUS_SETTINGS: FocusSettingsResponse = {
	focus_minutes: 25,
	short_break_minutes: 5,
	long_break_minutes: 15,
	sessions_before_long_break: 3,
	auto_start_breaks: false,
	sound: 'bell',
}

export function normalizeFocusSession(session: FocusSession): FocusSessionResponse {
	return {
		id: session.id ?? 0,
		task_id: session.task_id ?? 0,
		kind: (session.kind ?? 'focus') as FocusKind,
		status: (session.status ?? 'running') as FocusStatus,
		planned_seconds: session.planned_seconds ?? 0,
		actual_seconds: session.actual_seconds ?? 0,
		paused_seconds: session.paused_seconds ?? 0,
		started_at: session.started_at ?? '',
		paused_at: session.paused_at ?? null,
		ended_at: session.ended_at ?? null,
		device: session.device ?? '',
	}
}

export function normalizeFocusSettings(settings: FocusSettings | undefined): FocusSettingsResponse {
	return {
		focus_minutes: settings?.focus_minutes ?? DEFAULT_FOCUS_SETTINGS.focus_minutes,
		short_break_minutes: settings?.short_break_minutes ?? DEFAULT_FOCUS_SETTINGS.short_break_minutes,
		long_break_minutes: settings?.long_break_minutes ?? DEFAULT_FOCUS_SETTINGS.long_break_minutes,
		sessions_before_long_break: settings?.sessions_before_long_break ?? DEFAULT_FOCUS_SETTINGS.sessions_before_long_break,
		auto_start_breaks: settings?.auto_start_breaks ?? DEFAULT_FOCUS_SETTINGS.auto_start_breaks,
		sound: settings?.sound || DEFAULT_FOCUS_SETTINGS.sound,
	}
}

export const focusKeys = {
	all: ['focus'] as const,
	settings: ['focus', 'settings'] as const,
	current: ['focus', 'current'] as const,
	statsAll: ['focus', 'stats'] as const,
	stats: (from: string, to: string) => ['focus', 'stats', from, to] as const,
}

export function focusSettingsQuery() {
	return queryOptions({
		queryKey: focusKeys.settings,
		queryFn: async ({signal}) => normalizeFocusSettings((await focusSettingsRead({signal})).data),
	})
}

export function currentFocusSessionQuery() {
	return queryOptions({
		queryKey: focusKeys.current,
		// 204 when nothing runs: the body is empty.
		queryFn: async ({signal}) => {
			const {data} = await focusSessionsCurrent({signal})
			return data && typeof data === 'object' && data.id ? normalizeFocusSession(data) : null
		},
		// Fallback for missed websocket pushes from another device.
		refetchInterval: 30_000,
	})
}

export function focusStatsQuery(from: string, to: string) {
	return queryOptions({
		queryKey: focusKeys.stats(from, to),
		queryFn: async ({signal}): Promise<FocusStats> => (await focusStats({query: {from, to}, signal})).data,
	})
}

/** Writes the session into the current-session cache; a finished one clears it. */
export function patchCurrentFocusSession(client: QueryClient, session: FocusSessionResponse) {
	const active = session.status === 'running' || session.status === 'paused'
	client.setQueryData<FocusSessionResponse | null>(focusKeys.current, current => {
		if (active) return session
		return current?.id === session.id ? null : current ?? null
	})
}

function settle(client: QueryClient) {
	return Promise.all([
		client.invalidateQueries({queryKey: focusKeys.current}),
		client.invalidateQueries({queryKey: focusKeys.statsAll}),
	])
}

export function startFocusSessionMutationOptions() {
	return contextMutationOptions({
		mutationFn: async (body: FocusSessionWritable) => normalizeFocusSession((await focusSessionsStart({body})).data),
		onSuccess: (session, _input, client) => patchCurrentFocusSession(client, session),
		onSettled: (_input, client) => settle(client),
	})
}

export type FocusActionInput = {id: number}

export function pauseFocusSessionMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({id}: FocusActionInput) => normalizeFocusSession((await focusSessionsPause({path: {id}})).data),
		onSuccess: (session, _input, client) => patchCurrentFocusSession(client, session),
		onSettled: (_input, client) => client.invalidateQueries({queryKey: focusKeys.current}),
	})
}

export function resumeFocusSessionMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({id}: FocusActionInput) => normalizeFocusSession((await focusSessionsResume({path: {id}})).data),
		onSuccess: (session, _input, client) => patchCurrentFocusSession(client, session),
		onSettled: (_input, client) => client.invalidateQueries({queryKey: focusKeys.current}),
	})
}

export type FinishFocusInput = {id: number, status: 'completed' | 'abandoned'}

export function finishFocusSessionMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({id, status}: FinishFocusInput) =>
			normalizeFocusSession((await focusSessionsFinish({path: {id}, body: {status}})).data),
		onSuccess: (session, _input, client) => patchCurrentFocusSession(client, session),
		onSettled: (_input, client) => settle(client),
	})
}

export function updateFocusSettingsMutationOptions() {
	return contextMutationOptions({
		mutationFn: async (body: FocusSettingsWritable) => normalizeFocusSettings((await focusSettingsUpdate({body})).data),
		onSuccess: (settings, _input, client) => client.setQueryData(focusKeys.settings, settings),
		onSettled: (_input, client) => client.invalidateQueries({queryKey: focusKeys.settings}),
	})
}

export function useStartFocusSessionMutation() {
	return useMutation(startFocusSessionMutationOptions())
}

export function usePauseFocusSessionMutation() {
	return useMutation(pauseFocusSessionMutationOptions())
}

export function useResumeFocusSessionMutation() {
	return useMutation(resumeFocusSessionMutationOptions())
}

export function useFinishFocusSessionMutation() {
	return useMutation(finishFocusSessionMutationOptions())
}

export function useUpdateFocusSettingsMutation() {
	return useMutation(updateFocusSettingsMutationOptions())
}
