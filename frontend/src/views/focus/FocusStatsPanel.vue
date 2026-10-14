<template>
	<Card
		v-cy="'focusStats'"
		:title="$t('focus.stats.title')"
	>
		<div
			class="focus-range"
			role="tablist"
		>
			<button
				v-for="range in RANGES"
				:key="range"
				type="button"
				role="tab"
				class="focus-range-option"
				:class="{'is-selected': selectedRange === range}"
				:aria-selected="selectedRange === range"
				@click="selectedRange = range"
			>
				{{ $t(`focus.stats.range.${range}`) }}
			</button>
		</div>

		<dl class="focus-totals">
			<div>
				<dt>{{ $t('focus.stats.today') }}</dt>
				<dd>{{ formatDuration(todaySeconds) }}</dd>
			</div>
			<div>
				<dt>{{ $t(`focus.stats.total.${selectedRange}`) }}</dt>
				<dd>{{ formatDuration(stats?.total_focus_seconds ?? 0) }}</dd>
			</div>
			<div>
				<dt>{{ $t('focus.stats.sessions') }}</dt>
				<dd>{{ stats?.completed_sessions ?? 0 }}</dd>
			</div>
			<div>
				<dt>{{ $t('focus.stats.streak') }}</dt>
				<dd>{{ $t('focus.stats.days', {count: stats?.current_streak_days ?? 0}) }}</dd>
			</div>
		</dl>

		<figure class="focus-chart">
			<figcaption class="focus-chart-caption">
				{{ $t('focus.stats.perDay') }}
			</figcaption>
			<div
				class="focus-bars"
				:class="{'is-dense': bars.length > 14}"
			>
				<div
					v-for="bar in bars"
					:key="bar.date"
					class="focus-bar"
					:title="`${bar.label}: ${formatDuration(bar.seconds)}`"
				>
					<span
						class="focus-bar-fill"
						:style="{blockSize: `${bar.share * 100}%`}"
					/>
					<span class="focus-bar-label">{{ bar.short }}</span>
				</div>
			</div>
		</figure>

		<div class="focus-breakdowns">
			<section>
				<h3 class="focus-breakdown-title">
					{{ $t('focus.stats.byTask') }}
				</h3>
				<p
					v-if="tasks.length === 0"
					class="focus-empty"
				>
					{{ $t('focus.stats.none') }}
				</p>
				<table
					v-else
					class="table is-fullwidth is-narrow"
				>
					<tbody>
						<tr
							v-for="task in tasks"
							:key="task.task_id"
						>
							<td>
								<RouterLink
									v-if="task.title"
									:to="{name: 'task.detail', params: {id: task.task_id}}"
								>
									{{ task.title }}
								</RouterLink>
								<span
									v-else
									class="focus-empty"
								>{{ $t('focus.stats.unknownTask') }}</span>
							</td>
							<td class="has-text-end">
								{{ formatDuration(task.focus_seconds ?? 0) }}
							</td>
						</tr>
					</tbody>
				</table>
			</section>
			<section>
				<h3 class="focus-breakdown-title">
					{{ $t('focus.stats.byProject') }}
				</h3>
				<p
					v-if="projects.length === 0"
					class="focus-empty"
				>
					{{ $t('focus.stats.none') }}
				</p>
				<table
					v-else
					class="table is-fullwidth is-narrow"
				>
					<tbody>
						<tr
							v-for="project in projects"
							:key="project.project_id"
						>
							<td>
								<RouterLink :to="{name: 'project.index', params: {projectId: project.project_id}}">
									{{ project.title }}
								</RouterLink>
							</td>
							<td class="has-text-end">
								{{ formatDuration(project.focus_seconds ?? 0) }}
							</td>
						</tr>
					</tbody>
				</table>
			</section>
		</div>
	</Card>
</template>

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'

import Card from '@/components/misc/Card.vue'
import {focusStatsQuery} from '@/client/queries/focus'
import {formatDuration} from '@/helpers/focusTimer'
import {todayIn} from '@/composables/useFocusTimer'
import {useAuthStore} from '@/stores/auth'

const RANGES = ['week', 'month'] as const
type Range = typeof RANGES[number]
const RANGE_DAYS: Record<Range, number> = {week: 7, month: 30}

const {locale} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const selectedRange = ref<Range>('week')

function addDays(date: string, days: number): string {
	const d = new Date(`${date}T00:00:00Z`)
	d.setUTCDate(d.getUTCDate() + days)
	return d.toISOString().slice(0, 10)
}

const today = computed(() => todayIn(authStore.settings.timezone))
const from = computed(() => addDays(today.value, 1 - RANGE_DAYS[selectedRange.value]))

const statsQuery = useQuery(computed(() => ({
	...focusStatsQuery(from.value, today.value),
	enabled: authStore.authenticated && !authStore.isLinkShareAuth,
})))
const stats = computed(() => statsQuery.data.value)

const days = computed(() => stats.value?.days ?? [])
const todaySeconds = computed(() => days.value.find(day => day.date === today.value)?.focus_seconds ?? 0)

const bars = computed(() => {
	const max = Math.max(1, ...days.value.map(day => day.focus_seconds ?? 0))
	const weekday = new Intl.DateTimeFormat(locale.value, {weekday: 'narrow', timeZone: 'UTC'})
	const full = new Intl.DateTimeFormat(locale.value, {dateStyle: 'medium', timeZone: 'UTC'})
	return days.value.map(day => {
		const date = new Date(`${day.date}T00:00:00Z`)
		return {
			date: day.date ?? '',
			seconds: day.focus_seconds ?? 0,
			share: (day.focus_seconds ?? 0) / max,
			label: full.format(date),
			short: selectedRange.value === 'week' ? weekday.format(date) : String(date.getUTCDate()),
		}
	})
})

const tasks = computed(() => (stats.value?.tasks ?? []).slice(0, 8))
const projects = computed(() => (stats.value?.projects ?? []).slice(0, 8))
</script>

<style lang="scss" scoped>
.focus-range {
	display: inline-flex;
	gap: .25rem;
	margin-block-end: 1rem;
}

.focus-range-option {
	padding: .25rem .75rem;
	border: 1px solid var(--grey-300);
	border-radius: 1rem;
	background: var(--white);
	color: var(--text);
	font: inherit;
	font-size: .85rem;
	cursor: pointer;

	&.is-selected {
		border-color: var(--primary);
		background: var(--primary);
		color: #ffffff;
	}
}

.focus-totals {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
	gap: .75rem;
	margin: 0 0 1.25rem;

	dt {
		font-size: .8rem;
		color: var(--grey-600);
	}

	dd {
		margin: 0;
		font-size: 1.4rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}
}

.focus-chart {
	margin: 0 0 1.25rem;
}

.focus-chart-caption,
.focus-breakdown-title {
	margin-block-end: .5rem;
	font-style: normal;
	text-align: start;
	font-size: .9rem;
	font-weight: 600;
}

.focus-bars {
	display: flex;
	align-items: stretch;
	gap: .35rem;
	block-size: 8rem;

	&.is-dense {
		gap: 2px;
	}
}

.focus-bar {
	display: flex;
	flex: 1 1 0;
	flex-direction: column;
	justify-content: flex-end;
	align-items: center;
	min-inline-size: 0;
}

.focus-bar-fill {
	inline-size: 100%;
	min-block-size: 2px;
	border-radius: 3px 3px 0 0;
	background: var(--primary);
}

.focus-bar-label {
	margin-block-start: .25rem;
	font-size: .7rem;
	color: var(--grey-600);
}

.focus-breakdowns {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
	gap: 1rem;
}

.focus-empty {
	color: var(--grey-500);
	font-size: .9rem;
}
</style>
