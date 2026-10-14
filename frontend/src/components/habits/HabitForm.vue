<template>
	<form
		class="habit-form"
		@submit.prevent="submit"
	>
		<div class="habit-form-row">
			<FormField
				:label="$t('habits.form.icon')"
				class="habit-form-icon"
			>
				<FormInput
					v-model="draft.icon"
					maxlength="8"
					:placeholder="'📖'"
				/>
			</FormField>
			<FormField
				:label="$t('habits.form.title')"
				class="habit-form-title"
			>
				<FormInput
					v-model="draft.title"
					v-focus
					required
					maxlength="250"
				/>
			</FormField>
		</div>

		<FormField :label="$t('habits.form.schedule')">
			<FormSelect
				v-model="draft.schedule_kind"
				:options="scheduleOptions"
			/>
		</FormField>

		<fieldset
			v-if="draft.schedule_kind === 'weekdays'"
			class="habit-form-days"
		>
			<legend>{{ $t('habits.form.days') }}</legend>
			<label
				v-for="day in weekdayOptions"
				:key="day.bit"
				class="habit-form-day"
				:class="{'is-selected': (draft.weekdays & day.bit) !== 0}"
			>
				<input
					type="checkbox"
					:checked="(draft.weekdays & day.bit) !== 0"
					@change="toggleDay(day.bit)"
				>
				{{ day.label }}
			</label>
		</fieldset>

		<FormField
			v-if="draft.schedule_kind === 'times_per_week'"
			:label="$t('habits.form.timesPerWeek')"
		>
			<FormInput
				v-model.number="draft.times_per_week"
				type="number"
				min="1"
				max="7"
			/>
		</FormField>

		<div class="habit-form-row">
			<FormField :label="$t('habits.form.dailyTarget')">
				<FormInput
					v-model.number="draft.daily_target"
					type="number"
					min="1"
					max="100"
				/>
			</FormField>
			<FormField :label="$t('habits.form.reminder')">
				<FormInput
					v-model="draft.reminder_time"
					type="time"
				/>
			</FormField>
		</div>

		<FormField :label="$t('habits.form.color')">
			<ColorPicker v-model="color" />
		</FormField>

		<FormCheckbox
			v-if="isEdit"
			v-model="draft.archived"
			:label="$t('habits.form.archived')"
		/>

		<p
			v-if="error"
			class="help is-danger"
		>
			{{ error }}
		</p>

		<div class="habit-form-actions">
			<XButton
				v-if="isEdit"
				variant="tertiary"
				class="is-danger"
				type="button"
				@click="$emit('delete')"
			>
				{{ $t('misc.delete') }}
			</XButton>
			<XButton
				variant="secondary"
				type="button"
				@click="$emit('cancel')"
			>
				{{ $t('misc.cancel') }}
			</XButton>
			<XButton
				type="submit"
				:loading="loading"
			>
				{{ $t('misc.save') }}
			</XButton>
		</div>
	</form>
</template>

<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import {useI18n} from 'vue-i18n'

import FormField from '@/components/input/FormField.vue'
import FormInput from '@/components/input/FormInput.vue'
import FormSelect from '@/components/input/FormSelect.vue'
import FormCheckbox from '@/components/input/FormCheckbox.vue'
import ColorPicker from '@/components/input/ColorPicker.vue'
import {emptyHabitInput, type HabitInput, type HabitResponse} from '@/client/queries/habits'
import {useAuthStore} from '@/stores/auth'

const props = defineProps<{
	habit?: HabitResponse | null
	loading?: boolean
}>()
const emit = defineEmits<{
	submit: [input: HabitInput]
	cancel: []
	delete: []
}>()

const {t, locale} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const isEdit = computed(() => Boolean(props.habit))

// The form edits a copy; the parent remounts it for another habit.
function initialDraft(): HabitInput {
	return props.habit
		? {
			title: props.habit.title,
			icon: props.habit.icon,
			hex_color: props.habit.hex_color,
			schedule_kind: props.habit.schedule_kind,
			weekdays: props.habit.weekdays,
			times_per_week: props.habit.times_per_week || 3,
			daily_target: props.habit.daily_target,
			reminder_time: props.habit.reminder_time,
			archived: props.habit.archived,
			position: props.habit.position,
		}
		: emptyHabitInput()
}

const draft = reactive<HabitInput>(initialDraft())

const color = computed({
	get: () => draft.hex_color,
	set: value => { draft.hex_color = (value ?? '').replace('#', '') },
})

const scheduleOptions = computed(() => [
	{value: 'daily', label: t('habits.schedule.daily')},
	{value: 'weekdays', label: t('habits.schedule.weekdays')},
	{value: 'times_per_week', label: t('habits.schedule.timesPerWeek')},
])

const weekdayOptions = computed(() => {
	const fmt = new Intl.DateTimeFormat(locale.value, {weekday: 'short', timeZone: 'UTC'})
	const start = authStore.settings.week_start ?? 0
	return Array.from({length: 7}, (_, i) => {
		const day = (i + start) % 7
		// 2023-01-01 was a Sunday.
		return {bit: 1 << day, label: fmt.format(new Date(Date.UTC(2023, 0, 1 + day)))}
	})
})

function toggleDay(bit: number) {
	draft.weekdays ^= bit
}

const error = ref('')

function submit() {
	error.value = ''
	if (draft.schedule_kind === 'weekdays' && draft.weekdays === 0) {
		error.value = t('habits.form.pickDays')
		return
	}
	emit('submit', {
		...draft,
		weekdays: draft.schedule_kind === 'weekdays' ? draft.weekdays : 0,
		times_per_week: draft.schedule_kind === 'times_per_week' ? draft.times_per_week : 0,
	})
}
</script>

<style lang="scss" scoped>
.habit-form {
	display: flex;
	flex-direction: column;
	gap: .5rem;
}

.habit-form-row {
	display: flex;
	flex-wrap: wrap;
	gap: .75rem;

	> * {
		flex: 1 1 8rem;
	}
}

.habit-form-icon {
	flex: 0 0 5rem;
}

.habit-form-days {
	display: flex;
	flex-wrap: wrap;
	gap: .35rem;
	border: 0;
	margin: 0;
	padding: 0;

	legend {
		margin-block-end: .35rem;
		font-weight: 600;
	}
}

.habit-form-day {
	padding: .25rem .6rem;
	border: 1px solid var(--grey-300);
	border-radius: 1rem;
	font-size: .85rem;
	cursor: pointer;

	input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	&.is-selected {
		border-color: var(--primary);
		background: var(--primary);
		color: #ffffff;
	}

	&:focus-within {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
}

.habit-form-actions {
	display: flex;
	flex-wrap: wrap;
	justify-content: flex-end;
	gap: .5rem;
	margin-block-start: .5rem;
}
</style>
