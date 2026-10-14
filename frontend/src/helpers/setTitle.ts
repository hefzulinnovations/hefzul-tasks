export function setTitle(title : undefined | string) {
	document.title = (typeof title === 'undefined' || title === '')
		? 'Hefzul Tasks'
		: `${title} | Hefzul Tasks`
}
