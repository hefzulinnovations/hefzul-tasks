import {queryOptions, useMutation} from '@tanstack/vue-query'
import {
	habitsCheckin,
	habitsCheckinUndo,
	habitsCreate,
	habitsDelete,
	habitsList,
	habitsStats,
	habitsUpdate,
} from '@/client/generated'
import type {Habit, HabitStats, HabitWritable} from '@/client/generated'
import {contextMutationOptions} from './contextMutation'
import {fetchAllPages} from './fetchAllPages'
import {offlineFirst} from '@/client/offline/offlineFirst'
import {enqueue} from '@/client/offline/outbox'
import {online} from '@/client/offline/network'
import {todayIn} from '@/helpers/todayIn'
import {useAuthStore} from '@/stores/auth'

export type HabitScheduleKind = 'daily' | 'weekdays' | 'times_per_week'

export interface HabitResponse {
	id: number
	title: string
	icon: string
	hex_color: string
	schedule_kind: HabitScheduleKind
	weekdays: number
	times_per_week: number
	daily_target: number
	reminder_time: string
	archived: boolean
	position: number
	today_count: number
	done_today: boolean
	due_today: boolean
	current_streak: number
}

export type HabitInput = Omit<HabitResponse, 'id' | 'today_count' | 'done_today' | 'due_today' | 'current_streak'>

export function normalizeHabit(habit: Habit): HabitResponse {
	return {
		id: habit.id ?? 0,
		title: habit.title ?? '',
		icon: habit.icon ?? '',
		hex_color: habit.hex_color ?? '',
		schedule_kind: (habit.schedule_kind ?? 'daily') as HabitScheduleKind,
		weekdays: habit.weekdays ?? 0,
		times_per_week: habit.times_per_week ?? 0,
		daily_target: habit.daily_target || 1,
		reminder_time: habit.reminder_time ?? '',
		archived: habit.archived ?? false,
		position: habit.position ?? 0,
		today_count: habit.today_count ?? 0,
		done_today: habit.done_today ?? false,
		due_today: habit.due_today ?? false,
		current_streak: habit.current_streak ?? 0,
	}
}

export function emptyHabitInput(): HabitInput {
	return {
		title: '',
		icon: '',
		hex_color: '',
		schedule_kind: 'daily',
		weekdays: 0,
		times_per_week: 3,
		daily_target: 1,
		reminder_time: '',
		archived: false,
		position: 0,
	}
}

export const habitKeys = {
	all: ['habits'] as const,
	lists: ['habits', 'list'] as const,
	list: (includeArchived: boolean) => ['habits', 'list', includeArchived] as const,
	statsAll: ['habits', 'stats'] as const,
	stats: (id: number, from: string, to: string) => ['habits', 'stats', id, from, to] as const,
}

export function habitsQuery(includeArchived = false) {
	return queryOptions({
		queryKey: habitKeys.list(includeArchived),
		queryFn: async ({signal}) => {
			const items = await fetchAllPages(async page => (await habitsList({
				query: {page, include_archived: includeArchived},
				signal,
			})).data)
			return items.map(normalizeHabit)
		},
	})
}

export function habitStatsQuery(id: number, from: string, to: string) {
	return queryOptions({
		queryKey: habitKeys.stats(id, from, to),
		queryFn: async ({signal}): Promise<HabitStats> => (await habitsStats({path: {id}, query: {from, to}, signal})).data,
	})
}

function writable(input: HabitInput): HabitWritable {
	return {...input, reminder_time: input.reminder_time || ''}
}

const invalidateHabits = (client: import('@tanstack/vue-query').QueryClient) =>
	client.invalidateQueries({queryKey: habitKeys.all})

export function useCreateHabitMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async (input: HabitInput) => normalizeHabit((await habitsCreate({body: writable(input)})).data),
		onSettled: (_input, client) => invalidateHabits(client),
	}))
}

export function useUpdateHabitMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({id, ...input}: HabitInput & {id: number}) =>
			normalizeHabit((await habitsUpdate({path: {id}, body: writable(input)})).data),
		onSettled: (_input, client) => invalidateHabits(client),
	}))
}

export function useDeleteHabitMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({id}: {id: number}) => { await habitsDelete({path: {id}}) },
		onSettled: (_input, client) => invalidateHabits(client),
	}))
}

export type CheckinInput = {id: number, date?: string}

// Check-ins update the list at once, so a tap feels instant.
function bumpToday(client: import('@tanstack/vue-query').QueryClient, id: number, delta: number) {
	for (const [key, habits] of client.getQueriesData<HabitResponse[]>({queryKey: habitKeys.lists})) {
		if (!habits) continue
		client.setQueryData<HabitResponse[]>(key, habits.map(habit => {
			if (habit.id !== id) return habit
			const todayCount = Math.max(0, habit.today_count + delta)
			return {...habit, today_count: todayCount, done_today: todayCount >= habit.daily_target}
		}))
	}
}

export function useCheckinHabitMutation() {
	return useMutation(contextMutationOptions({
		// The day is fixed when the user taps, so a check-in synced tomorrow still counts for today.
		mutationFn: async ({id, date}: CheckinInput) => {
			const day = date ?? todayIn(useAuthStore().settings.timezone)
			return offlineFirst(
				async headers => (await habitsCheckin({path: {id}, body: {date: day}, ...headers})).data,
				async key => {
					await enqueue({kind: 'habit.checkin', key, createdAt: new Date().toISOString(), habitId: id, date: day})
					return {habit_id: id, date: day}
				},
			)
		},
		optimistic: {
			queryKeys: () => [habitKeys.lists],
			update: ({id, date}, client) => { if (!date || date === todayIn(useAuthStore().settings.timezone)) bumpToday(client, id, 1) },
		},
		onSettled: (_input, client) => online.value ? invalidateHabits(client) : Promise.resolve(),
	}))
}

export function useUndoCheckinMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({id, date}: Required<CheckinInput> & {today?: boolean}) =>
			(await habitsCheckinUndo({path: {id, date}})).data,
		optimistic: {
			queryKeys: () => [habitKeys.lists],
			update: ({id, today}, client) => { if (today) bumpToday(client, id, -1) },
		},
		onSettled: (_input, client) => invalidateHabits(client),
	}))
}
