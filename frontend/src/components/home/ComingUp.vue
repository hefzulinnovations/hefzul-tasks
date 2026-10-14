<template>
	<section
		v-if="countdowns.length > 0 || dueHabits.length > 0"
		v-cy="'comingUp'"
		class="coming-up has-text-start"
	>
		<div
			v-if="dueHabits.length > 0"
			class="coming-up-block"
		>
			<h2 class="coming-up-title">
				<RouterLink :to="{name: 'habits'}">
					{{ $t('home.habitsToday', {done: doneCount, total: dueHabits.length}) }}
				</RouterLink>
			</h2>
			<ul class="coming-up-habits">
				<li
					v-for="habit in dueHabits"
					:key="habit.id"
					class="coming-up-habit"
				>
					<HabitCheckButton
						:habit="habit"
						@checkIn="checkin.mutate({id: habit.id})"
					/>
					<span class="coming-up-habit-title">
						{{ habit.icon }} {{ habit.title }}
					</span>
				</li>
			</ul>
		</div>
		<div
			v-if="countdowns.length > 0"
			class="coming-up-block"
		>
			<h2 class="coming-up-title">
				<RouterLink :to="{name: 'countdowns'}">
					{{ $t('home.countdowns') }}
				</RouterLink>
			</h2>
			<ul class="coming-up-countdowns">
				<li
					v-for="countdown in countdowns"
					:key="countdown.id"
					class="coming-up-countdown"
				>
					<CountdownBadge :countdown="countdown" />
					<span class="coming-up-countdown-title">{{ countdown.title }}</span>
				</li>
			</ul>
		</div>
	</section>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useQuery} from '@tanstack/vue-query'

import HabitCheckButton from '@/components/habits/HabitCheckButton.vue'
import CountdownBadge from '@/components/countdowns/CountdownBadge.vue'
import {countdownsQuery} from '@/client/queries/countdowns'
import {habitsQuery, useCheckinHabitMutation} from '@/client/queries/habits'
import {useAuthStore} from '@/stores/auth'

const props = withDefaults(defineProps<{showHabits?: boolean}>(), {showHabits: true})

const authStore = useAuthStore()
const enabled = computed(() => authStore.authenticated && !authStore.isLinkShareAuth)

// Pinned countdowns and those in the next 7 days (FR-25).
const countdownList = useQuery(computed(() => ({...countdownsQuery(7), enabled: enabled.value})))
const countdowns = computed(() => countdownList.data.value ?? [])

const habitList = useQuery(computed(() => ({...habitsQuery(false), enabled: enabled.value && props.showHabits})))
const dueHabits = computed(() => props.showHabits
	? (habitList.data.value ?? []).filter(habit => habit.due_today || habit.done_today)
	: [])
const doneCount = computed(() => dueHabits.value.filter(habit => habit.done_today).length)

const checkin = useCheckinHabitMutation()
</script>

<style lang="scss" scoped>
.coming-up {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
	gap: 1rem;
	margin-block: 1rem;
}

.coming-up-block {
	padding: .75rem 1rem;
	border-radius: $radius;
	background: var(--white);
	box-shadow: var(--shadow-sm);
}

.coming-up-title {
	margin: 0 0 .5rem;
	font-size: 1rem;
}

.coming-up-habits,
.coming-up-countdowns {
	display: flex;
	flex-wrap: wrap;
	gap: .5rem .75rem;
	margin: 0;
	padding: 0;
	list-style: none;
}

.coming-up-habit,
.coming-up-countdown {
	display: flex;
	align-items: center;
	gap: .5rem;
	min-inline-size: 0;
}

.coming-up-habit-title,
.coming-up-countdown-title {
	font-size: .9rem;
	overflow-wrap: anywhere;
}
</style>
