<template>
	<div
		v-if="visible"
		v-cy="'offlineStatus'"
		class="offline-status"
		role="status"
		aria-live="polite"
	>
		<p
			v-if="!online"
			class="offline-line is-offline"
		>
			<Icon icon="cloud-upload-alt" />
			{{ $t('offlineSync.offline') }}
			<template v-if="pendingCount > 0">
				· {{ $t('offlineSync.pending', {count: pendingCount}, pendingCount) }}
			</template>
		</p>
		<p
			v-else-if="needsSignIn"
			class="offline-line is-warning"
		>
			{{ $t('offlineSync.signInAgain', {count: pendingCount}, pendingCount) }}
			<RouterLink :to="{name: 'user.login'}">
				{{ $t('offlineSync.signIn') }}
			</RouterLink>
		</p>
		<p
			v-else-if="pendingCount > 0"
			class="offline-line"
		>
			{{ syncing ? $t('offlineSync.syncing', {count: pendingCount}, pendingCount) : $t('offlineSync.pending', {count: pendingCount}, pendingCount) }}
			<BaseButton
				v-if="!syncing"
				class="offline-retry"
				@click="syncOutbox()"
			>
				{{ $t('offlineSync.syncNow') }}
			</BaseButton>
		</p>
		<p
			v-for="notice in notices"
			:key="notice.key"
			class="offline-line is-warning"
		>
			{{ notice.kind === 'delete-skipped'
				? $t('offlineSync.deleteSkipped', {title: notice.title})
				: $t('offlineSync.rejected', {title: notice.title || $t('offlineSync.aChange')}) }}
			<BaseButton
				class="offline-retry"
				@click="dismissNotice(notice.key)"
			>
				{{ $t('misc.close') }}
			</BaseButton>
		</p>

		<Modal
			v-if="conflict"
			@close="() => {}"
		>
			<Card
				class="has-no-shadow"
				:title="$t('offlineSync.conflictTitle', {title: conflict.title})"
			>
				<p>{{ $t('offlineSync.conflictIntro') }}</p>
				<table class="table is-fullwidth offline-conflict">
					<thead>
						<tr>
							<th>{{ $t('offlineSync.field') }}</th>
							<th>{{ $t('offlineSync.yours') }}</th>
							<th>{{ $t('offlineSync.theirs') }}</th>
						</tr>
					</thead>
					<tbody>
						<tr
							v-for="field in conflict.fields"
							:key="field.field"
						>
							<th scope="row">
								{{ fieldLabel(field.field) }}
							</th>
							<td>
								<label>
									<input
										v-model="choices[field.field]"
										type="radio"
										value="mine"
										:name="`conflict-${field.field}`"
									>
									{{ show(field.field, field.mine) }}
								</label>
							</td>
							<td>
								<label>
									<input
										v-model="choices[field.field]"
										type="radio"
										value="theirs"
										:name="`conflict-${field.field}`"
									>
									{{ show(field.field, field.theirs) }}
								</label>
							</td>
						</tr>
					</tbody>
				</table>
				<template #footer>
					<XButton
						v-cy="'conflictKeep'"
						:loading="resolving"
						@click="resolve"
					>
						{{ $t('offlineSync.keepSelected') }}
					</XButton>
				</template>
			</Card>
		</Modal>
	</div>
</template>

<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Icon from '@/components/misc/Icon'
import Modal from '@/components/misc/Modal.vue'
import {online} from '@/client/offline/network'
import {
	dismissNotice,
	outboxConflicts,
	outboxNeedsSignIn,
	outboxNotices,
	outboxSyncing,
	pendingCount,
	resolveConflict,
	syncOutbox,
} from '@/client/offline/outbox'
import {formatDateLong} from '@/helpers/time/formatDate'
import {error} from '@/message'

const {t, te} = useI18n({useScope: 'global'})

const needsSignIn = outboxNeedsSignIn
const syncing = outboxSyncing
const notices = outboxNotices
const conflict = computed(() => outboxConflicts.value[0])
const visible = computed(() => !online.value || pendingCount.value > 0 || needsSignIn.value
	|| notices.value.length > 0 || Boolean(conflict.value))

const choices = reactive<Record<string, 'mine' | 'theirs'>>({})
watch(conflict, value => {
	for (const key of Object.keys(choices)) delete choices[key]
	for (const field of value?.fields ?? []) choices[field.field] = 'mine'
}, {immediate: true})

const resolving = ref(false)
async function resolve() {
	if (!conflict.value) return
	resolving.value = true
	try {
		await resolveConflict(conflict.value.key, {...choices})
	} catch (e) {
		error(e)
	} finally {
		resolving.value = false
	}
}

const FIELD_LABELS: Record<string, string> = {
	title: 'offlineSync.fields.title',
	description: 'offlineSync.fields.description',
	done: 'offlineSync.fields.done',
	due_date: 'offlineSync.fields.dueDate',
	start_date: 'offlineSync.fields.startDate',
	end_date: 'offlineSync.fields.endDate',
	priority: 'offlineSync.fields.priority',
	percent_done: 'offlineSync.fields.percentDone',
}

function fieldLabel(field: string) {
	const key = FIELD_LABELS[field]
	return key && te(key) ? t(key) : field
}

function show(field: string, value: unknown) {
	if (value === null || value === undefined || value === '') return '—'
	if (field.endsWith('_date') && typeof value === 'string') {
		const date = new Date(value)
		return Number.isNaN(date.getTime()) || date.getFullYear() < 1970 ? '—' : formatDateLong(date)
	}
	if (typeof value === 'boolean') return value ? t('offlineSync.yes') : t('offlineSync.no')
	if (typeof value === 'object') return JSON.stringify(value)
	return String(value)
}
</script>

<style lang="scss" scoped>
.offline-status {
	display: flex;
	flex-direction: column;
	gap: .25rem;
	margin-block-end: 1rem;
}

.offline-line {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .5rem;
	margin: 0;
	padding: .4rem .75rem;
	border-radius: $radius;
	background: var(--grey-100);
	font-size: .9rem;

	&.is-offline {
		background: var(--grey-200);
	}

	&.is-warning {
		background: color-mix(in srgb, var(--warning) 20%, var(--white));
	}
}

.offline-retry {
	font-weight: 600;
	color: var(--primary);
}

.offline-conflict td,
.offline-conflict th {
	vertical-align: top;
	overflow-wrap: anywhere;
}
</style>
