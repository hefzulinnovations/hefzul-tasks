<template>
	<div class="habit-heatmap">
		<div class="habit-heatmap-nav">
			<BaseButton
				class="habit-heatmap-step"
				:aria-label="$t('habits.previousMonth')"
				@click="shiftMonth(-1)"
			>
				<Icon icon="angle-left" />
			</BaseButton>
			<span class="habit-heatmap-month">{{ monthLabel }}</span>
			<BaseButton
				class="habit-heatmap-step"
				:aria-label="$t('habits.nextMonth')"
				:disabled="isCurrentMonth"
				@click="shiftMonth(1)"
			>
				<Icon icon="angle-right" />
			</BaseButton>
		</div>
		<div
			class="habit-heatmap-grid"
			role="grid"
			:aria-label="monthLabel"
		>
			<span
				v-for="day in weekdayLabels"
				:key="day"
				class="habit-heatmap-weekday"
				aria-hidden="true"
			>{{ day }}</span>
			<span
				v-for="n in leadingBlanks"
				:key="`blank-${n}`"
			/>
			<span
				v-for="cell in cells"
				:key="cell.date"
				role="gridcell"
				class="habit-heatmap-cell"
				:class="{'is-done': cell.done, 'is-partial': !cell.done && cell.count > 0, 'is-today': cell.date === today}"
				:style="{'--habit-color': color}"
				:title="$t('habits.dayCount', {date: cell.label, count: cell.count})"
			>{{ cell.day }}</span>
		</div>
		<dl class="habit-heatmap-stats">
			<div>
				<dt>{{ $t('habits.currentStreak') }}</dt>
				<dd>{{ streakText(stats?.current_streak ?? 0) }}</dd>
			</div>
			<div>
				<dt>{{ $t('habits.bestStreak') }}</dt>
				<dd>{{ streakText(stats?.best_streak ?? 0) }}</dd>
			</div>
			<div>
				<dt>{{ $t('habits.monthRate') }}</dt>
				<dd>{{ Math.round((monthStats?.rate ?? 0) * 100) }}%</dd>
			</div>
		</dl>
	</div>
</template>

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import Icon from '@/components/misc/Icon'
import {habitStatsQuery, type HabitResponse} from '@/client/queries/habits'
import {todayIn} from '@/composables/useFocusTimer'
import {useAuthStore} from '@/stores/auth'

const props = defineProps<{habit: HabitResponse}>()

const {t, locale} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const today = computed(() => todayIn(authStore.settings.timezone))
const weekStart = computed(() => authStore.settings.week_start ?? 0)

const month = ref(currentMonth())
function currentMonth() {
	return today.value.slice(0, 7)
}
const isCurrentMonth = computed(() => month.value >= today.value.slice(0, 7))

function shiftMonth(delta: number) {
	const [y, m] = month.value.split('-').map(Number)
	const d = new Date(Date.UTC(y, m - 1 + delta, 1))
	month.value = d.toISOString().slice(0, 7)
}

const range = computed(() => {
	const [y, m] = month.value.split('-').map(Number)
	const last = new Date(Date.UTC(y, m, 0)).getUTCDate()
	return {from: `${month.value}-01`, to: `${month.value}-${String(last).padStart(2, '0')}`, days: last, firstWeekday: new Date(Date.UTC(y, m - 1, 1)).getUTCDay()}
})

const statsQuery = useQuery(computed(() => habitStatsQuery(props.habit.id, range.value.from, range.value.to)))
const stats = computed(() => statsQuery.data.value)
const monthStats = computed(() => stats.value?.months?.find(m => m.month === month.value))

const monthLabel = computed(() => new Intl.DateTimeFormat(locale.value, {month: 'long', year: 'numeric', timeZone: 'UTC'})
	.format(new Date(`${month.value}-01T00:00:00Z`)))

const weekdayLabels = computed(() => {
	const fmt = new Intl.DateTimeFormat(locale.value, {weekday: 'narrow', timeZone: 'UTC'})
	// 2023-01-01 was a Sunday.
	return Array.from({length: 7}, (_, i) => fmt.format(new Date(Date.UTC(2023, 0, 1 + (i + weekStart.value) % 7))))
})

const leadingBlanks = computed(() => (range.value.firstWeekday - weekStart.value + 7) % 7)

const cells = computed(() => {
	const byDate = new Map((stats.value?.days ?? []).map(day => [day.date, day]))
	const full = new Intl.DateTimeFormat(locale.value, {dateStyle: 'medium', timeZone: 'UTC'})
	return Array.from({length: range.value.days}, (_, i) => {
		const date = `${month.value}-${String(i + 1).padStart(2, '0')}`
		const entry = byDate.get(date)
		return {
			date,
			day: i + 1,
			count: entry?.count ?? 0,
			done: entry?.done ?? false,
			label: full.format(new Date(`${date}T00:00:00Z`)),
		}
	})
})

const color = computed(() => props.habit.hex_color ? `#${props.habit.hex_color}` : 'var(--primary)')

function streakText(n: number) {
	return stats.value?.streak_unit === 'weeks' ? t('habits.weeks', {count: n}, n) : t('habits.days', {count: n}, n)
}
</script>

<style lang="scss" scoped>
.habit-heatmap {
	max-inline-size: 22rem;
}

.habit-heatmap-nav {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-block-end: .5rem;
}

.habit-heatmap-month {
	font-weight: 600;
}

.habit-heatmap-step {
	padding: .25rem .5rem;
}

.habit-heatmap-grid {
	display: grid;
	grid-template-columns: repeat(7, minmax(0, 1fr));
	gap: 4px;
}

.habit-heatmap-weekday {
	font-size: .7rem;
	text-align: center;
	color: var(--grey-500);
}

.habit-heatmap-cell {
	display: flex;
	align-items: center;
	justify-content: center;
	aspect-ratio: 1;
	border-radius: 4px;
	background: var(--grey-100);
	font-size: .75rem;
	color: var(--grey-600);

	&.is-partial {
		background: color-mix(in srgb, var(--habit-color) 30%, transparent);
		color: var(--text);
	}

	&.is-done {
		background: var(--habit-color);
		color: #ffffff;
	}

	&.is-today {
		box-shadow: inset 0 0 0 2px var(--grey-700);
	}
}

.habit-heatmap-stats {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: .5rem;
	margin: .75rem 0 0;

	dt {
		font-size: .75rem;
		color: var(--grey-600);
	}

	dd {
		margin: 0;
		font-weight: 700;
	}
}
</style>
