import {registerPlugin, type PluginListenerHandle} from '@capacitor/core'

export type FocusNotificationAction = 'pause' | 'resume' | 'stop'

export interface FocusNotificationOptions {
	title: string
	/** Epoch milliseconds when a running session ends; omitted while paused. */
	endsAt?: number
	paused: boolean
	/** Shown while paused, for example "Paused · 12:30 left". */
	pausedText: string
	pauseLabel: string
	resumeLabel: string
	stopLabel: string
}

// Implemented in android/native/app/src/main/java/com/hefzul/tasks/FocusPlugin.java:
// a foreground service keeps an ongoing notification with the time left and
// Pause / Stop actions while a focus session runs.
export interface FocusPlugin {
	show(options: FocusNotificationOptions): Promise<void>
	hide(): Promise<void>
	addListener(event: 'action', listener: (data: {action: FocusNotificationAction}) => void): Promise<PluginListenerHandle>
}

export const HefzulFocus = registerPlugin<FocusPlugin>('HefzulFocus')
