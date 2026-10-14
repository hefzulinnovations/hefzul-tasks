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

// Fixtures (pkg/db/fixtures/focus_sessions.yml): user1 has finished sessions 1
// (focus on task 1, completed), 2 (focus, abandoned), 3 (short break) and 5
// (focus on task 2); session 4 belongs to user2. Only user2 has saved settings.
// Task 1 is in project 1 (user1 only); user4 cannot read it.

func TestFocusSettings(t *testing.T) {
	t.Run("defaults when never saved", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		settings, err := GetFocusSettings(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.Equal(t, 25, settings.FocusMinutes)
		assert.Equal(t, 5, settings.ShortBreakMinutes)
		assert.Equal(t, 15, settings.LongBreakMinutes)
		assert.Equal(t, 3, settings.SessionsBeforeLongBreak)
	})
	t.Run("saved settings are returned", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		settings, err := GetFocusSettings(s, &user.User{ID: 2})
		require.NoError(t, err)
		assert.Equal(t, 50, settings.FocusMinutes)
		assert.True(t, settings.AutoStartBreaks)
	})
	t.Run("save inserts then updates", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u := &user.User{ID: 1}
		in := defaultFocusSettings(0)
		in.FocusMinutes = 40
		require.NoError(t, SaveFocusSettings(s, u, in))
		in.ShortBreakMinutes = 7
		require.NoError(t, SaveFocusSettings(s, u, in))
		require.NoError(t, s.Commit())

		db.AssertExists(t, "focus_settings", map[string]any{"user_id": 1, "focus_minutes": 40, "short_break_minutes": 7}, false)
	})
	t.Run("out of range is rejected", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		in := defaultFocusSettings(0)
		in.FocusMinutes = 0
		err := SaveFocusSettings(s, &user.User{ID: 1}, in)
		require.Error(t, err)
		assert.True(t, IsErrInvalidFocusSettings(err))
	})
	t.Run("link share is forbidden", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := GetFocusSettings(s, &LinkSharing{ID: 1, ProjectID: 1})
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestFocusSession_Lifecycle(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	current, err := GetCurrentFocusSession(s, u)
	require.NoError(t, err)
	assert.Nil(t, current)

	fs := &FocusSession{TaskID: 1, Device: "web"}
	require.NoError(t, StartFocusSession(s, u, fs))
	assert.NotZero(t, fs.ID)
	assert.Equal(t, FocusSessionKindFocus, fs.Kind)
	assert.Equal(t, int64(25*60), fs.PlannedSeconds, "planned length comes from the default settings")
	assert.Equal(t, FocusSessionStatusRunning, fs.Status)

	current, err = GetCurrentFocusSession(s, u)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, fs.ID, current.ID)

	err = StartFocusSession(s, u, &FocusSession{})
	require.Error(t, err, "a second session cannot start while one runs")
	assert.True(t, IsErrFocusSessionAlreadyRunning(err))

	_, err = ResumeFocusSession(s, u, fs.ID)
	assert.True(t, IsErrFocusSessionInvalidState(err), "a running session cannot be resumed")

	paused, err := PauseFocusSession(s, u, fs.ID)
	require.NoError(t, err)
	assert.Equal(t, FocusSessionStatusPaused, paused.Status)
	assert.NotNil(t, paused.PausedAt)

	_, err = PauseFocusSession(s, u, fs.ID)
	assert.True(t, IsErrFocusSessionInvalidState(err), "a paused session cannot be paused again")

	resumed, err := ResumeFocusSession(s, u, fs.ID)
	require.NoError(t, err)
	assert.Equal(t, FocusSessionStatusRunning, resumed.Status)
	assert.Nil(t, resumed.PausedAt)

	finished, err := FinishFocusSession(s, u, fs.ID, FocusSessionStatusAbandoned)
	require.NoError(t, err)
	assert.Equal(t, FocusSessionStatusAbandoned, finished.Status)
	assert.NotNil(t, finished.EndedAt)

	_, err = FinishFocusSession(s, u, fs.ID, FocusSessionStatusCompleted)
	assert.True(t, IsErrFocusSessionInvalidState(err), "a finished session cannot be finished again")

	current, err = GetCurrentFocusSession(s, u)
	require.NoError(t, err)
	assert.Nil(t, current)

	require.NoError(t, StartFocusSession(s, u, &FocusSession{Kind: FocusSessionKindShortBreak}),
		"a new session can start once the last one is finished")
}

func TestFocusSession_Start(t *testing.T) {
	t.Run("task the user cannot read is forbidden", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := StartFocusSession(s, &user.User{ID: 4}, &FocusSession{TaskID: 1})
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
	t.Run("unknown kind is rejected", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := StartFocusSession(s, &user.User{ID: 1}, &FocusSession{Kind: "nap"})
		assert.True(t, IsErrInvalidFocusSessionKind(err))
	})
	t.Run("break length comes from the user's settings", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		fs := &FocusSession{Kind: FocusSessionKindLongBreak}
		require.NoError(t, StartFocusSession(s, &user.User{ID: 2}, fs))
		assert.Equal(t, int64(30*60), fs.PlannedSeconds)
	})
	t.Run("a running session whose time is up is completed automatically", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		u := &user.User{ID: 1}

		stale := &FocusSession{UserID: 1, Kind: FocusSessionKindFocus, PlannedSeconds: 60,
			Status: FocusSessionStatusRunning, StartedAt: time.Now().Add(-10 * time.Minute)}
		_, err := s.Insert(stale)
		require.NoError(t, err)

		require.NoError(t, StartFocusSession(s, u, &FocusSession{}))

		reloaded := &FocusSession{}
		_, err = s.ID(stale.ID).Get(reloaded)
		require.NoError(t, err)
		assert.Equal(t, FocusSessionStatusCompleted, reloaded.Status)
		assert.Equal(t, int64(60), reloaded.ActualSeconds, "a completed session counts at most its planned length")
	})
	t.Run("link share is forbidden", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := StartFocusSession(s, &LinkSharing{ID: 1, ProjectID: 1}, &FocusSession{})
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestFocusSession_OtherUsersSession(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	running := &FocusSession{}
	require.NoError(t, StartFocusSession(s, &user.User{ID: 2}, running))

	_, err := PauseFocusSession(s, &user.User{ID: 1}, running.ID)
	require.Error(t, err)
	assert.True(t, IsErrFocusSessionDoesNotExist(err), "another user's session reads as nonexistent")

	_, err = FinishFocusSession(s, &user.User{ID: 1}, running.ID, FocusSessionStatusAbandoned)
	assert.True(t, IsErrFocusSessionDoesNotExist(err))
}

func TestFocusSession_FocusSeconds(t *testing.T) {
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	pausedAt := start.Add(10 * time.Minute)
	fs := &FocusSession{StartedAt: start, PausedSeconds: 120, PausedAt: &pausedAt}
	assert.Equal(t, int64(8*60), fs.focusSeconds(start.Add(20*time.Minute)),
		"the current pause and earlier pauses are both excluded")
}

func TestListFocusSessions(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	all, total, err := ListFocusSessions(s, u, FocusSessionFilter{}, 1, 50)
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	ids := []int64{}
	for _, fs := range all {
		ids = append(ids, fs.ID)
	}
	assert.Equal(t, []int64{5, 2, 3, 1}, ids, "newest first, only the user's own")

	byTask, _, err := ListFocusSessions(s, u, FocusSessionFilter{TaskID: 1}, 1, 50)
	require.NoError(t, err)
	require.Len(t, byTask, 1)
	assert.Equal(t, int64(1), byTask[0].ID)

	byProject, _, err := ListFocusSessions(s, u, FocusSessionFilter{ProjectID: 1}, 1, 50)
	require.NoError(t, err)
	assert.Len(t, byProject, 2, "sessions on tasks 1 and 2, both in project 1")

	ranged, _, err := ListFocusSessions(s, u, FocusSessionFilter{
		From: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
	}, 1, 50)
	require.NoError(t, err)
	require.Len(t, ranged, 1)
	assert.Equal(t, int64(5), ranged[0].ID)
}

func TestGetFocusStats(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	from := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	stats, err := GetFocusStats(s, &user.User{ID: 1}, from, to)
	require.NoError(t, err)

	assert.Equal(t, "2026-09-30", stats.From)
	assert.Equal(t, "2026-10-04", stats.To)
	assert.Len(t, stats.Days, 5, "every day of the range, including empty ones")
	assert.Equal(t, int64(1500+600+1500), stats.TotalFocusSeconds, "focus sessions only, abandoned ones included, breaks left out")
	assert.Equal(t, int64(2), stats.CompletedSessions)

	var daySum int64
	for _, d := range stats.Days {
		daySum += d.FocusSeconds
	}
	assert.Equal(t, stats.TotalFocusSeconds, daySum)

	require.Len(t, stats.Tasks, 2)
	assert.Equal(t, "task #1", stats.Tasks[0].Title)
	assert.Equal(t, int64(1), stats.Tasks[0].ProjectID)
	require.Len(t, stats.Projects, 1)
	assert.Equal(t, int64(1), stats.Projects[0].ProjectID)
	assert.Equal(t, int64(3000), stats.Projects[0].FocusSeconds)

	t.Run("range over the limit is rejected", func(t *testing.T) {
		_, err := GetFocusStats(s, &user.User{ID: 1}, from, from.AddDate(2, 0, 0))
		require.Error(t, err)
	})
}

func TestFocusStreak(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	loc := time.UTC
	today := dayStart(time.Now().In(loc))
	for i, daysAgo := range []int{1, 2, 3, 5} {
		_, err := s.Insert(&FocusSession{
			UserID: 3, Kind: FocusSessionKindFocus, PlannedSeconds: 60, ActualSeconds: 60,
			Status: FocusSessionStatusCompleted, StartedAt: today.AddDate(0, 0, -daysAgo).Add(time.Duration(i+10) * time.Hour),
		})
		require.NoError(t, err)
	}

	streak, err := focusStreak(s, 3, loc, today)
	require.NoError(t, err)
	assert.Equal(t, 3, streak, "today has no session yet, so the streak runs back from yesterday")
}

func TestFocusSession_TaskHardDeleteKeepsHistory(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, hardDeleteTask(s, &Task{ID: 1}))
	require.NoError(t, s.Commit())

	db.AssertExists(t, "focus_sessions", map[string]any{"id": 1, "task_id": 0}, false)
}

func TestFocusSession_DeleteUserRemovesFocusData(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, deleteFocusDataForUser(s, 2))
	require.NoError(t, s.Commit())

	db.AssertMissing(t, "focus_sessions", map[string]any{"user_id": 2})
	db.AssertMissing(t, "focus_settings", map[string]any{"user_id": 2})
}
