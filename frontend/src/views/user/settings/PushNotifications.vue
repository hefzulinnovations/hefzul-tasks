<template>
	<Card :title="$t('user.settings.push.title')">
		<p>{{ $t('user.settings.push.intro') }}</p>

		<Message
			v-if="!supported"
			variant="warning"
			class="mbs-2"
		>
			{{ $t('user.settings.push.unsupported') }}
		</Message>

		<template v-else>
			<p class="mbs-2">
				<strong>{{ $t('user.settings.push.thisDevice') }}:</strong>
				{{ thisDeviceSubscribed ? $t('user.settings.push.on') : $t('user.settings.push.off') }}
			</p>
			<FormCheckbox
				v-model="includeContent"
				class="mbs-2"
				:label="$t('user.settings.push.includeContent')"
			/>
			<p class="help">
				{{ $t('user.settings.push.includeContentHint') }}
			</p>
			<div class="push-actions">
				<XButton
					v-cy="'pushEnable'"
					:loading="enable.isPending.value"
					@click="turnOn"
				>
					{{ thisDeviceSubscribed ? $t('user.settings.push.update') : $t('user.settings.push.enable') }}
				</XButton>
				<XButton
					v-if="thisDeviceSubscribed"
					variant="secondary"
					:loading="disable.isPending.value"
					@click="turnOff"
				>
					{{ $t('user.settings.push.disable') }}
				</XButton>
				<XButton
					v-if="devices.length > 0"
					variant="secondary"
					:loading="test.isPending.value"
					@click="sendTest"
				>
					{{ $t('user.settings.push.test') }}
				</XButton>
			</div>
			<p
				v-if="message"
				class="mbs-2"
			>
				{{ message }}
			</p>
		</template>

		<template v-if="devices.length > 0">
			<h3 class="mbs-4">
				{{ $t('user.settings.push.devices') }}
			</h3>
			<table class="table is-fullwidth">
				<tbody>
					<tr
						v-for="device in devices"
						:key="device.id"
					>
						<td>
							{{ device.device || $t('user.settings.push.unknownDevice') }}
							<span
								v-if="device.endpoint === browserEndpoint"
								class="tag"
							>{{ $t('user.settings.push.thisDevice') }}</span>
						</td>
						<td>{{ device.include_content ? $t('user.settings.push.withTitles') : $t('user.settings.push.private') }}</td>
						<td class="has-text-end">
							<BaseButton
								class="push-remove"
								@click="disable.mutate({endpoint: device.endpoint})"
							>
								{{ $t('misc.delete') }}
							</BaseButton>
						</td>
					</tr>
				</tbody>
			</table>
		</template>
	</Card>
</template>

<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Message from '@/components/misc/Message.vue'
import FormCheckbox from '@/components/input/FormCheckbox.vue'
import {useTitle} from '@/composables/useTitle'
import {
	currentBrowserSubscription,
	pushSubscriptionsQuery,
	pushSupported,
	useDisablePushMutation,
	useEnablePushMutation,
	useTestPushMutation,
} from '@/client/queries/push'

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('user.settings.push.title'))

const supported = pushSupported()
const list = useQuery(pushSubscriptionsQuery())
const devices = computed(() => list.data.value ?? [])

const browserEndpoint = ref('')
async function refreshBrowserEndpoint() {
	browserEndpoint.value = (await currentBrowserSubscription().catch(() => null))?.endpoint ?? ''
}
onMounted(refreshBrowserEndpoint)

const thisDevice = computed(() => devices.value.find(device => device.endpoint === browserEndpoint.value))
const thisDeviceSubscribed = computed(() => Boolean(thisDevice.value))
const includeContent = ref(false)
watch(thisDevice, device => {
	if (device) includeContent.value = device.include_content ?? false
}, {immediate: true})

const enable = useEnablePushMutation()
const disable = useDisablePushMutation()
const test = useTestPushMutation()
const message = ref('')

async function turnOn() {
	message.value = ''
	try {
		await enable.mutateAsync({includeContent: includeContent.value})
		message.value = t('user.settings.push.enabled')
	} catch {
		// The toast shows the error.
	}
	await refreshBrowserEndpoint()
}

async function turnOff() {
	await disable.mutateAsync({}).catch(() => {})
	await refreshBrowserEndpoint()
	message.value = ''
}

async function sendTest() {
	const delivered = await test.mutateAsync().catch(() => -1)
	if (delivered >= 0) message.value = t('user.settings.push.testSent', {count: delivered}, delivered)
}
</script>

<style lang="scss" scoped>
.push-actions {
	display: flex;
	flex-wrap: wrap;
	gap: .5rem;
	margin-block-start: 1rem;
}

.push-remove {
	color: var(--danger);
}
</style>
