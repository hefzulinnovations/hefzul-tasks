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
	"sort"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// FocusStatsMaxDays caps the range one stats request may cover.
const FocusStatsMaxDays = 366

// focusStreakLookbackDays bounds how far back the daily streak is searched.
const focusStreakLookbackDays = 400

// FocusStatsDay is the focus time of one calendar day in the user's time zone.
type FocusStatsDay struct {
	Date         string `json:"date" doc:"Calendar date, YYYY-MM-DD, in the user's time zone."`
	FocusSeconds int64  `json:"focus_seconds" doc:"Seconds spent in focus sessions that started on this day, pauses excluded."`
	Sessions     int64  `json:"sessions" doc:"Completed focus sessions that started on this day."`
}

// FocusStatsTask is the focus time spent on one task.
type FocusStatsTask struct {
	TaskID       int64  `json:"task_id" doc:"The task's id."`
	Title        string `json:"title" doc:"The task's title; empty if the task was deleted or you can no longer read it."`
	ProjectID    int64  `json:"project_id" doc:"The task's project; 0 if unknown."`
	FocusSeconds int64  `json:"focus_seconds" doc:"Seconds of focus on this task in the range."`
	Sessions     int64  `json:"sessions" doc:"Completed focus sessions on this task in the range."`
}

// FocusStatsProject is the focus time spent on tasks of one project.
type FocusStatsProject struct {
	ProjectID    int64  `json:"project_id" doc:"The project's id."`
	Title        string `json:"title" doc:"The project's title."`
	FocusSeconds int64  `json:"focus_seconds" doc:"Seconds of focus on this project's tasks in the range."`
}

// FocusStats summarises a user's focus sessions over a date range.
type FocusStats struct {
	From              string               `json:"from" doc:"First day of the range, YYYY-MM-DD."`
	To                string               `json:"to" doc:"Last day of the range (inclusive), YYYY-MM-DD."`
	Timezone          string               `json:"timezone" doc:"The time zone days are counted in: the user's setting, or the server default."`
	TotalFocusSeconds int64                `json:"total_focus_seconds" doc:"Seconds of focus in the range."`
	CompletedSessions int64                `json:"completed_sessions" doc:"Completed focus sessions in the range."`
	CurrentStreakDays int                  `json:"current_streak_days" doc:"Consecutive days, up to today, with at least one completed focus session. Today counts once it has one; until then the streak runs to yesterday."`
	Days              []*FocusStatsDay     `json:"days" doc:"One entry per day in the range, oldest first, including days with no focus."`
	Tasks             []*FocusStatsTask    `json:"tasks" doc:"Focus per task, most focused first. Sessions without a task are left out."`
	Projects          []*FocusStatsProject `json:"projects" doc:"Focus per project, most focused first, from the tasks you can still read."`
}

func focusUserLocation(s *xorm.Session, userID int64) (*time.Location, error) {
	u, err := user.GetUserByID(s, userID)
	if err != nil {
		return nil, err
	}
	if u.Timezone != "" {
		if loc, err := time.LoadLocation(u.Timezone); err == nil {
			return loc, nil
		}
	}
	return config.GetTimeZone(), nil
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// GetFocusStats computes focus stats for the days from..to (inclusive, in the
// user's time zone). Zero dates default to the last 7 days.
func GetFocusStats(s *xorm.Session, a web.Auth, from, to time.Time) (*FocusStats, error) {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return nil, err
	}
	loc, err := focusUserLocation(s, userID)
	if err != nil {
		return nil, err
	}

	today := dayStart(time.Now().In(loc))
	if to.IsZero() {
		to = today
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -6)
	}
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	if to.Before(from) || to.Sub(from) > (FocusStatsMaxDays-1)*24*time.Hour+time.Hour {
		return nil, ErrInvalidFocusSettings{Field: "date range in days", Min: 1, Max: FocusStatsMaxDays}
	}
	end := to.AddDate(0, 0, 1)

	sessions := []*FocusSession{}
	err = s.Where(builder.Eq{"user_id": userID, "kind": FocusSessionKindFocus}).
		And(builder.In("status", FocusSessionStatusCompleted, FocusSessionStatusAbandoned)).
		And(builder.Gte{"started_at": from}).
		And(builder.Lt{"started_at": end}).
		Find(&sessions)
	if err != nil {
		return nil, err
	}

	stats := &FocusStats{
		From:     from.Format(time.DateOnly),
		To:       to.Format(time.DateOnly),
		Timezone: loc.String(),
		Days:     []*FocusStatsDay{},
		Tasks:    []*FocusStatsTask{},
		Projects: []*FocusStatsProject{},
	}

	days := map[string]*FocusStatsDay{}
	for d := from; d.Before(end); d = d.AddDate(0, 0, 1) {
		day := &FocusStatsDay{Date: d.Format(time.DateOnly)}
		days[day.Date] = day
		stats.Days = append(stats.Days, day)
	}

	tasks := map[int64]*FocusStatsTask{}
	for _, fs := range sessions {
		completed := int64(0)
		if fs.Status == FocusSessionStatusCompleted {
			completed = 1
		}
		stats.TotalFocusSeconds += fs.ActualSeconds
		stats.CompletedSessions += completed
		if day, ok := days[fs.StartedAt.In(loc).Format(time.DateOnly)]; ok {
			day.FocusSeconds += fs.ActualSeconds
			day.Sessions += completed
		}
		if fs.TaskID == 0 {
			continue
		}
		t, ok := tasks[fs.TaskID]
		if !ok {
			t = &FocusStatsTask{TaskID: fs.TaskID}
			tasks[fs.TaskID] = t
			stats.Tasks = append(stats.Tasks, t)
		}
		t.FocusSeconds += fs.ActualSeconds
		t.Sessions += completed
	}

	if err := addFocusStatsTitles(s, a, stats, tasks); err != nil {
		return nil, err
	}

	stats.CurrentStreakDays, err = focusStreak(s, userID, loc, today)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// addFocusStatsTitles fills task and project titles, only for tasks the user can still read.
func addFocusStatsTitles(s *xorm.Session, a web.Auth, stats *FocusStats, tasks map[int64]*FocusStatsTask) error {
	if len(tasks) > 0 {
		ids := make([]int64, 0, len(tasks))
		for id := range tasks {
			ids = append(ids, id)
		}
		accessible, err := accessibleProjectIDsCond(s, a, "project_id")
		if err != nil {
			return err
		}
		readable := []*Task{}
		if err := s.Where(builder.In("id", ids)).And(accessible).Find(&readable); err != nil {
			return err
		}

		projects := map[int64]*FocusStatsProject{}
		for _, task := range readable {
			t := tasks[task.ID]
			t.Title = task.Title
			t.ProjectID = task.ProjectID
			p, ok := projects[task.ProjectID]
			if !ok {
				p = &FocusStatsProject{ProjectID: task.ProjectID}
				projects[task.ProjectID] = p
				stats.Projects = append(stats.Projects, p)
			}
			p.FocusSeconds += t.FocusSeconds
		}

		if len(projects) > 0 {
			projectIDs := make([]int64, 0, len(projects))
			for id := range projects {
				projectIDs = append(projectIDs, id)
			}
			found := []*Project{}
			if err := s.Where(builder.In("id", projectIDs)).Find(&found); err != nil {
				return err
			}
			for _, p := range found {
				projects[p.ID].Title = p.Title
			}
		}
	}

	sort.SliceStable(stats.Tasks, func(i, j int) bool {
		if stats.Tasks[i].FocusSeconds != stats.Tasks[j].FocusSeconds {
			return stats.Tasks[i].FocusSeconds > stats.Tasks[j].FocusSeconds
		}
		return stats.Tasks[i].TaskID < stats.Tasks[j].TaskID
	})
	sort.SliceStable(stats.Projects, func(i, j int) bool {
		if stats.Projects[i].FocusSeconds != stats.Projects[j].FocusSeconds {
			return stats.Projects[i].FocusSeconds > stats.Projects[j].FocusSeconds
		}
		return stats.Projects[i].ProjectID < stats.Projects[j].ProjectID
	})
	return nil
}

// focusStreak counts consecutive days with a completed focus session, ending
// today, or yesterday when today has none yet.
func focusStreak(s *xorm.Session, userID int64, loc *time.Location, today time.Time) (int, error) {
	starts := []*FocusSession{}
	err := s.
		Where(builder.Eq{"user_id": userID, "kind": FocusSessionKindFocus, "status": FocusSessionStatusCompleted}).
		And(builder.Gte{"started_at": today.AddDate(0, 0, -focusStreakLookbackDays)}).
		Cols("started_at").
		Find(&starts)
	if err != nil {
		return 0, err
	}

	active := map[string]bool{}
	for _, st := range starts {
		active[st.StartedAt.In(loc).Format(time.DateOnly)] = true
	}

	day := today
	if !active[day.Format(time.DateOnly)] {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for active[day.Format(time.DateOnly)] {
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak, nil
}
