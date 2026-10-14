/** Today's date (YYYY-MM-DD) in the given IANA time zone, or the browser's. */
export function todayIn(timezone: string | undefined): string {
	try {
		return new Intl.DateTimeFormat('en-CA', {timeZone: timezone || undefined}).format(new Date())
	} catch {
		return new Intl.DateTimeFormat('en-CA').format(new Date())
	}
}
