<template>
	<div
		class="countdown-badge"
		:style="{'--countdown-color': color}"
	>
		<span class="countdown-days">{{ number }}</span>
		<span class="countdown-unit">{{ unit }}</span>
	</div>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import type {CountdownResponse} from '@/client/queries/countdowns'

const props = defineProps<{countdown: CountdownResponse}>()
const {t} = useI18n({useScope: 'global'})

const color = computed(() => props.countdown.hex_color ? `#${props.countdown.hex_color}` : 'var(--primary)')
const number = computed(() => props.countdown.days_remaining === 0 ? '🎉' : String(Math.abs(props.countdown.days_remaining)))
const unit = computed(() => {
	const days = props.countdown.days_remaining
	if (days === 0) return t('countdowns.today')
	if (days < 0) return t('countdowns.daysAgo', {count: -days}, -days)
	return t('countdowns.daysLeft', {count: days}, days)
})
</script>

<style lang="scss" scoped>
.countdown-badge {
	display: flex;
	flex: none;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	min-inline-size: 4rem;
	padding: .35rem .5rem;
	border-radius: $radius;
	background: var(--countdown-color);
	color: #ffffff;
	line-height: 1.1;
}

.countdown-days {
	font-size: 1.4rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}

.countdown-unit {
	font-size: .65rem;
	text-transform: uppercase;
	letter-spacing: .04em;
}
</style>
