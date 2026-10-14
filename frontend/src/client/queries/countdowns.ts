import {queryOptions, useMutation, type QueryClient} from '@tanstack/vue-query'
import {
	countdownsCreate,
	countdownsDelete,
	countdownsList,
	countdownsUpdate,
} from '@/client/generated'
import type {Countdown} from '@/client/generated'
import {contextMutationOptions} from './contextMutation'
import {fetchAllPages} from './fetchAllPages'

export interface CountdownResponse {
	id: number
	title: string
	target_date: string
	repeat_yearly: boolean
	hex_color: string
	pinned: boolean
	next_date: string
	days_remaining: number
	years: number
}

export type CountdownInput = Pick<CountdownResponse, 'title' | 'target_date' | 'repeat_yearly' | 'hex_color' | 'pinned'>

export function normalizeCountdown(countdown: Countdown): CountdownResponse {
	return {
		id: countdown.id ?? 0,
		title: countdown.title ?? '',
		target_date: countdown.target_date ?? '',
		repeat_yearly: countdown.repeat_yearly ?? false,
		hex_color: countdown.hex_color ?? '',
		pinned: countdown.pinned ?? false,
		next_date: countdown.next_date ?? '',
		days_remaining: countdown.days_remaining ?? 0,
		years: countdown.years ?? 0,
	}
}

export const countdownKeys = {
	all: ['countdowns'] as const,
	list: (withinDays: number) => ['countdowns', 'list', withinDays] as const,
}

/** withinDays 0 lists all countdowns; otherwise pinned ones and those due within that many days. */
export function countdownsQuery(withinDays = 0) {
	return queryOptions({
		queryKey: countdownKeys.list(withinDays),
		queryFn: async ({signal}) => {
			const items = await fetchAllPages(async page => (await countdownsList({
				query: {page, within_days: withinDays},
				signal,
			})).data)
			return items.map(normalizeCountdown)
		},
	})
}

const invalidate = (client: QueryClient) => client.invalidateQueries({queryKey: countdownKeys.all})

export function useCreateCountdownMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async (body: CountdownInput) => normalizeCountdown((await countdownsCreate({body})).data),
		onSettled: (_input, client) => invalidate(client),
	}))
}

export function useUpdateCountdownMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({id, ...body}: CountdownInput & {id: number}) =>
			normalizeCountdown((await countdownsUpdate({path: {id}, body})).data),
		onSettled: (_input, client) => invalidate(client),
	}))
}

export function useDeleteCountdownMutation() {
	return useMutation(contextMutationOptions({
		mutationFn: async ({id}: {id: number}) => { await countdownsDelete({path: {id}}) },
		onSettled: (_input, client) => invalidate(client),
	}))
}
