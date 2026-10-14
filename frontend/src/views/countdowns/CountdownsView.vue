<template>
	<div
		v-cy="'countdownsView'"
		class="countdowns-page has-text-start"
	>
		<header class="countdowns-header">
			<h2 class="title">
				{{ $t('countdowns.title') }}
			</h2>
			<XButton
				v-cy="'countdownNew'"
				icon="plus"
				@click="creating = true; editingId = null"
			>
				{{ $t('countdowns.new') }}
			</XButton>
		</header>

		<Card
			v-if="creating"
			:title="$t('countdowns.new')"
			class="mbe-4"
		>
			<CountdownForm
				:loading="createCountdown.isPending.value"
				@submit="create"
				@cancel="creating = false"
			/>
		</Card>

		<ul
			v-if="countdowns.length > 0"
			class="countdowns-list"
		>
			<li
				v-for="countdown in countdowns"
				:key="countdown.id"
				class="countdown-card"
				:class="{'is-past': countdown.days_remaining < 0}"
			>
				<div class="countdown-row">
					<CountdownBadge :countdown="countdown" />
					<div class="countdown-main">
						<span class="countdown-title">
							<Icon
								v-if="countdown.pinned"
								icon="thumbtack"
								class="countdown-pin"
							/>
							{{ countdown.title }}
						</span>
						<span class="countdown-meta">
							{{ formatDay(countdown.next_date) }}
							<template v-if="countdown.repeat_yearly && countdown.years > 0">
								· {{ $t('countdowns.years', {count: countdown.years}, countdown.years) }}
							</template>
							<template v-else-if="countdown.repeat_yearly">
								· {{ $t('countdowns.everyYear') }}
							</template>
						</span>
					</div>
					<BaseButton
						class="countdown-edit"
						:aria-label="$t('countdowns.edit', {title: countdown.title})"
						:title="$t('countdowns.edit', {title: countdown.title})"
						@click="editingId = editingId === countdown.id ? null : countdown.id"
					>
						<Icon icon="pen" />
					</BaseButton>
				</div>
				<CountdownForm
					v-if="editingId === countdown.id"
					:countdown="countdown"
					:loading="updateCountdown.isPending.value"
					class="countdown-edit-form"
					@submit="input => save(countdown.id, input)"
					@cancel="editingId = null"
					@delete="remove(countdown)"
				/>
			</li>
		</ul>
		<div
			v-else-if="!list.isPending.value && !creating"
			class="countdowns-empty"
		>
			<p>{{ $t('countdowns.empty') }}</p>
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
import CountdownForm from '@/components/countdowns/CountdownForm.vue'
import CountdownBadge from '@/components/countdowns/CountdownBadge.vue'
import {setTitle} from '@/helpers/setTitle'
import {useAuthStore} from '@/stores/auth'
import {
	countdownsQuery,
	useCreateCountdownMutation,
	useDeleteCountdownMutation,
	useUpdateCountdownMutation,
	type CountdownInput,
	type CountdownResponse,
} from '@/client/queries/countdowns'

const {t, locale} = useI18n({useScope: 'global'})
const authStore = useAuthStore()

const list = useQuery(computed(() => ({...countdownsQuery(0), enabled: authStore.authenticated})))
const countdowns = computed(() => list.data.value ?? [])

const creating = ref(false)
const editingId = ref<number | null>(null)
const createCountdown = useCreateCountdownMutation()
const updateCountdown = useUpdateCountdownMutation()
const deleteCountdown = useDeleteCountdownMutation()

async function create(input: CountdownInput) {
	await createCountdown.mutateAsync(input).then(() => { creating.value = false }).catch(() => {})
}

async function save(id: number, input: CountdownInput) {
	await updateCountdown.mutateAsync({...input, id}).then(() => { editingId.value = null }).catch(() => {})
}

async function remove(countdown: CountdownResponse) {
	if (!window.confirm(t('countdowns.confirmDelete', {title: countdown.title}))) return
	await deleteCountdown.mutateAsync({id: countdown.id}).catch(() => {})
	editingId.value = null
}

function formatDay(date: string) {
	if (!date) return ''
	return new Intl.DateTimeFormat(locale.value, {dateStyle: 'full', timeZone: 'UTC'}).format(new Date(`${date}T00:00:00Z`))
}

watchEffect(() => setTitle(t('countdowns.title')))
</script>

<style lang="scss" scoped>
.countdowns-page {
	max-inline-size: 48rem;
}

.countdowns-header {
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

.countdowns-list {
	display: flex;
	flex-direction: column;
	gap: .5rem;
	margin: 0;
	padding: 0;
	list-style: none;
}

.countdown-card {
	padding: .75rem;
	border: 1px solid var(--grey-200);
	border-radius: $radius;
	background: var(--white);

	&.is-past {
		opacity: .6;
	}
}

.countdown-row {
	display: flex;
	align-items: center;
	gap: .75rem;
}

.countdown-main {
	display: flex;
	flex: 1;
	flex-direction: column;
	min-inline-size: 0;
}

.countdown-title {
	font-weight: 600;
	overflow-wrap: anywhere;
}

.countdown-pin {
	margin-inline-end: .25rem;
	color: var(--grey-500);
	font-size: .8rem;
}

.countdown-meta {
	font-size: .8rem;
	color: var(--grey-600);
}

.countdown-edit {
	padding: .25rem;
	color: var(--grey-500);
}

.countdown-edit-form {
	margin-block-start: .75rem;
	padding-block-start: .75rem;
	border-block-start: 1px solid var(--grey-200);
}

.countdowns-empty {
	padding: 2rem 0;
	text-align: center;
	color: var(--grey-600);
}
</style>
