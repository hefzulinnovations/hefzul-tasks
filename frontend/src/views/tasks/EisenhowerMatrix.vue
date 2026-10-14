<template>
	<div
		v-cy="'eisenhowerMatrix'"
		class="matrix-page has-text-start"
	>
		<h2 class="title mbe-2">
			{{ $t('task.matrix.title') }}
		</h2>
		<p class="matrix-explanation mbe-4">
			{{ $t('task.matrix.explanation', {days: URGENT_WITHIN_DAYS}) }}
		</p>

		<div
			class="matrix"
			:class="{'is-loading': isPending}"
		>
			<span
				class="axis axis-top-left"
				aria-hidden="true"
			>
				{{ $t('task.matrix.urgent') }}
			</span>
			<span
				class="axis axis-top-right"
				aria-hidden="true"
			>
				{{ $t('task.matrix.notUrgent') }}
			</span>

			<section
				v-for="quadrant in QUADRANTS"
				:key="quadrant"
				class="quadrant"
				:class="[
					`quadrant-${quadrant}`,
					{'is-drop-target': overQuadrant === quadrant},
				]"
				:aria-labelledby="`matrix-${quadrant}`"
				@dragover.prevent="overQuadrant = quadrant"
				@dragleave="onDragLeave(quadrant)"
				@drop.prevent="onDrop(quadrant)"
			>
				<header class="quadrant-header">
					<h3
						:id="`matrix-${quadrant}`"
						class="quadrant-title"
					>
						{{ $t(`task.matrix.${quadrant}`) }}
						<span class="quadrant-count">{{ grouped[quadrant].length }}</span>
					</h3>
					<p class="quadrant-hint">
						{{ $t(`task.matrix.${quadrant}Hint`) }}
					</p>
				</header>

				<ul
					v-if="grouped[quadrant].length > 0"
					class="quadrant-tasks"
				>
					<li
						v-for="task in grouped[quadrant]"
						:key="task.id"
						class="matrix-task"
						:class="{'is-dragging': draggingId === task.id}"
						:draggable="canEdit(task)"
						@dragstart="onDragStart(task, $event)"
						@dragend="onDragEnd"
					>
						<RouterLink
							:to="{name: 'task.detail', params: {id: task.id}}"
							class="matrix-task-title"
						>
							{{ task.title }}
						</RouterLink>
						<div class="matrix-task-meta">
							<span v-if="projectTitle(task)">{{ projectTitle(task) }}</span>
							<span>{{ priorityLabel(task) }}</span>
							<span v-if="dueLabel(task)">{{ dueLabel(task) }}</span>
						</div>
						<select
							v-if="canEdit(task)"
							class="matrix-task-move"
							:aria-label="$t('task.matrix.moveTo')"
							:value="quadrant"
							@change="onMoveSelect(task, $event)"
						>
							<option
								v-for="option in QUADRANTS"
								:key="option"
								:value="option"
							>
								{{ $t(`task.matrix.${option}`) }}
							</option>
						</select>
					</li>
				</ul>
				<p
					v-else
					class="quadrant-empty"
				>
					{{ $t('task.matrix.empty') }}
				</p>
			</section>
		</div>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watchEffect} from 'vue'
import {useI18n} from 'vue-i18n'

import {setTitle} from '@/helpers/setTitle'
import {formatDateShort} from '@/helpers/time/formatDate'
import {
	QUADRANTS,
	URGENT_WITHIN_DAYS,
	getQuadrant,
	quadrantChanges,
	type Quadrant,
} from '@/helpers/eisenhower'

import {useAuthStore} from '@/stores/auth'
import {useProjects} from '@/composables/useProjects'
import {useAllTasks} from '@/composables/useAllTasks'
import {useUpdateTaskMutation} from '@/client/queries/taskMutations'
import type {TaskResponse, TaskScope} from '@/client/queries/tasks'
import {PERMISSIONS} from '@/constants/permissions'

const {t} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const projectList = useProjects()
const updateTask = useUpdateTaskMutation(true)

const taskScope = computed<TaskScope>(() => ({
	project: null,
	params: {
		sort_by: ['priority', 'due_date', 'id'],
		order_by: ['desc', 'asc', 'desc'],
		filter: 'done = false',
		filter_include_nulls: false,
		filter_timezone: authStore.settings.timezone,
		q: '',
		expand: [],
	},
}))

const {tasks, isPending} = useAllTasks(taskScope, {enabled: () => authStore.authenticated})

const grouped = computed(() => {
	const now = new Date()
	const groups: Record<Quadrant, TaskResponse[]> = {do: [], schedule: [], delegate: [], eliminate: []}
	for (const task of tasks.value) {
		groups[getQuadrant(task, now)].push(task)
	}
	return groups
})

const PRIORITY_KEYS = ['unset', 'low', 'medium', 'high', 'urgent', 'doNow'] as const

function priorityLabel(task: TaskResponse) {
	const key = PRIORITY_KEYS[Math.min(Math.max(task.priority, 0), PRIORITY_KEYS.length - 1)]
	return t(`task.priority.${key}`)
}

function projectTitle(task: TaskResponse) {
	return projectList.projects[task.project_id]?.title ?? ''
}

function dueLabel(task: TaskResponse) {
	// A due date before 1970 is the API's "unset" value.
	if (!task.due_date || new Date(task.due_date).getFullYear() < 1970) return ''
	return formatDateShort(task.due_date)
}

function canEdit(task: TaskResponse) {
	return (projectList.projects[task.project_id]?.max_permission ?? 0) > PERMISSIONS.READ
}

function moveTask(task: TaskResponse, target: Quadrant) {
	if (!canEdit(task)) return
	const changes = quadrantChanges(task, target)
	if (Object.keys(changes).length === 0) return
	updateTask.mutate({id: task.id, ...changes})
}

const draggingId = ref<number | null>(null)
const overQuadrant = ref<Quadrant | null>(null)

function onDragStart(task: TaskResponse, event: DragEvent) {
	if (!canEdit(task)) {
		event.preventDefault()
		return
	}
	draggingId.value = task.id
	if (event.dataTransfer) {
		event.dataTransfer.effectAllowed = 'move'
		event.dataTransfer.setData('text/plain', String(task.id))
	}
}

function onDragEnd() {
	draggingId.value = null
	overQuadrant.value = null
}

function onDragLeave(quadrant: Quadrant) {
	if (overQuadrant.value === quadrant) overQuadrant.value = null
}

function onDrop(quadrant: Quadrant) {
	const task = tasks.value.find(item => item.id === draggingId.value)
	draggingId.value = null
	overQuadrant.value = null
	if (task) moveTask(task, quadrant)
}

// Keyboard and screen-reader alternative to dragging.
function onMoveSelect(task: TaskResponse, event: Event) {
	moveTask(task, (event.target as HTMLSelectElement).value as Quadrant)
}

watchEffect(() => setTitle(t('task.matrix.title')))
</script>

<style lang="scss" scoped>
.matrix-page {
	max-inline-size: 80rem;
}

.matrix-explanation {
	color: var(--grey-600);
}

.matrix {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 1rem;
	position: relative;
	padding-block-start: 1.5rem;

	&.is-loading {
		opacity: .6;
	}
}

.axis {
	position: absolute;
	inset-block-start: 0;
	font-size: .8rem;
	font-weight: 600;
	text-transform: uppercase;
	letter-spacing: .04em;
	color: var(--grey-600);
}

.axis-top-left {
	inset-inline-start: 0;
}

.axis-top-right {
	inset-inline-start: calc(50% + .5rem);
}

.quadrant {
	min-block-size: 14rem;
	padding: 1rem;
	border: 1px solid var(--grey-200);
	border-block-start-width: 4px;
	border-radius: 6px;
	background: var(--grey-100);
	transition: box-shadow .15s ease, border-color .15s ease;

	&.is-drop-target {
		box-shadow: 0 0 0 2px var(--primary);
	}
}

.quadrant-do { border-block-start-color: var(--danger); }
.quadrant-schedule { border-block-start-color: var(--primary); }
.quadrant-delegate { border-block-start-color: var(--warning); }
.quadrant-eliminate { border-block-start-color: var(--grey-500); }

.quadrant-header {
	margin-block-end: .75rem;
}

.quadrant-title {
	display: flex;
	align-items: center;
	gap: .5rem;
	margin: 0;
	font-size: 1.1rem;
	font-weight: 700;
}

.quadrant-count {
	font-size: .8rem;
	font-weight: 600;
	padding-inline: .5rem;
	border-radius: 1rem;
	background: var(--grey-200);
}

.quadrant-hint,
.quadrant-empty {
	margin: 0;
	font-size: .85rem;
	color: var(--grey-600);
}

.quadrant-tasks {
	display: flex;
	flex-direction: column;
	gap: .5rem;
	margin: 0;
	padding: 0;
	list-style: none;
}

.matrix-task {
	display: flex;
	flex-direction: column;
	gap: .25rem;
	padding: .5rem .75rem;
	border: 1px solid var(--grey-200);
	border-radius: 4px;
	background: var(--white);

	&[draggable='true'] {
		cursor: grab;
	}

	&.is-dragging {
		opacity: .4;
	}
}

.matrix-task-title {
	font-weight: 600;
	overflow-wrap: anywhere;
}

.matrix-task-meta {
	display: flex;
	flex-wrap: wrap;
	gap: .25rem .75rem;
	font-size: .8rem;
	color: var(--grey-600);
}

.matrix-task-move {
	align-self: flex-start;
	max-inline-size: 100%;
	font-size: .8rem;
}

@media (max-width: 768px) {
	.matrix {
		grid-template-columns: minmax(0, 1fr);
	}

	.axis {
		display: none;
	}
}
</style>
