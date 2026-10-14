<template>
	<aside
		:class="{'is-active': baseStore.menuActive, 'is-resizing': isResizing}"
		class="menu-container"
		:style="{'--sidebar-width': sidebarWidthStyle}"
	>
		<nav
			class="menu top-menu"
			:aria-label="$t('navigation.main')"
		>
			<RouterLink
				:to="{name: 'home'}"
				class="logo"
				:aria-label="$t('navigation.home')"
			>
				<Logo
					width="164"
					height="48"
				/>
			</RouterLink>
			<menu class="menu-list other-menu-items">
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.overview"
						:to="{ name: 'home'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="calendar" />
						</span>
						{{ $t('navigation.overview') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink :to="{ name: 'tasks.today'}">
						<span class="menu-item-icon icon">
							<Icon icon="calendar-day" />
						</span>
						{{ $t('navigation.today') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink :to="{ name: 'tasks.next7Days'}">
						<span class="menu-item-icon icon">
							<Icon icon="calendar-week" />
						</span>
						{{ $t('navigation.next7Days') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.upcoming"
						:to="{ name: 'tasks.range'}"
					>
						<span class="menu-item-icon icon">
							<Icon :icon="['far', 'calendar-alt']" />
						</span>
						{{ $t('navigation.upcoming') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink :to="{ name: 'tasks.matrix'}">
						<span class="menu-item-icon icon">
							<Icon icon="th" />
						</span>
						{{ $t('navigation.matrix') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink :to="{ name: 'focus'}">
						<span class="menu-item-icon icon">
							<Icon icon="stopwatch" />
						</span>
						{{ $t('navigation.focus') }}
						<span
							v-if="focusTimer.current.value"
							class="focus-nav-clock"
							:class="{'is-paused': focusTimer.isPaused.value}"
						>{{ formatClock(focusTimer.remaining.value) }}</span>
					</RouterLink>
				</li>
				<li>
					<RouterLink :to="{ name: 'habits'}">
						<span class="menu-item-icon icon">
							<Icon icon="repeat" />
						</span>
						{{ $t('navigation.habits') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink :to="{ name: 'countdowns'}">
						<span class="menu-item-icon icon">
							<Icon icon="hourglass-half" />
						</span>
						{{ $t('navigation.countdowns') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.projects"
						:to="{ name: 'projects.index'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="layer-group" />
						</span>
						{{ $t('project.projects') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.labels"
						:to="{ name: 'labels.index'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="tags" />
						</span>
						{{ $t('label.title') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.teams"
						:to="{ name: 'teams.index'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="users" />
						</span>
						{{ $t('team.title') }}
					</RouterLink>
				</li>
				<li v-if="timeTrackingEnabled">
					<RouterLink :to="{ name: 'time-tracking'}">
						<span class="menu-item-icon icon">
							<Icon :icon="['far', 'clock']" />
						</span>
						{{ $t('timeTracking.title') }}
					</RouterLink>
				</li>
			</menu>
		</nav>

		<Loading
			v-if="projectList.isLoading"
			variant="small"
		/>
		<template v-else>
			<nav
				v-if="favoriteProjects.length"
				class="menu"
				:aria-label="$t('project.pseudo.favorites.title')"
			>
				<ProjectsNavigation
					:model-value="favoriteProjects"
					:can-edit-order="false"
					:can-collapse="false"
				/>
			</nav>
			
			<nav
				v-if="savedFilterProjects.length"
				class="menu"
				:aria-label="$t('navigation.savedFilters')"
			>
				<ProjectsNavigation
					:model-value="savedFilterProjects"
					:can-edit-order="false"
					:can-collapse="false"
				/>
			</nav>

			<nav
				class="menu"
				:aria-label="$t('project.projects')"
			>
				<ProjectsNavigation
					:model-value="projects"
					:can-edit-order="true"
					:can-collapse="true"
				/>
			</nav>
		</template>

		<PoweredByLink
			class="mbs-auto"
			utm-medium="navigation"
		/>

		<div
			v-if="!isMobile"
			class="resize-handle"
			@mousedown="startResize"
			@touchstart="startResize"
		/>
	</aside>
</template>

<script setup lang="ts">
import {computed} from 'vue'

import {SHORTCUTS} from '@/constants/shortcuts'
import PoweredByLink from '@/components/home/PoweredByLink.vue'
import Logo from '@/components/home/Logo.vue'
import Loading from '@/components/misc/Loading.vue'

import {useBaseStore} from '@/stores/base'
import {useProjects} from '@/composables/useProjects'
import {useConfigStore} from '@/stores/config'
import {PRO_FEATURE} from '@/constants/proFeatures'
import ProjectsNavigation from '@/components/home/ProjectsNavigation.vue'
import {useSidebarResize} from '@/composables/useSidebarResize'
import {useFocusTimer} from '@/composables/useFocusTimer'
import {formatClock} from '@/helpers/focusTimer'

const baseStore = useBaseStore()
const projectList = useProjects()
const configStore = useConfigStore()

const focusTimer = useFocusTimer()

const timeTrackingEnabled = computed(() => configStore.isProFeatureEnabled(PRO_FEATURE.TIME_TRACKING))

const {sidebarWidthStyle, isResizing, startResize, isMobile} = useSidebarResize()

const projects = computed(() => projectList.notArchivedRootProjects)
const favoriteProjects = computed(() => projectList.favoriteProjects)
const savedFilterProjects = computed(() => projectList.savedFilterProjects)
</script>

<style lang="scss" scoped>
.focus-nav-clock {
	margin-inline-start: auto;
	padding-inline: .4rem;
	border-radius: 1rem;
	background: var(--danger);
	color: #ffffff;
	font-size: .75rem;
	font-weight: 600;
	font-variant-numeric: tabular-nums;

	&.is-paused {
		background: var(--grey-500);
	}
}

.logo {
	display: block;

	padding-inline-start: 1rem;
	margin-inline-end: 1rem;
	margin-block-end: 1rem;

	@media screen and (min-width: $tablet) {
		display: none;
	}
}

.menu-container {
	--sidebar-width: #{$navbar-width};

	display: flex;
	flex-direction: column;
	background: var(--site-background);
	color: $vikunja-nav-color;
	padding: 1rem 0;
	transition: transform $transition-duration ease-in;
	position: fixed;
	inset-block-start: $navbar-height;
	inset-block-end: 0;
	inset-inline-start: 0;
	transform: translateX(-100%);
	inline-size: var(--sidebar-width);
	overflow-y: auto;

	[dir="rtl"] & {
		transform: translateX(100%);
	}

	@media screen and (max-width: $tablet) {
		inset-block-start: 0;
		inline-size: 70vw;
		z-index: 20;
	}

	&.is-active {
		transform: translateX(0);
		transition: transform $transition-duration ease-out;
	}

	&.is-resizing {
		transition: none;
	}
}

.resize-handle {
	position: absolute;
	inset-block-start: 0;
	inset-block-end: 0;
	inset-inline-end: 0;
	inline-size: 4px;
	cursor: ew-resize;
	background: transparent;
	transition: background-color $transition-duration ease;
	touch-action: none;

	&:hover,
	&:active {
		background-color: var(--primary);
	}
}

.top-menu .menu-list {
	li {
		font-weight: 600;
		font-family: $vikunja-font;
	}

	.list-menu-link,
	li > a {
		padding-inline-start: 2rem;
		display: inline-block;

		.icon {
			padding-block-end: .25rem;
		}
	}
}

.menu + .menu {
	padding-block-start: math.div($navbar-padding, 2);
}
</style>
