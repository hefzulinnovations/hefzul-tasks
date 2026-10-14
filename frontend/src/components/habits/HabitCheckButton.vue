<template>
	<button
		type="button"
		class="habit-check"
		:class="{'is-done': habit.done_today}"
		:style="{'--habit-color': color}"
		:aria-label="label"
		:title="label"
		@click="$emit('checkIn')"
	>
		<svg
			viewBox="0 0 44 44"
			aria-hidden="true"
		>
			<circle
				class="habit-check-track"
				cx="22"
				cy="22"
				r="19"
			/>
			<circle
				class="habit-check-progress"
				cx="22"
				cy="22"
				r="19"
				:stroke-dasharray="RING"
				:stroke-dashoffset="RING * (1 - share)"
			/>
		</svg>
		<span class="habit-check-inner">
			<Icon
				v-if="habit.done_today && habit.daily_target === 1"
				icon="check"
			/>
			<span
				v-else-if="habit.daily_target > 1"
				class="habit-check-count"
			>{{ habit.today_count }}/{{ habit.daily_target }}</span>
			<span
				v-else
				class="habit-check-icon"
			>{{ habit.icon || '' }}</span>
		</span>
	</button>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import Icon from '@/components/misc/Icon'
import type {HabitResponse} from '@/client/queries/habits'

const props = defineProps<{habit: HabitResponse}>()
defineEmits<{checkIn: []}>()

const RING = 2 * Math.PI * 19
const {t} = useI18n({useScope: 'global'})

const share = computed(() => Math.min(1, props.habit.today_count / Math.max(1, props.habit.daily_target)))
const color = computed(() => props.habit.hex_color ? `#${props.habit.hex_color}` : 'var(--primary)')
const label = computed(() => t('habits.checkIn', {title: props.habit.title, count: props.habit.today_count, target: props.habit.daily_target}))
</script>

<style lang="scss" scoped>
.habit-check {
	position: relative;
	flex: none;
	inline-size: 2.75rem;
	block-size: 2.75rem;
	padding: 0;
	border: 0;
	border-radius: 50%;
	background: transparent;
	color: var(--habit-color);
	cursor: pointer;

	> svg {
		position: absolute;
		inset: 0;
		transform: rotate(-90deg);
	}

	&:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}

	&.is-done .habit-check-inner {
		background: var(--habit-color);
		color: #ffffff;
	}
}

.habit-check-track,
.habit-check-progress {
	fill: none;
	stroke-width: 4;
}

.habit-check-track {
	stroke: var(--grey-200);
}

.habit-check-progress {
	stroke: var(--habit-color);
	stroke-linecap: round;
	transition: stroke-dashoffset .3s ease;
}

.habit-check-inner {
	position: absolute;
	inset: 6px;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: 50%;
	font-size: .75rem;
	font-weight: 700;
}

.habit-check-icon {
	font-size: 1rem;
}
</style>
