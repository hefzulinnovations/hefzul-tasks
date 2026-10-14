import {describe, expect, it} from 'vitest'

import {deleteStillApplies, resolveTaskUpdate, sameValue} from './conflicts'

const BASE_UPDATED = '2026-10-14T09:00:00Z'

describe('offline conflict resolution', () => {
	it('applies every offline change when the server copy is unchanged', () => {
		const result = resolveTaskUpdate(
			{title: 'New', done: true},
			{title: 'Old', done: false},
			BASE_UPDATED,
			{title: 'Old', done: false, updated: '2026-10-14T09:00:00+00:00'},
		)
		expect(result).toEqual({apply: {title: 'New', done: true}, conflicts: []})
	})

	it('keeps offline values for fields the server did not touch', () => {
		const result = resolveTaskUpdate(
			{title: 'Mine'},
			{title: 'Old', priority: 1},
			BASE_UPDATED,
			{title: 'Old', priority: 4, updated: '2026-10-14T10:00:00Z'},
		)
		expect(result.apply).toEqual({title: 'Mine'})
		expect(result.conflicts).toEqual([])
	})

	it('reports a field both sides changed differently', () => {
		const result = resolveTaskUpdate(
			{title: 'Mine', done: true},
			{title: 'Old', done: false},
			BASE_UPDATED,
			{title: 'Theirs', done: false, updated: '2026-10-14T10:00:00Z'},
		)
		expect(result.apply).toEqual({done: true})
		expect(result.conflicts).toEqual([{field: 'title', mine: 'Mine', theirs: 'Theirs'}])
	})

	it('treats the same change on both sides as no conflict', () => {
		const result = resolveTaskUpdate(
			{due_date: '2026-10-20T08:00:00Z'},
			{due_date: '0001-01-01T00:00:00Z'},
			BASE_UPDATED,
			{due_date: '2026-10-20T14:00:00+06:00', updated: '2026-10-14T10:00:00Z'},
		)
		expect(result).toEqual({apply: {}, conflicts: []})
	})

	it('compares dates by instant and the zero date as empty', () => {
		expect(sameValue('due_date', '2026-10-20T08:00:00Z', '2026-10-20T14:00:00+06:00')).toBe(true)
		expect(sameValue('due_date', '0001-01-01T00:00:00Z', '')).toBe(true)
		expect(sameValue('title', 'a', 'b')).toBe(false)
	})

	it('lets a delete through only for an unchanged task', () => {
		expect(deleteStillApplies(BASE_UPDATED, {updated: '2026-10-14T09:00:00Z'})).toBe(true)
		expect(deleteStillApplies(BASE_UPDATED, {updated: '2026-10-14T09:30:00Z'})).toBe(false)
	})
})
