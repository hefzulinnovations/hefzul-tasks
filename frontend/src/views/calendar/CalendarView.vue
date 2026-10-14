<template>
	<div
		v-cy="'calendarView'"
		class="calendar-page has-text-start"
	>
		<header class="calendar-toolbar">
			<div class="calendar-nav">
				<XButton
					variant="secondary"
					:shadow="false"
					@click="anchor = new Date()"
				>
					{{ $t('calendar.today') }}
				</XButton>
				<BaseButton
					class="calendar-step"
					:aria-label="$t('calendar.previous')"
					@click="anchor = stepAnchor(mode, anchor, -1)"
				>
					<Icon icon="chevron-left" />
				</BaseButton>
				<BaseButton
					class="calendar-step"
					:aria-label="$t('calendar.next')"
					@click="anchor = stepAnchor(mode, anchor, 1)"
				>
					<Icon icon="chevron-right" />
				</BaseButton>
				<h2 class="calendar-title">
					{{ title }}
				</h2>
			</div>
			<div class="calendar-options">
				<FormSelect
					v-model.number="projectFilter"
					:aria-label="$t('calendar.project')"
					:options="projectOptions"
				/>
				<div
					class="calendar-modes"
					role="tablist"
				>
					<button
						v-for="m in MODES"
						:key="m"
						type="button"
						role="tab"
						class="calendar-mode"
						:class="{'is-selected': mode === m}"
						:aria-selected="mode === m"
						@click="mode = m"
					>
						{{ $t(`calendar.mode.${m}`) }}
					</button>
				</div>
			</div>
		</header>

		<div class="calendar-body">
			<!-- Month -->
			<div
				v-if="mode === 'month'"
				class="calendar-month"
				:class="{'is-loading': isPending}"
			>
				<span
					v-for="name in weekdayNames"
					:key="name"
					class="calendar-weekday"
				>{{ name }}</span>
				<div
					v-for="day in days"
					:key="day.toISOString()"
					class="calendar-cell"
					:class="{
						'is-other-month': day.getMonth() !== anchor.getMonth(),
						'is-today': sameDay(day, today),
						'is-drop-target': dropKey === dayKey(day),
					}"
					@dragover.prevent="dropKey = dayKey(day)"
					@dragleave="dropKey === dayKey(day) && (dropKey = '')"
					@drop.prevent="dropOnDay(day)"
				>
					<button
						type="button"
						class="calendar-cell-date"
						:aria-label="formatDay(day)"
						@click="openDay(day)"
					>
						{{ day.getDate() }}
					</button>
					<button
						v-for="task in tasksOnDay(day).slice(0, MONTH_CHIPS)"
						:key="task.id"
						type="button"
						class="calendar-chip"
						:class="{'is-done': task.done}"
						:style="taskStyle(task)"
						:draggable="canEdit(task)"
						@dragstart="dragStart(task, $event)"
						@dragend="dropKey = ''"
						@click="openEditor(task)"
					>
						<span
							v-if="!isAllDay(task)"
							class="calendar-chip-time"
						>{{ formatTime(taskSpan(task)!.start) }}</span>
						{{ task.title }}
					</button>
					<button
						v-if="tasksOnDay(day).length > MONTH_CHIPS"
						type="button"
						class="calendar-more"
						@click="openDay(day)"
					>
						{{ $t('calendar.more', {count: tasksOnDay(day).length - MONTH_CHIPS}) }}
					</button>
				</div>
			</div>

			<!-- Week and day -->
			<div
				v-else
				class="calendar-grid"
				:class="{'is-loading': isPending}"
				:style="{'--days': days.length}"
			>
				<div class="calendar-grid-head">
					<span class="calendar-gutter" />
					<button
						v-for="day in days"
						:key="day.toISOString()"
						type="button"
						class="calendar-grid-day"
						:class="{'is-today': sameDay(day, today)}"
						@click="openDay(day)"
					>
						{{ formatDayShort(day) }}
					</button>
				</div>
				<div class="calendar-allday">
					<span class="calendar-gutter calendar-allday-label">{{ $t('calendar.allDay') }}</span>
					<div
						v-for="day in days"
						:key="day.toISOString()"
						class="calendar-allday-cell"
						:class="{'is-drop-target': dropKey === `all-${dayKey(day)}`}"
						@dragover.prevent="dropKey = `all-${dayKey(day)}`"
						@dragleave="dropKey === `all-${dayKey(day)}` && (dropKey = '')"
						@drop.prevent="dropOnDay(day, true)"
					>
						<button
							v-for="task in allDayTasks(day)"
							:key="task.id"
							type="button"
							class="calendar-chip"
							:class="{'is-done': task.done}"
							:style="taskStyle(task)"
							:draggable="canEdit(task)"
							@dragstart="dragStart(task, $event)"
							@click="openEditor(task)"
						>
							{{ task.title }}
						</button>
					</div>
				</div>
				<div
					ref="scroller"
					class="calendar-grid-scroll"
				>
					<div class="calendar-grid-body">
						<div class="calendar-hours">
							<span
								v-for="hour in 24"
								:key="hour"
								class="calendar-hour"
							>{{ formatHour(hour - 1) }}</span>
						</div>
						<div
							v-for="day in days"
							:key="day.toISOString()"
							class="calendar-column"
							:class="{'is-today': sameDay(day, today)}"
							@dragover.prevent="dragOverColumn(day, $event)"
							@dragleave="dropKey = ''"
							@drop.prevent="dropOnColumn(day, $event)"
						>
							<span
								v-if="dropSlot && dropKey === `col-${dayKey(day)}`"
								class="calendar-drop-slot"
								:style="{insetBlockStart: `${dropSlot * PX_PER_MINUTE}px`, blockSize: `${SLOT_MINUTES * PX_PER_MINUTE}px`}"
							/>
							<span
								v-if="sameDay(day, today)"
								class="calendar-now"
								:style="{insetBlockStart: `${nowMinutes * PX_PER_MINUTE}px`}"
							/>
							<button
								v-for="placed in timedLayout(day)"
								:key="placed.task.id"
								type="button"
								class="calendar-event"
								:class="{'is-done': placed.task.done}"
								:style="{
									...taskStyle(placed.task),
									insetBlockStart: `${placed.top * PX_PER_MINUTE}px`,
									blockSize: `${placed.height * PX_PER_MINUTE - 2}px`,
									insetInlineStart: `${(placed.column / placed.columns) * 100}%`,
									inlineSize: `calc(${100 / placed.columns}% - 4px)`,
								}"
								:title="`${formatTime(taskSpan(placed.task)!.start)} ${placed.task.title}`"
								:draggable="canEdit(placed.task)"
								@dragstart="dragStart(placed.task, $event)"
								@click="openEditor(placed.task)"
							>
								<span class="calendar-event-time">{{ formatTime(taskSpan(placed.task)!.start) }}</span>
								<span class="calendar-event-title">{{ placed.task.title }}</span>
							</button>
						</div>
					</div>
				</div>
			</div>

			<aside class="calendar-unscheduled">
				<h3 class="calendar-unscheduled-title">
					{{ $t('calendar.unscheduled') }}
				</h3>
				<p class="calendar-hint">
					{{ $t('calendar.dragHint') }}
				</p>
				<ul>
					<li
						v-for="task in unscheduled"
						:key="task.id"
					>
						<button
							type="button"
							class="calendar-chip"
							:style="taskStyle(task)"
							:draggable="canEdit(task)"
							@dragstart="dragStart(task, $event)"
							@click="openEditor(task)"
						>
							{{ task.title }}
						</button>
					</li>
				</ul>
				<p
					v-if="unscheduled.length === 0"
					class="calendar-hint"
				>
					{{ $t('calendar.noUnscheduled') }}
				</p>
			</aside>
		</div>

		<Modal
			v-if="editing"
			variant="hint-modal"
			@close="editing = null"
		>
			<Card
				class="has-no-shadow"
				:title="editing.title"
				:show-close="true"
				@close="editing = null"
			>
				<form
					class="calendar-editor"
					@submit.prevent="saveEditor"
				>
					<FormField :label="$t('calendar.dueAt')">
						<FormInput
							v-model="editorValue"
							v-focus
							type="datetime-local"
							:disabled="!canEdit(editing)"
						/>
					</FormField>
					<p
						v-if="!canEdit(editing)"
						class="calendar-hint"
					>
						{{ $t('task.matrix.readOnly') }}
					</p>
					<div class="calendar-editor-actions">
						<RouterLink :to="{name: 'task.detail', params: {id: editing.id}}">
							{{ $t('calendar.openTask') }}
						</RouterLink>
						<XButton
							v-if="canEdit(editing) && hasDate(editing)"
							variant="secondary"
							type="button"
							@click="clearDate"
						>
							{{ $t('calendar.removeDate') }}
						</XButton>
						<XButton
							v-if="canEdit(editing)"
							type="submit"
							:disabled="!editorValue"
						>
							{{ $t('misc.save') }}
						</XButton>
					</div>
				</form>
			</Card>
		</Modal>
	</div>
</template>

<script setup lang="ts">
import {computed, nextTick, onMounted, ref, watch, watchEffect} from 'vue'
import {useI18n} from 'vue-i18n'
import {useNow} from '@vueuse/core'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Icon from '@/components/misc/Icon'
import Modal from '@/components/misc/Modal.vue'
import FormField from '@/components/input/FormField.vue'
import FormInput from '@/components/input/FormInput.vue'
import FormSelect from '@/components/input/FormSelect.vue'

import {setTitle} from '@/helpers/setTitle'
import {
	SLOT_MINUTES,
	addDays,
	isAllDay,
	layoutDay,
	moveChanges,
	onDay,
	parseTaskDate,
	sameDay,
	slotTime,
	startOfDay,
	stepAnchor,
	taskSpan,
	visibleDays,
	type CalendarMode,
} from '@/helpers/calendarView'
import {useAuthStore} from '@/stores/auth'
import {useProjects} from '@/composables/useProjects'
import {useAllTasks} from '@/composables/useAllTasks'
import {useTasks} from '@/composables/useTasks'
import {useUpdateTaskMutation} from '@/client/queries/taskMutations'
import {isSavedFilterProject} from '@/client/queries/projects'
import type {TaskResponse, TaskScope} from '@/client/queries/tasks'
import {PERMISSIONS} from '@/constants/permissions'

const MODES: readonly CalendarMode[] = ['day', 'week', 'month']
const MONTH_CHIPS = 3
const PX_PER_MINUTE = 0.8

const {t, locale} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const projectList = useProjects()
const updateTask = useUpdateTaskMutation(true)

const STORAGE_KEY = 'hefzul.calendar.mode'
function storedMode(): CalendarMode {
	try {
		const value = localStorage.getItem(STORAGE_KEY)
		return MODES.includes(value as CalendarMode) ? value as CalendarMode : 'week'
	} catch {
		return 'week'
	}
}
const mode = ref<CalendarMode>(storedMode())
watch(mode, value => {
	try {
		localStorage.setItem(STORAGE_KEY, value)
	} catch {
		// Remembering the mode is a convenience.
	}
})

const anchor = ref(new Date())
const now = useNow({interval: 60_000})
const today = computed(() => startOfDay(now.value))
const nowMinutes = computed(() => now.value.getHours() * 60 + now.value.getMinutes())
const weekStart = computed(() => authStore.settings.week_start ?? 0)
const days = computed(() => visibleDays(mode.value, anchor.value, weekStart.value))

const projectFilter = ref(0)
const projectOptions = computed(() => [
	{value: 0, label: t('task.matrix.allProjects')},
	...projectList.projectsArray
		.filter(project => project.id > 0 && !project.is_archived && !isSavedFilterProject(project))
		.map(project => ({value: project.id, label: project.title}))
		.sort((a, b) => a.label.localeCompare(b.label)),
])

const range = computed(() => {
	const first = days.value[0]
	return {from: first.toISOString(), to: addDays(days.value[days.value.length - 1], 1).toISOString()}
})

const scope = computed<TaskScope>(() => ({
	project: null,
	params: {
		sort_by: ['due_date', 'id'],
		order_by: ['asc', 'asc'],
		filter: `((due_date >= '${range.value.from}' && due_date < '${range.value.to}') || (start_date < '${range.value.to}' && end_date >= '${range.value.from}'))`,
		filter_include_nulls: false,
		filter_timezone: authStore.settings.timezone,
		q: '',
		expand: [],
	},
}))
const {tasks, isPending, refetch} = useAllTasks(scope, {enabled: () => authStore.authenticated})

const openScope = computed<TaskScope>(() => ({
	project: null,
	params: {
		sort_by: ['priority', 'id'],
		order_by: ['desc', 'desc'],
		filter: 'done = false',
		filter_include_nulls: false,
		filter_timezone: authStore.settings.timezone,
		q: '',
		expand: [],
	},
}))
const openTasks = useTasks(openScope, {enabled: () => authStore.authenticated})

const matchesProject = (task: TaskResponse) => projectFilter.value === 0 || task.project_id === projectFilter.value
const visibleTasks = computed(() => tasks.value.filter(matchesProject))
const unscheduled = computed(() => openTasks.tasks.value
	.filter(task => matchesProject(task) && taskSpan(task) === null)
	.slice(0, 30))

function dayKey(day: Date) {
	return `${day.getFullYear()}-${day.getMonth()}-${day.getDate()}`
}

function tasksOnDay(day: Date) {
	return visibleTasks.value.filter(task => onDay(task, day))
}

function allDayTasks(day: Date) {
	return tasksOnDay(day).filter(isAllDay)
}

function timedLayout(day: Date) {
	return layoutDay(tasksOnDay(day).filter(task => !isAllDay(task)), day)
}

function canEdit(task: TaskResponse) {
	return (projectList.projects[task.project_id]?.max_permission ?? 0) > PERMISSIONS.READ
}

function hasDate(task: TaskResponse) {
	return taskSpan(task) !== null
}

function taskStyle(task: TaskResponse) {
	const color = task.hex_color || projectList.projects[task.project_id]?.hex_color
	return color ? {'--event-color': `#${color.replace('#', '')}`} : {}
}

// Formatting
const title = computed(() => {
	const first = days.value[0]
	if (mode.value === 'month') {
		return new Intl.DateTimeFormat(locale.value, {month: 'long', year: 'numeric'}).format(anchor.value)
	}
	if (mode.value === 'day') {
		return new Intl.DateTimeFormat(locale.value, {dateStyle: 'full'}).format(first)
	}
	const last = days.value[days.value.length - 1]
	const fmt = new Intl.DateTimeFormat(locale.value, {day: 'numeric', month: 'short', year: 'numeric'})
	return fmt.formatRange ? fmt.formatRange(first, last) : `${fmt.format(first)} – ${fmt.format(last)}`
})
const weekdayNames = computed(() => {
	const fmt = new Intl.DateTimeFormat(locale.value, {weekday: 'short'})
	return days.value.slice(0, 7).map(day => fmt.format(day))
})
const formatDay = (day: Date) => new Intl.DateTimeFormat(locale.value, {dateStyle: 'full'}).format(day)
const formatDayShort = (day: Date) => new Intl.DateTimeFormat(locale.value, {weekday: 'short', day: 'numeric'}).format(day)
const formatTime = (date: Date) => new Intl.DateTimeFormat(locale.value, {hour: 'numeric', minute: '2-digit'}).format(date)
const formatHour = (hour: number) => new Intl.DateTimeFormat(locale.value, {hour: 'numeric'}).format(new Date(2026, 0, 1, hour))

function openDay(day: Date) {
	anchor.value = day
	mode.value = 'day'
}

// Dragging
const draggingId = ref<number | null>(null)
const dropKey = ref('')
const dropSlot = ref<number | null>(null)

function dragStart(task: TaskResponse, event: DragEvent) {
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

function draggedTask(): TaskResponse | undefined {
	const id = draggingId.value
	draggingId.value = null
	dropKey.value = ''
	dropSlot.value = null
	return [...tasks.value, ...openTasks.tasks.value].find(task => task.id === id)
}

function minutesAt(event: DragEvent) {
	const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
	return (event.clientY - rect.top) / PX_PER_MINUTE
}

function dragOverColumn(day: Date, event: DragEvent) {
	dropKey.value = `col-${dayKey(day)}`
	const minutes = minutesAt(event)
	dropSlot.value = Math.max(0, Math.floor(minutes / SLOT_MINUTES) * SLOT_MINUTES)
}

function move(task: TaskResponse, target: Date, keepTimeOfDay: boolean) {
	if (!canEdit(task)) return
	// A task dragged in from "No date yet" is not in the calendar's list yet.
	updateTask.mutate({id: task.id, ...moveChanges(task, target, keepTimeOfDay)}, {
		onSettled: () => {
			refetch()
			openTasks.refetch()
		},
	})
}

function dropOnColumn(day: Date, event: DragEvent) {
	const minutes = minutesAt(event)
	const task = draggedTask()
	if (task) move(task, slotTime(day, minutes), false)
}

function dropOnDay(day: Date, allDay = false) {
	const task = draggedTask()
	if (!task) return
	// A task without a time keeps none: it lands at midnight, in the all-day row.
	move(task, startOfDay(day), !allDay && taskSpan(task) !== null)
}

// Keyboard alternative to dragging: a dialog with a date and time field.
const editing = ref<TaskResponse | null>(null)
const editorValue = ref('')

function toLocalInput(date: Date) {
	const pad = (n: number) => String(n).padStart(2, '0')
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function openEditor(task: TaskResponse) {
	editing.value = task
	const start = taskSpan(task)?.start ?? (() => {
		const d = startOfDay(anchor.value)
		d.setHours(9)
		return d
	})()
	editorValue.value = toLocalInput(start)
}

function saveEditor() {
	if (!editing.value || !editorValue.value) return
	const target = new Date(editorValue.value)
	if (Number.isNaN(target.getTime())) return
	move(editing.value, target, false)
	editing.value = null
}

function clearDate() {
	if (!editing.value) return
	// An empty date is sent as the API's "unset" zero date.
	const changes: Record<string, string> = {due_date: ''}
	if (parseTaskDate(editing.value.start_date)) changes.start_date = ''
	if (parseTaskDate(editing.value.end_date)) changes.end_date = ''
	updateTask.mutate({id: editing.value.id, ...changes}, {
		onSettled: () => {
			refetch()
			openTasks.refetch()
		},
	})
	editing.value = null
}

// Start the time grid at 7:00 instead of midnight.
const scroller = ref<HTMLElement | null>(null)
async function scrollToMorning() {
	await nextTick()
	if (scroller.value) scroller.value.scrollTop = 7 * 60 * PX_PER_MINUTE
}
onMounted(scrollToMorning)
watch(mode, scrollToMorning)

watchEffect(() => setTitle(t('calendar.title')))
</script>

<style lang="scss" scoped>
.calendar-page {
	--event-color: var(--primary);
}

.calendar-toolbar {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: space-between;
	gap: .75rem;
	margin-block-end: 1rem;
}

.calendar-nav,
.calendar-options {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .5rem;
}

.calendar-title {
	margin: 0 0 0 .5rem;
	font-size: 1.25rem;
}

.calendar-step {
	padding: .4rem .6rem;
}

.calendar-modes {
	display: inline-flex;
	border: 1px solid var(--grey-300);
	border-radius: $radius;
	overflow: hidden;
}

.calendar-mode {
	padding: .35rem .75rem;
	border: 0;
	background: var(--white);
	color: var(--text);
	font: inherit;
	font-size: .9rem;
	cursor: pointer;

	&.is-selected {
		background: var(--primary);
		color: #ffffff;
	}
}

.calendar-body {
	display: grid;
	grid-template-columns: minmax(0, 1fr) 14rem;
	gap: 1rem;
	align-items: start;

	@media (width <= 900px) {
		grid-template-columns: minmax(0, 1fr);
	}
}

.is-loading {
	opacity: .6;
}

// Month
.calendar-month {
	display: grid;
	grid-template-columns: repeat(7, minmax(0, 1fr));
	border: 1px solid var(--grey-200);
	border-radius: $radius;
	background: var(--white);
	overflow: hidden;
}

.calendar-weekday {
	padding: .35rem;
	border-block-end: 1px solid var(--grey-200);
	font-size: .75rem;
	font-weight: 600;
	text-align: center;
	color: var(--grey-600);
}

.calendar-cell {
	display: flex;
	flex-direction: column;
	gap: 2px;
	min-block-size: 6.5rem;
	padding: .25rem;
	border-inline-end: 1px solid var(--grey-100);
	border-block-end: 1px solid var(--grey-100);
	min-inline-size: 0;

	&.is-other-month {
		background: var(--grey-50);

		.calendar-cell-date {
			color: var(--grey-400);
		}
	}

	&.is-drop-target {
		box-shadow: inset 0 0 0 2px var(--primary);
	}

	&.is-today .calendar-cell-date {
		background: var(--primary);
		color: #ffffff;
	}
}

.calendar-cell-date {
	align-self: flex-start;
	min-inline-size: 1.6rem;
	padding: 0 .3rem;
	border: 0;
	border-radius: 1rem;
	background: transparent;
	color: var(--text);
	font: inherit;
	font-size: .8rem;
	cursor: pointer;
}

.calendar-chip {
	display: block;
	inline-size: 100%;
	padding: 1px .35rem;
	border: 0;
	border-inline-start: 3px solid var(--event-color);
	border-radius: 3px;
	background: color-mix(in srgb, var(--event-color) 14%, var(--white));
	color: var(--text);
	font: inherit;
	font-size: .75rem;
	text-align: start;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
	cursor: pointer;

	&[draggable='true'] {
		cursor: grab;
	}

	&.is-done {
		text-decoration: line-through;
		opacity: .6;
	}
}

.calendar-chip-time {
	font-weight: 600;
}

.calendar-more {
	padding: 0 .35rem;
	border: 0;
	background: transparent;
	color: var(--grey-600);
	font: inherit;
	font-size: .7rem;
	text-align: start;
	cursor: pointer;
}

// Week and day
.calendar-grid {
	--gutter: 3.5rem;

	border: 1px solid var(--grey-200);
	border-radius: $radius;
	background: var(--white);
	overflow: hidden;
}

.calendar-grid-head,
.calendar-allday,
.calendar-grid-body {
	display: grid;
	grid-template-columns: var(--gutter) repeat(var(--days), minmax(0, 1fr));
}

.calendar-grid-head {
	border-block-end: 1px solid var(--grey-200);
}

.calendar-grid-day {
	padding: .4rem;
	border: 0;
	background: transparent;
	color: var(--text);
	font: inherit;
	font-size: .85rem;
	font-weight: 600;
	cursor: pointer;

	&.is-today {
		color: var(--primary);
	}
}

.calendar-allday {
	border-block-end: 1px solid var(--grey-200);
}

.calendar-allday-label {
	padding: .25rem;
	font-size: .7rem;
	color: var(--grey-500);
}

.calendar-allday-cell {
	display: flex;
	flex-direction: column;
	gap: 2px;
	min-block-size: 1.75rem;
	padding: 2px;
	border-inline-start: 1px solid var(--grey-100);
	min-inline-size: 0;

	&.is-drop-target {
		box-shadow: inset 0 0 0 2px var(--primary);
	}
}

.calendar-grid-scroll {
	max-block-size: 65vh;
	overflow-y: auto;
}

.calendar-grid-body {
	position: relative;
}

.calendar-hour {
	display: block;
	block-size: calc(60px * .8);
	padding-inline-end: .35rem;
	font-size: .7rem;
	text-align: end;
	color: var(--grey-500);
	transform: translateY(-.45em);
}

.calendar-column {
	position: relative;
	block-size: calc(24 * 60px * .8);
	border-inline-start: 1px solid var(--grey-100);
	background-image: repeating-linear-gradient(to bottom, var(--grey-100) 0 1px, transparent 1px calc(60px * .8));
	min-inline-size: 0;

	&.is-today {
		background-color: color-mix(in srgb, var(--primary) 4%, transparent);
	}
}

.calendar-drop-slot {
	position: absolute;
	inset-inline: 2px;
	border-radius: 3px;
	background: color-mix(in srgb, var(--primary) 25%, transparent);
	pointer-events: none;
}

.calendar-now {
	position: absolute;
	inset-inline: 0;
	z-index: 2;
	border-block-start: 2px solid var(--danger);
	pointer-events: none;
}

.calendar-event {
	position: absolute;
	display: block;
	min-block-size: 1rem;
	padding: 1px .3rem;
	border: 0;
	border-inline-start: 3px solid var(--event-color);
	border-radius: 3px;
	background: color-mix(in srgb, var(--event-color) 18%, var(--white));
	color: var(--text);
	font: inherit;
	font-size: .72rem;
	text-align: start;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
	cursor: pointer;

	&[draggable='true'] {
		cursor: grab;
	}

	&.is-done {
		opacity: .55;
		text-decoration: line-through;
	}
}

.calendar-event-time {
	margin-inline-end: .3rem;
	font-weight: 600;
}

// Unscheduled list
.calendar-unscheduled {
	padding: .75rem;
	border: 1px solid var(--grey-200);
	border-radius: $radius;
	background: var(--white);

	ul {
		display: flex;
		flex-direction: column;
		gap: 3px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
}

.calendar-unscheduled-title {
	margin: 0 0 .25rem;
	font-size: .95rem;
}

.calendar-hint {
	margin: 0 0 .5rem;
	font-size: .8rem;
	color: var(--grey-600);
}

.calendar-editor {
	display: flex;
	flex-direction: column;
	gap: .5rem;
}

.calendar-editor-actions {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: flex-end;
	gap: .75rem;

	a {
		margin-inline-end: auto;
	}
}
</style>
