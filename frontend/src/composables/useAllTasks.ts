import {computed, toValue, type MaybeRefOrGetter} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {allTasksQuery, type TaskScope} from '@/client/queries/tasks'

export type UseAllTasksOptions = {
	enabled?: MaybeRefOrGetter<boolean>
}

// Like useTasks, but sweeps every page of the scope. Use it only when the view
// needs the whole set (such as the Eisenhower matrix); lists should paginate.
export function useAllTasks(
	scope: MaybeRefOrGetter<TaskScope>,
	{enabled = true}: UseAllTasksOptions = {},
) {
	const query = useQuery(computed(() => ({
		...allTasksQuery(toValue(scope)),
		enabled: toValue(enabled),
	})))
	const tasks = computed(() => query.data.value ?? [])
	return {...query, tasks}
}
