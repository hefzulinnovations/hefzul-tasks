<template>
	<div
		v-cy="'focusView'"
		class="focus-page has-text-start"
	>
		<h2 class="title mbe-4">
			{{ $t('focus.title') }}
		</h2>

		<div class="focus-layout">
			<Card class="focus-timer-card">
				<div
					class="focus-timer"
					:class="`is-${displayKind}`"
				>
					<svg
						class="focus-ring"
						viewBox="0 0 120 120"
						aria-hidden="true"
					>
						<circle
							class="focus-ring-track"
							cx="60"
							cy="60"
							r="54"
						/>
						<circle
							class="focus-ring-progress"
							cx="60"
							cy="60"
							r="54"
							:stroke-dasharray="RING_LENGTH"
							:stroke-dashoffset="RING_LENGTH * (1 - ringProgress)"
						/>
					</svg>
					<div class="focus-clock-wrap">
						<p class="focus-kind">
							{{ $t(`focus.kind.${displayKind}`) }}
						</p>
						<p
							class="focus-clock"
							role="timer"
							:aria-label="$t('focus.timeLeft', {time: clock})"
						>
							{{ clock }}
						</p>
						<p
							v-if="timer.isPaused.value"
							class="focus-state"
						>
							{{ $t('focus.paused') }}
						</p>
					</div>
				</div>

				<p
					v-if="timer.current.value && runningTaskTitle"
					class="focus-task has-text-centered"
				>
					<RouterLink :to="{name: 'task.detail', params: {id: timer.current.value.task_id}}">
						{{ runningTaskTitle }}
					</RouterLink>
				</p>

				<div
					v-if="timer.current.value"
					class="focus-controls"
				>
					<XButton
						v-if="timer.isRunning.value"
						v-cy="'focusPause'"
						icon="pause"
						:loading="timer.busy.value"
						@click="timer.pause()"
					>
						{{ $t('focus.pause') }}
					</XButton>
					<XButton
						v-else
						v-cy="'focusResume'"
						icon="play"
						:loading="timer.busy.value"
						@click="timer.resume()"
					>
						{{ $t('focus.resume') }}
					</XButton>
					<XButton
						v-cy="'focusStop'"
						variant="secondary"
						:icon="timer.current.value.kind === 'focus' ? 'stop' : 'forward'"
						:disabled="timer.busy.value"
						@click="timer.abandon()"
					>
						{{ timer.current.value.kind === 'focus' ? $t('focus.stop') : $t('focus.skipBreak') }}
					</XButton>
				</div>

				<div
					v-else
					class="focus-start"
				>
					<p
						v-if="timer.suggestedNext.value"
						class="focus-suggestion has-text-centered"
					>
						{{ timer.suggestedNext.value === 'focus' ? $t('focus.breakOver') : $t('focus.wellDone') }}
					</p>

					<div
						class="focus-kind-picker"
						role="radiogroup"
						:aria-label="$t('focus.sessionType')"
					>
						<button
							v-for="kind in KINDS"
							:key="kind"
							type="button"
							role="radio"
							class="focus-kind-option"
							:class="{'is-selected': selectedKind === kind}"
							:aria-checked="selectedKind === kind"
							@click="selectedKind = kind"
						>
							{{ $t(`focus.kind.${kind}`) }}
							<span class="focus-kind-minutes">{{ $t('focus.minutes', {count: minutesFor(kind)}) }}</span>
						</button>
					</div>

					<label
						v-if="selectedKind === 'focus'"
						class="focus-task-picker"
					>
						<span>{{ $t('focus.task') }}</span>
						<FormSelect
							v-model.number="selectedTaskId"
							v-cy="'focusTaskSelect'"
							:options="taskOptions"
						/>
					</label>

					<XButton
						v-cy="'focusStart'"
						class="focus-start-button"
						icon="play"
						:loading="timer.busy.value"
						@click="startSelected"
					>
						{{ $t('focus.start', {kind: $t(`focus.kind.${selectedKind}`)}) }}
					</XButton>
				</div>
			</Card>

			<Card
				:title="$t('focus.settings.title')"
				class="focus-settings-card"
			>
				<form
					class="focus-settings"
					@submit.prevent="saveSettings"
				>
					<FormField
						v-for="field in MINUTE_FIELDS"
						:key="field.key"
						:label="$t(field.label)"
					>
						<FormInput
							v-model.number="draft[field.key]"
							type="number"
							:min="field.min"
							:max="field.max"
							required
						/>
					</FormField>
					<FormField :label="$t('focus.settings.sound')">
						<FormSelect
							v-model="draft.sound"
							:options="soundOptions"
						/>
					</FormField>
					<FormCheckbox
						v-model="draft.auto_start_breaks"
						:label="$t('focus.settings.autoStartBreaks')"
					/>
					<div class="focus-settings-actions">
						<XButton
							variant="secondary"
							:shadow="false"
							type="button"
							@click="playFocusSound(draft.sound)"
						>
							{{ $t('focus.settings.testSound') }}
						</XButton>
						<XButton
							v-cy="'focusSaveSettings'"
							:loading="updateSettings.isPending.value"
							:disabled="!settingsChanged"
							type="submit"
						>
							{{ $t('misc.save') }}
						</XButton>
					</div>
				</form>
			</Card>
		</div>

		<FocusStatsPanel class="mbs-4" />
	</div>
</template>

<script setup lang="ts">
import {computed, reactive, ref, watch, watchEffect} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute} from 'vue-router'
import {useQuery} from '@tanstack/vue-query'

import Card from '@/components/misc/Card.vue'
import FormField from '@/components/input/FormField.vue'
import FormInput from '@/components/input/FormInput.vue'
import FormSelect from '@/components/input/FormSelect.vue'
import FormCheckbox from '@/components/input/FormCheckbox.vue'
import FocusStatsPanel from './FocusStatsPanel.vue'

import {setTitle} from '@/helpers/setTitle'
import {formatClock} from '@/helpers/focusTimer'
import {FOCUS_SOUNDS, playFocusSound} from '@/helpers/focusSound'
import {useFocusTimer} from '@/composables/useFocusTimer'
import {useTasks} from '@/composables/useTasks'
import {useAuthStore} from '@/stores/auth'
import {taskQuery} from '@/client/queries/tasks'
import {
	DEFAULT_FOCUS_SETTINGS,
	useUpdateFocusSettingsMutation,
	type FocusKind,
	type FocusSettingsResponse,
} from '@/client/queries/focus'

const KINDS: readonly FocusKind[] = ['focus', 'short_break', 'long_break']
const RING_LENGTH = 2 * Math.PI * 54

type MinuteKey = 'focus_minutes' | 'short_break_minutes' | 'long_break_minutes' | 'sessions_before_long_break'
const MINUTE_FIELDS: {key: MinuteKey, label: string, min: number, max: number}[] = [
	{key: 'focus_minutes', label: 'focus.settings.focusMinutes', min: 1, max: 180},
	{key: 'short_break_minutes', label: 'focus.settings.shortBreakMinutes', min: 1, max: 60},
	{key: 'long_break_minutes', label: 'focus.settings.longBreakMinutes', min: 1, max: 120},
	{key: 'sessions_before_long_break', label: 'focus.settings.sessionsBeforeLongBreak', min: 1, max: 12},
]

const {t} = useI18n({useScope: 'global'})
const route = useRoute()
const authStore = useAuthStore()
const timer = useFocusTimer()

const settings = computed<FocusSettingsResponse>(() => timer.settings.value ?? DEFAULT_FOCUS_SETTINGS)

const selectedKind = ref<FocusKind>('focus')
watch(timer.suggestedNext, next => {
	if (next) selectedKind.value = next
})

const queryTaskId = Number(route.query.task)
const selectedTaskId = ref(Number.isInteger(queryTaskId) && queryTaskId > 0 ? queryTaskId : 0)

const openTasks = useTasks(() => ({
	params: {
		sort_by: ['priority', 'due_date', 'id'],
		order_by: ['desc', 'asc', 'desc'],
		filter: 'done = false',
		filter_include_nulls: false,
		filter_timezone: authStore.settings.timezone,
		q: '',
		expand: [],
	},
}), {enabled: () => authStore.authenticated})

// The task linked to the running session may not be on the first page of open tasks.
const runningTaskId = computed(() => timer.current.value?.task_id ?? 0)
const runningTaskQuery = useQuery(computed(() => ({
	...taskQuery(runningTaskId.value),
	enabled: runningTaskId.value > 0,
})))
const runningTaskTitle = computed(() => runningTaskId.value > 0 ? runningTaskQuery.data.value?.title ?? '' : '')

const preselectedTaskQuery = useQuery(computed(() => ({
	...taskQuery(selectedTaskId.value),
	enabled: selectedTaskId.value > 0,
})))

const taskOptions = computed(() => {
	const options = [{value: 0, label: t('focus.noTask')}]
	const seen = new Set<number>()
	const preselected = preselectedTaskQuery.data.value
	if (preselected?.id) {
		options.push({value: preselected.id, label: preselected.title})
		seen.add(preselected.id)
	}
	for (const task of openTasks.tasks.value) {
		if (seen.has(task.id)) continue
		options.push({value: task.id, label: task.title})
	}
	return options
})

function minutesFor(kind: FocusKind) {
	if (kind === 'short_break') return settings.value.short_break_minutes
	if (kind === 'long_break') return settings.value.long_break_minutes
	return settings.value.focus_minutes
}

const displayKind = computed<FocusKind>(() => timer.current.value?.kind ?? selectedKind.value)
const clock = computed(() => formatClock(timer.current.value ? timer.remaining.value : minutesFor(selectedKind.value) * 60))
const ringProgress = computed(() => {
	const current = timer.current.value
	if (!current) return 0
	return current.planned_seconds > 0 ? 1 - timer.remaining.value / current.planned_seconds : 1
})

async function startSelected() {
	const taskId = selectedKind.value === 'focus' ? selectedTaskId.value : 0
	await timer.start(selectedKind.value, taskId).catch(() => {})
}

const draft = reactive<FocusSettingsResponse>({...DEFAULT_FOCUS_SETTINGS})
watch(settings, value => Object.assign(draft, value), {immediate: true})
const settingsChanged = computed(() => (Object.keys(draft) as (keyof FocusSettingsResponse)[])
	.some(key => draft[key] !== settings.value[key]))

const soundOptions = computed(() => FOCUS_SOUNDS.map(sound => ({value: sound, label: t(`focus.settings.sounds.${sound}`)})))

const updateSettings = useUpdateFocusSettingsMutation()
function saveSettings() {
	updateSettings.mutate({...draft})
}

watchEffect(() => setTitle(timer.current.value ? `${clock.value} · ${t('focus.title')}` : t('focus.title')))
</script>

<style lang="scss" scoped>
.focus-page {
	max-inline-size: 64rem;
}

.focus-layout {
	display: grid;
	grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
	gap: 1rem;
	align-items: start;

	@media (width <= 768px) {
		grid-template-columns: minmax(0, 1fr);
	}
}

.focus-timer {
	--focus-color: var(--danger);

	position: relative;
	inline-size: min(16rem, 100%);
	margin-inline: auto;
	aspect-ratio: 1;

	&.is-short_break { --focus-color: var(--success); }
	&.is-long_break { --focus-color: var(--primary); }
}

.focus-ring {
	inline-size: 100%;
	block-size: 100%;
	transform: rotate(-90deg);
}

.focus-ring-track,
.focus-ring-progress {
	fill: none;
	stroke-width: 6;
}

.focus-ring-track {
	stroke: var(--grey-200);
}

.focus-ring-progress {
	stroke: var(--focus-color);
	stroke-linecap: round;
	transition: stroke-dashoffset 1s linear;
}

.focus-clock-wrap {
	position: absolute;
	inset: 0;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
}

.focus-kind,
.focus-state {
	margin: 0;
	font-size: .85rem;
	font-weight: 600;
	text-transform: uppercase;
	letter-spacing: .05em;
	color: var(--focus-color);
}

.focus-clock {
	margin: 0;
	font-size: 3rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	line-height: 1.1;
}

.focus-task {
	margin-block: .75rem 0;
	font-weight: 600;
	overflow-wrap: anywhere;
}

.focus-controls,
.focus-settings-actions {
	display: flex;
	flex-wrap: wrap;
	justify-content: center;
	gap: .5rem;
	margin-block-start: 1rem;
}

.focus-start {
	display: flex;
	flex-direction: column;
	gap: .75rem;
	margin-block-start: 1rem;
}

.focus-suggestion {
	margin: 0;
	font-weight: 600;
}

.focus-kind-picker {
	display: flex;
	flex-wrap: wrap;
	justify-content: center;
	gap: .5rem;
}

.focus-kind-option {
	display: flex;
	flex-direction: column;
	align-items: center;
	padding: .5rem .75rem;
	border: 1px solid var(--grey-300);
	border-radius: $radius;
	background: var(--white);
	color: var(--text);
	font: inherit;
	cursor: pointer;

	&.is-selected {
		border-color: var(--primary);
		box-shadow: 0 0 0 1px var(--primary);
	}

	&:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
}

.focus-kind-minutes {
	font-size: .75rem;
	color: var(--grey-600);
}

.focus-task-picker {
	display: flex;
	flex-direction: column;
	gap: .25rem;
	font-size: .9rem;
}

.focus-start-button {
	align-self: center;
	min-inline-size: 10rem;
}

.focus-settings {
	display: flex;
	flex-direction: column;
	gap: .5rem;
}

.focus-settings-actions {
	justify-content: flex-end;
}
</style>
