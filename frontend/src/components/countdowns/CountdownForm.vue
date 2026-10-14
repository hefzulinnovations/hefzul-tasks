<template>
	<form
		class="countdown-form"
		@submit.prevent="$emit('submit', {...draft})"
	>
		<div class="countdown-form-row">
			<FormField :label="$t('countdowns.form.title')">
				<FormInput
					v-model="draft.title"
					v-focus
					required
					maxlength="250"
				/>
			</FormField>
			<FormField :label="$t('countdowns.form.date')">
				<FormInput
					v-model="draft.target_date"
					type="date"
					required
				/>
			</FormField>
		</div>
		<FormCheckbox
			v-model="draft.repeat_yearly"
			:label="$t('countdowns.form.repeatYearly')"
		/>
		<FormCheckbox
			v-model="draft.pinned"
			:label="$t('countdowns.form.pinned')"
		/>
		<FormField :label="$t('countdowns.form.color')">
			<ColorPicker v-model="color" />
		</FormField>
		<div class="countdown-form-actions">
			<XButton
				v-if="countdown"
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
import {computed, reactive} from 'vue'

import FormField from '@/components/input/FormField.vue'
import FormInput from '@/components/input/FormInput.vue'
import FormCheckbox from '@/components/input/FormCheckbox.vue'
import ColorPicker from '@/components/input/ColorPicker.vue'
import type {CountdownInput, CountdownResponse} from '@/client/queries/countdowns'

const props = defineProps<{
	countdown?: CountdownResponse | null
	loading?: boolean
}>()
defineEmits<{
	submit: [input: CountdownInput]
	cancel: []
	delete: []
}>()

// The form edits a copy; the parent remounts it for another countdown.
function initialDraft(): CountdownInput {
	return {
		title: props.countdown?.title ?? '',
		target_date: props.countdown?.target_date ?? '',
		repeat_yearly: props.countdown?.repeat_yearly ?? false,
		hex_color: props.countdown?.hex_color ?? '',
		pinned: props.countdown?.pinned ?? false,
	}
}

const draft = reactive<CountdownInput>(initialDraft())

const color = computed({
	get: () => draft.hex_color,
	set: value => { draft.hex_color = (value ?? '').replace('#', '') },
})
</script>

<style lang="scss" scoped>
.countdown-form {
	display: flex;
	flex-direction: column;
	gap: .5rem;
}

.countdown-form-row {
	display: flex;
	flex-wrap: wrap;
	gap: .75rem;

	> * {
		flex: 1 1 12rem;
	}
}

.countdown-form-actions {
	display: flex;
	flex-wrap: wrap;
	justify-content: flex-end;
	gap: .5rem;
}
</style>
