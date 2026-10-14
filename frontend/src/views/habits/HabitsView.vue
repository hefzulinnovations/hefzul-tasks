<template>
	<div
		v-cy="'habitsView'"
		class="habits-page has-text-start"
	>
		<header class="habits-header">
			<h2 class="title">
				{{ $t('habits.title') }}
			</h2>
			<div class="habits-header-actions">
				<FormCheckbox
					v-model="showArchived"
					:label="$t('habits.showArchived')"
				/>
				<XButton
					v-cy="'habitNew'"
					icon="plus"
					@click="startCreate"
				>
					{{ $t('habits.new') }}
				</XButton>
			</div>
		</header>

		<Card
			v-if="creating"
			:title="$t('habits.new')"
			class="mbe-4"
		>
			<HabitForm
				:loading="createHabit.isPending.value"
				@submit="create"
				@cancel="creating = false"
			/>
		</Card>

		<p
			v-if="habits.length > 0"
			class="habits-summary"
		>
			{{ $t('habits.summary', {done: doneCount, due: dueCount}) }}
		</p>

		<ul
			v-if="habits.length > 0"
			class="habits-list"
		>
			<li
				v-for="habit in habits"
				:key="habit.id"
				class="habit-card"
				:class="{'is-archived': habit.archived, 'is-not-due': !habit.due_today && !habit.done_today}"
			>
				<div class="habit-row">
					<HabitCheckButton
						:habit="habit"
						@checkIn="checkIn(habit)"
					/>
					<button
						type="button"
						class="habit-main"
						:aria-expanded="expandedId === habit.id"
						@click="toggleExpanded(habit.id)"
					>
						<span class="habit-title">
							<span
								v-if="habit.icon"
								class="habit-icon"
								aria-hidden="true"
							>{{ habit.icon }}</span>
							{{ habit.title }}
						</span>
						<span class="habit-meta">
							{{ scheduleText(habit) }}
							<template v-if="habit.reminder_time"> · {{ $t('habits.remindAt', {time: habit.reminder_time}) }}</template>
						</span>
					</button>
					<span
						class="habit-streak"
						:title="$t('habits.currentStreak')"
					>
						<Icon icon="bolt" />
						{{ habit.current_streak }}
					</span>
					<BaseButton
						v-if="habit.today_count > 0"
						class="habit-undo"
						:aria-label="$t('habits.undo', {title: habit.title})"
						:title="$t('habits.undo', {title: habit.title})"
						@click="undo(habit)"
					>
						<Icon icon="undo" />
					</BaseButton>
					<BaseButton
						class="habit-edit"
						:aria-label="$t('habits.edit', {title: habit.title})"
						:title="$t('habits.edit', {title: habit.title})"
						@click="editingId = editingId === habit.id ? null : habit.id"
					>
						<Icon icon="pen" />
					</BaseButton>
				</div>

				<HabitForm
					v-if="editingId === habit.id"
					:habit="habit"
					:loading="updateHabit.isPending.value"
					class="habit-edit-form"
					@submit="input => save(habit, input)"
					@cancel="editingId = null"
					@delete="remove(habit)"
				/>
				<HabitHeatmap
					v-else-if="expandedId === habit.id"
					:habit="habit"
					class="habit-details"
				/>
			</li>
		</ul>

		<div
			v-else-if="!habitList.isPending.value && !creating"
			class="habits-empty"
		>
			<p>{{ $t('habits.empty') }}</p>
			<XButton
				icon="plus"
				@click="startCreate"
			>
				{{ $t('habits.new') }}
			</XButton>
		</div>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watchEffect} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Icon from '@/components/misc/Icon'
import FormCheckbox from '@/components/input/FormCheckbox.vue'
import HabitForm from '@/components/habits/HabitForm.vue'
import HabitHeatmap from '@/components/habits/HabitHeatmap.vue'
import HabitCheckButton from '@/components/habits/HabitCheckButton.vue'

import {setTitle} from '@/helpers/setTitle'
import {todayIn} from '@/composables/useFocusTimer'
import {useAuthStore} from '@/stores/auth'
import {
	habitsQuery,
	useCheckinHabitMutation,
	useCreateHabitMutation,
	useDeleteHabitMutation,
	useUndoCheckinMutation,
	useUpdateHabitMutation,
	type HabitInput,
	type HabitResponse,
} from '@/client/queries/habits'

const {t, locale} = useI18n({useScope: 'global'})
const authStore = useAuthStore()

const showArchived = ref(false)
const habitList = useQuery(computed(() => ({
	...habitsQuery(showArchived.value),
	enabled: authStore.authenticated,
})))
const habits = computed(() => habitList.data.value ?? [])

const doneCount = computed(() => habits.value.filter(h => !h.archived && h.done_today).length)
const dueCount = computed(() => habits.value.filter(h => !h.archived && (h.due_today || h.done_today)).length)

const creating = ref(false)
const editingId = ref<number | null>(null)
const expandedId = ref<number | null>(null)

const createHabit = useCreateHabitMutation()
const updateHabit = useUpdateHabitMutation()
const deleteHabit = useDeleteHabitMutation()
const checkinHabit = useCheckinHabitMutation()
const undoCheckin = useUndoCheckinMutation()

function startCreate() {
	editingId.value = null
	creating.value = true
}

async function create(input: HabitInput) {
	try {
		await createHabit.mutateAsync({...input, position: habits.value.length + 1})
		creating.value = false
	} catch {
		// The toast shows the error.
	}
}

async function save(habit: HabitResponse, input: HabitInput) {
	try {
		await updateHabit.mutateAsync({...input, id: habit.id})
		editingId.value = null
	} catch {
		// The toast shows the error.
	}
}

async function remove(habit: HabitResponse) {
	if (!window.confirm(t('habits.confirmDelete', {title: habit.title}))) return
	await deleteHabit.mutateAsync({id: habit.id}).catch(() => {})
	editingId.value = null
}

function checkIn(habit: HabitResponse) {
	checkinHabit.mutate({id: habit.id})
}

function undo(habit: HabitResponse) {
	undoCheckin.mutate({id: habit.id, date: todayIn(authStore.settings.timezone), today: true})
}

function toggleExpanded(id: number) {
	editingId.value = null
	expandedId.value = expandedId.value === id ? null : id
}

function scheduleText(habit: HabitResponse) {
	const target = habit.daily_target > 1 ? ` · ${t('habits.timesADay', {count: habit.daily_target}, habit.daily_target)}` : ''
	if (habit.schedule_kind === 'times_per_week') {
		return t('habits.schedule.timesPerWeekShort', {count: habit.times_per_week}, habit.times_per_week) + target
	}
	if (habit.schedule_kind === 'weekdays') {
		const fmt = new Intl.DateTimeFormat(locale.value, {weekday: 'short', timeZone: 'UTC'})
		const start = authStore.settings.week_start ?? 0
		const days = Array.from({length: 7}, (_, i) => (i + start) % 7)
			.filter(day => habit.weekdays & (1 << day))
			// 2023-01-01 was a Sunday.
			.map(day => fmt.format(new Date(Date.UTC(2023, 0, 1 + day))))
		return days.join(', ') + target
	}
	return t('habits.schedule.daily') + target
}

watchEffect(() => setTitle(t('habits.title')))
</script>

<style lang="scss" scoped>
.habits-page {
	max-inline-size: 48rem;
}

.habits-header {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: space-between;
	gap: .75rem;
	margin-block-end: 1rem;

	.title {
		margin: 0;
	}
}

.habits-header-actions {
	display: flex;
	align-items: center;
	gap: 1rem;
}

.habits-summary {
	margin-block-end: .75rem;
	color: var(--grey-600);
}

.habits-list {
	display: flex;
	flex-direction: column;
	gap: .5rem;
	margin: 0;
	padding: 0;
	list-style: none;
}

.habit-card {
	padding: .75rem;
	border: 1px solid var(--grey-200);
	border-radius: $radius;
	background: var(--white);

	&.is-archived,
	&.is-not-due {
		opacity: .65;
	}
}

.habit-row {
	display: flex;
	align-items: center;
	gap: .75rem;
}

.habit-main {
	display: flex;
	flex: 1;
	flex-direction: column;
	min-inline-size: 0;
	padding: 0;
	border: 0;
	background: transparent;
	color: var(--text);
	font: inherit;
	text-align: start;
	cursor: pointer;
}

.habit-title {
	font-weight: 600;
	overflow-wrap: anywhere;
}

.habit-icon {
	margin-inline-end: .25rem;
}

.habit-meta {
	font-size: .8rem;
	color: var(--grey-600);
}

.habit-streak {
	display: flex;
	align-items: center;
	gap: .25rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	color: var(--warning);
}

.habit-undo,
.habit-edit {
	padding: .25rem;
	color: var(--grey-500);
}

.habit-edit-form,
.habit-details {
	margin-block-start: .75rem;
	padding-block-start: .75rem;
	border-block-start: 1px solid var(--grey-200);
}

.habits-empty {
	padding: 2rem 0;
	text-align: center;
	color: var(--grey-600);
}
</style>
