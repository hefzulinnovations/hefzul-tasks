<template>
	<BaseButton
		class="menu-bottom-link"
		:href="computedUrl"
		target="_blank"
	>
		{{ $t('misc.poweredBy') }}
	</BaseButton>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import {SOURCE_CODE} from '@/urls'

defineProps<{
	utmMedium: string;
}>()

const computedUrl = computed(() => SOURCE_CODE)
</script>

<style lang="scss">
.menu-bottom-link {
	color: var(--grey-300);
	text-align: center;
	display: block;
	padding-block-start: 1rem;
	padding-block-end: 1rem;
	font-size: .8rem;
}
</style>
