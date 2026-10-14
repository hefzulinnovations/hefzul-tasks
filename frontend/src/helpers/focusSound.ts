// End-of-session sounds, synthesised with the Web Audio API so nothing is
// loaded from another host.

type AudioContextConstructor = typeof AudioContext

function audioContext(): AudioContext | null {
	const Ctor: AudioContextConstructor | undefined = window.AudioContext
		?? (window as unknown as {webkitAudioContext?: AudioContextConstructor}).webkitAudioContext
	return Ctor ? new Ctor() : null
}

function tone(ctx: AudioContext, frequency: number, startAt: number, duration: number, volume: number) {
	const osc = ctx.createOscillator()
	const gain = ctx.createGain()
	osc.type = 'sine'
	osc.frequency.value = frequency
	gain.gain.setValueAtTime(0.0001, startAt)
	gain.gain.exponentialRampToValueAtTime(volume, startAt + 0.02)
	gain.gain.exponentialRampToValueAtTime(0.0001, startAt + duration)
	osc.connect(gain).connect(ctx.destination)
	osc.start(startAt)
	osc.stop(startAt + duration + 0.05)
}

export const FOCUS_SOUNDS = ['bell', 'chime', 'none'] as const

export function playFocusSound(sound: string) {
	if (sound === 'none') return
	try {
		const ctx = audioContext()
		if (!ctx) return
		const t0 = ctx.currentTime
		if (sound === 'chime') {
			tone(ctx, 660, t0, 0.5, 0.25)
			tone(ctx, 880, t0 + 0.25, 0.6, 0.25)
			tone(ctx, 1320, t0 + 0.5, 0.9, 0.2)
		} else {
			tone(ctx, 880, t0, 1.6, 0.3)
			tone(ctx, 1760, t0, 0.8, 0.08)
			tone(ctx, 880, t0 + 1.0, 1.6, 0.25)
		}
		setTimeout(() => ctx.close().catch(() => {}), 3000)
	} catch {
		// Audio is best-effort.
	}
}
