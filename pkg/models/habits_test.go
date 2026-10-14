// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package models

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/habits.yml): user1 owns 1 (daily), 2 (daily, target
// 8), 3 (weekdays Mon/Wed/Fri), 4 (3 times per week) and 6 (archived); habit 5
// belongs to user2.

func checkinOn(t *testing.T, habitID int64, date time.Time, count int64) {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	_, err := s.Insert(&HabitCheckin{HabitID: habitID, Date: date.Format(time.DateOnly), Count: count})
	require.NoError(t, err)
	require.NoError(t, s.Commit())
}

func habitStatsFor(t *testing.T, habitID int64) *HabitStats {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	stats, err := GetHabitStats(s, &user.User{ID: 1}, habitID, time.Time{}, time.Time{})
	require.NoError(t, err)
	return stats
}

func todayForUser1(t *testing.T) time.Time {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	clock, err := habitClockForUser(s, 1)
	require.NoError(t, err)
	return clock.today
}

func TestHabit_Permissions(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	can, _, err := (&Habit{ID: 1}).CanRead(s, &user.User{ID: 1})
	require.NoError(t, err)
	assert.True(t, can)

	_, _, err = (&Habit{ID: 1}).CanRead(s, &user.User{ID: 2})
	assert.True(t, IsErrHabitDoesNotExist(err), "another user's habit reads as missing")

	can, err = (&Habit{ID: 1}).CanUpdate(s, &user.User{ID: 2})
	assert.False(t, can)
	assert.True(t, IsErrHabitDoesNotExist(err))

	can, err = (&Habit{ID: 1}).CanDelete(s, &user.User{ID: 1})
	require.NoError(t, err)
	assert.True(t, can)

	can, err = (&Habit{}).CanCreate(s, &LinkSharing{ID: 1})
	require.NoError(t, err)
	assert.False(t, can, "link shares have no habits")

	can, _, err = (&Habit{ID: 1}).CanRead(s, &LinkSharing{ID: 1})
	require.NoError(t, err)
	assert.False(t, can)
}

func TestHabit_ReadAll(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	result, _, total, err := (&Habit{}).ReadAll(s, &user.User{ID: 1}, "", 1, 50)
	require.NoError(t, err)
	habits := result.([]*Habit)
	assert.Equal(t, int64(4), total, "archived habit left out")
	assert.Equal(t, "Read 20 pages", habits[0].Title)

	result, _, total, err = (&Habit{IncludeArchived: true}).ReadAll(s, &user.User{ID: 1}, "", 1, 50)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, result.([]*Habit), 5)
}

func TestHabit_CreateValidates(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	err := (&Habit{Title: "x", ScheduleKind: HabitScheduleWeekdays}).Create(s, u)
	assert.True(t, IsErrInvalidHabit(err), "weekdays habit needs days")
	err = (&Habit{Title: "x", ScheduleKind: HabitScheduleTimesPerWeek, TimesPerWeek: 9}).Create(s, u)
	assert.True(t, IsErrInvalidHabit(err))
	err = (&Habit{Title: "x", ReminderTime: "25:00"}).Create(s, u)
	assert.True(t, IsErrInvalidHabit(err))

	h := &Habit{Title: "Meditate", HexColor: "#ff0000", ReminderTime: "07:30"}
	require.NoError(t, h.Create(s, u))
	assert.Equal(t, HabitScheduleDaily, h.ScheduleKind)
	assert.Equal(t, "ff0000", h.HexColor)
	assert.Equal(t, int64(1), h.UserID)
}

func TestHabit_CheckinAndUndo(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	first, err := CheckInHabit(s, u, 2, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), first.Count)
	second, err := CheckInHabit(s, u, 2, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), second.Count, "a second check-in on the same day adds to the count")
	assert.Equal(t, first.ID, second.ID)

	h := &Habit{ID: 2}
	_, _, err = h.CanRead(s, u)
	require.NoError(t, err)
	require.NoError(t, h.ReadOne(s, u))
	assert.Equal(t, int64(2), h.TodayCount)
	assert.False(t, h.DoneToday, "target is 8")

	undone, err := UndoHabitCheckin(s, u, 2, second.Date)
	require.NoError(t, err)
	assert.Equal(t, int64(1), undone.Count)
	undone, err = UndoHabitCheckin(s, u, 2, second.Date)
	require.NoError(t, err)
	assert.Equal(t, int64(0), undone.Count)
	_, err = UndoHabitCheckin(s, u, 2, second.Date)
	assert.True(t, IsErrHabitCheckinDoesNotExist(err))

	_, err = CheckInHabit(s, u, 5, "")
	assert.True(t, IsErrHabitDoesNotExist(err), "cannot check in on another user's habit")
	_, err = CheckInHabit(s, u, 1, "14-10-2026")
	assert.True(t, IsErrInvalidHabit(err))
}

func TestHabit_DailyStreak(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	today := todayForUser1(t)
	for _, daysAgo := range []int{1, 2, 3, 5} {
		checkinOn(t, 1, today.AddDate(0, 0, -daysAgo), 1)
	}

	stats := habitStatsFor(t, 1)
	assert.Equal(t, 3, stats.CurrentStreak, "today is still pending, so the streak counts back from yesterday")
	assert.Equal(t, 3, stats.BestStreak)
	assert.Equal(t, "days", stats.StreakUnit)
	assert.Equal(t, today.Format(time.DateOnly), stats.Days[len(stats.Days)-1].Date)

	checkinOn(t, 1, today, 1)
	stats = habitStatsFor(t, 1)
	assert.Equal(t, 4, stats.CurrentStreak, "done today extends the streak")
}

func TestHabit_DailyTarget(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	today := todayForUser1(t)
	checkinOn(t, 2, today.AddDate(0, 0, -1), 8)
	checkinOn(t, 2, today.AddDate(0, 0, -2), 3)

	stats := habitStatsFor(t, 2)
	assert.Equal(t, 1, stats.CurrentStreak, "a day below the target breaks the streak")
	assert.Equal(t, int64(11), stats.TotalCheckins)
}

func TestHabit_WeekdaysStreakSkipsUnscheduledDays(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	today := todayForUser1(t)
	// Check in on the last three scheduled days (Mon/Wed/Fri) before today.
	done := 0
	for day := today.AddDate(0, 0, -1); done < 3; day = day.AddDate(0, 0, -1) {
		if wd := day.Weekday(); wd == time.Monday || wd == time.Wednesday || wd == time.Friday {
			checkinOn(t, 3, day, 1)
			done++
		}
	}

	stats := habitStatsFor(t, 3)
	assert.Equal(t, 3, stats.CurrentStreak, "days off the schedule neither count nor break the streak")
}

func TestHabit_TimesPerWeekStreak(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	clock, err := habitClockForUser(s, 1)
	require.NoError(t, err)
	s.Close()
	hh := &habitHistory{weekStart: clock.weekStart}
	thisWeek := hh.weekBegin(clock.today)
	for _, week := range []int{1, 2} {
		begin := thisWeek.AddDate(0, 0, -7*week)
		for i := 0; i < 3; i++ {
			checkinOn(t, 4, begin.AddDate(0, 0, i), 1)
		}
	}
	// Two days in week 3 back: not enough.
	checkinOn(t, 4, thisWeek.AddDate(0, 0, -21), 1)
	checkinOn(t, 4, thisWeek.AddDate(0, 0, -20), 1)

	stats := habitStatsFor(t, 4)
	assert.Equal(t, "weeks", stats.StreakUnit)
	assert.Equal(t, 2, stats.CurrentStreak, "this week is still open; the last two full weeks met the target")

	s = db.NewSession()
	defer s.Close()
	h := &Habit{ID: 4}
	_, _, err = h.CanRead(s, &user.User{ID: 1})
	require.NoError(t, err)
	require.NoError(t, h.ReadOne(s, &user.User{ID: 1}))
	assert.True(t, h.DueToday, "the week's target is not met yet")
}

func TestHabit_Delete(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	require.NoError(t, (&Habit{ID: 1}).Delete(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())
	db.AssertMissing(t, "habits", map[string]any{"id": 1})
	db.AssertMissing(t, "habit_checkins", map[string]any{"habit_id": 1})
}

func TestNextOccurrence(t *testing.T) {
	today := time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC)
	birthday := time.Date(1990, 3, 15, 0, 0, 0, 0, time.UTC)

	next, years := nextOccurrence(birthday, true, today)
	assert.Equal(t, "2027-03-15", next.Format(time.DateOnly))
	assert.Equal(t, 37, years)

	next, _ = nextOccurrence(birthday, true, time.Date(2027, 3, 16, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, "2028-03-15", next.Format(time.DateOnly), "rolls over once the day has passed")

	next, _ = nextOccurrence(birthday, true, time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, "2027-03-15", next.Format(time.DateOnly), "today is still the occurrence")

	leap := time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC)
	next, _ = nextOccurrence(leap, true, today.AddDate(0, -2, 0))
	assert.Equal(t, "2027-02-28", next.Format(time.DateOnly), "29 February falls on the 28th in other years")

	oneOff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	next, years = nextOccurrence(oneOff, false, today)
	assert.Equal(t, oneOff, next)
	assert.Equal(t, 0, years)
}

func TestCountdown_ReadAll(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	result, _, total, err := (&Countdown{}).ReadAll(s, u, "", 1, 50)
	require.NoError(t, err)
	countdowns := result.([]*Countdown)
	assert.Equal(t, int64(3), total, "only the user's own")
	assert.Equal(t, "Old deadline", countdowns[2].Title, "passed one-off dates come last")
	assert.Negative(t, countdowns[2].DaysRemaining)
	for _, c := range countdowns[:2] {
		assert.GreaterOrEqual(t, c.DaysRemaining, 0)
	}
	assert.LessOrEqual(t, countdowns[0].DaysRemaining, countdowns[1].DaysRemaining)

	result, _, _, err = (&Countdown{WithinDays: 7}).ReadAll(s, u, "", 1, 50)
	require.NoError(t, err)
	titles := []string{}
	for _, c := range result.([]*Countdown) {
		if c.DaysRemaining > 7 {
			assert.True(t, c.Pinned, "only pinned countdowns may be further out")
		}
		titles = append(titles, c.Title)
	}
	assert.Contains(t, titles, "Exam", "pinned countdowns always show")
	assert.NotContains(t, titles, "Old deadline")
}

func TestCountdown_Permissions(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	can, _, err := (&Countdown{ID: 4}).CanRead(s, &user.User{ID: 1})
	assert.False(t, can)
	assert.True(t, IsErrCountdownDoesNotExist(err))

	c := &Countdown{ID: 1}
	can, _, err = c.CanRead(s, &user.User{ID: 1})
	require.NoError(t, err)
	assert.True(t, can)
	require.NoError(t, c.ReadOne(s, &user.User{ID: 1}))
	assert.Greater(t, c.Years, 30, "a 1990 birthday is over 30 years ago")

	err = (&Countdown{Title: "x", TargetDate: "tomorrow"}).Create(s, &user.User{ID: 1})
	assert.True(t, IsErrInvalidCountdown(err))
}
