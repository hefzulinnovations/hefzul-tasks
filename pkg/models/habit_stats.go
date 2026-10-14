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
	"math"
	"time"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// habitHistoryDays bounds how far back streaks are computed.
const habitHistoryDays = 730

// HabitStatsDay is one day of a habit's history.
type HabitStatsDay struct {
	Date  string `json:"date" doc:"Local calendar date, YYYY-MM-DD."`
	Count int64  `json:"count" doc:"Check-ins on that day."`
	Done  bool   `json:"done" doc:"True when count reached the daily target."`
}

// HabitStatsMonth is a habit's completion in one calendar month.
type HabitStatsMonth struct {
	Month    string  `json:"month" doc:"The month, YYYY-MM."`
	DoneDays int     `json:"done_days" doc:"Days the habit was done."`
	DueDays  int     `json:"due_days" doc:"Days the schedule asked for it, counted from the habit's creation up to today."`
	Rate     float64 `json:"rate" doc:"done_days / due_days, from 0 to 1 (capped at 1)."`
}

// HabitStats is a habit's streaks and history.
type HabitStats struct {
	HabitID       int64              `json:"habit_id" doc:"The habit."`
	CurrentStreak int                `json:"current_streak" doc:"Current streak: days for daily and weekdays habits, weeks for times_per_week habits."`
	BestStreak    int                `json:"best_streak" doc:"Longest streak in the last two years, in the same unit."`
	StreakUnit    string             `json:"streak_unit" enum:"days,weeks" doc:"days or weeks."`
	TotalCheckins int64              `json:"total_checkins" doc:"All check-ins in the last two years."`
	Days          []*HabitStatsDay   `json:"days" doc:"Every day from from to to, oldest first, for the calendar heatmap."`
	Months        []*HabitStatsMonth `json:"months" doc:"Completion per month for the months touched by from..to, oldest first."`
}

// habitHistory is one habit's check-ins indexed by local date.
type habitHistory struct {
	habit     *Habit
	counts    map[string]int64
	today     time.Time
	weekStart time.Weekday
	start     time.Time
}

func (hh *habitHistory) done(day time.Time) bool {
	return hh.counts[day.Format(time.DateOnly)] >= hh.habit.target()
}

func (hh *habitHistory) weekBegin(day time.Time) time.Time {
	offset := (int(day.Weekday()) - int(hh.weekStart) + 7) % 7
	return day.AddDate(0, 0, -offset)
}

func (hh *habitHistory) doneDaysInWeek(begin time.Time) int {
	n := 0
	for i := 0; i < 7; i++ {
		if hh.done(begin.AddDate(0, 0, i)) {
			n++
		}
	}
	return n
}

func (hh *habitHistory) weekDone(begin time.Time) bool {
	return hh.doneDaysInWeek(begin) >= hh.habit.TimesPerWeek
}

func (hh *habitHistory) currentStreak() int {
	if hh.habit.ScheduleKind == HabitScheduleTimesPerWeek {
		week := hh.weekBegin(hh.today)
		if !hh.weekDone(week) {
			week = week.AddDate(0, 0, -7)
		}
		streak := 0
		for !week.Before(hh.weekBegin(hh.start)) && hh.weekDone(week) {
			streak++
			week = week.AddDate(0, 0, -7)
		}
		return streak
	}

	day := hh.today
	// Today still counts as pending until it is done.
	if !hh.done(day) {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for !day.Before(hh.start) {
		if hh.habit.scheduledOn(day.Weekday()) {
			if !hh.done(day) {
				break
			}
			streak++
		}
		day = day.AddDate(0, 0, -1)
	}
	return streak
}

func (hh *habitHistory) bestStreak() int {
	best, run := 0, 0
	if hh.habit.ScheduleKind == HabitScheduleTimesPerWeek {
		for week := hh.weekBegin(hh.start); !week.After(hh.today); week = week.AddDate(0, 0, 7) {
			if hh.weekDone(week) {
				run++
				best = max(best, run)
			} else if hh.weekBegin(hh.today) != week {
				run = 0
			}
		}
		return best
	}
	for day := hh.start; !day.After(hh.today); day = day.AddDate(0, 0, 1) {
		if !hh.habit.scheduledOn(day.Weekday()) {
			continue
		}
		if hh.done(day) {
			run++
			best = max(best, run)
		} else if !day.Equal(hh.today) {
			run = 0
		}
	}
	return best
}

func (hh *habitHistory) dueToday() bool {
	if hh.habit.ScheduleKind == HabitScheduleTimesPerWeek {
		return !hh.weekDone(hh.weekBegin(hh.today))
	}
	return hh.habit.scheduledOn(hh.today.Weekday())
}

type habitClock struct {
	today     time.Time
	weekStart time.Weekday
}

func habitClockForUser(s *xorm.Session, userID int64) (*habitClock, error) {
	u, err := user.GetUserByID(s, userID)
	if err != nil {
		return nil, err
	}
	loc, err := focusUserLocation(s, userID)
	if err != nil {
		return nil, err
	}
	return &habitClock{today: dayStart(time.Now().In(loc)), weekStart: time.Weekday(u.WeekStart % 7)}, nil
}

// loadHabitHistories reads the last two years of check-ins for the habits.
func loadHabitHistories(s *xorm.Session, clock *habitClock, habits []*Habit) (map[int64]*habitHistory, error) {
	histories := make(map[int64]*habitHistory, len(habits))
	if len(habits) == 0 {
		return histories, nil
	}
	ids := make([]int64, 0, len(habits))
	earliest := clock.today.AddDate(0, 0, -habitHistoryDays)
	for _, h := range habits {
		ids = append(ids, h.ID)
		start := dayStart(h.Created.In(clock.today.Location()))
		if start.Before(earliest) || h.Created.IsZero() {
			start = earliest
		}
		if start.After(clock.today) {
			start = clock.today
		}
		histories[h.ID] = &habitHistory{habit: h, counts: map[string]int64{}, today: clock.today, weekStart: clock.weekStart, start: start}
	}

	checkins := []*HabitCheckin{}
	err := s.Where(builder.In("habit_id", ids)).
		And(builder.Gte{"date": earliest.Format(time.DateOnly)}).
		Find(&checkins)
	if err != nil {
		return nil, err
	}
	for _, c := range checkins {
		hh := histories[c.HabitID]
		date := calendarDate(c.Date)
		hh.counts[date] = c.Count
		// A check-in made for a day before the habit existed still counts.
		if d, err := time.ParseInLocation(time.DateOnly, date, clock.today.Location()); err == nil && d.Before(hh.start) {
			hh.start = d
		}
	}
	return histories, nil
}

// addHabitProgress fills today's progress and the current streak.
func addHabitProgress(s *xorm.Session, userID int64, habits []*Habit) error {
	if len(habits) == 0 {
		return nil
	}
	clock, err := habitClockForUser(s, userID)
	if err != nil {
		return err
	}
	histories, err := loadHabitHistories(s, clock, habits)
	if err != nil {
		return err
	}
	for _, h := range habits {
		hh := histories[h.ID]
		h.TodayCount = hh.counts[clock.today.Format(time.DateOnly)]
		h.DoneToday = hh.done(clock.today)
		h.DueToday = hh.dueToday()
		h.CurrentStreak = hh.currentStreak()
	}
	return nil
}

// GetHabitStats returns streaks, the day-by-day history from..to (local dates,
// default the last 12 months) and completion per month.
func GetHabitStats(s *xorm.Session, a web.Auth, habitID int64, from, to time.Time) (*HabitStats, error) {
	h, err := getOwnHabit(s, a, habitID)
	if err != nil {
		return nil, err
	}
	clock, err := habitClockForUser(s, h.UserID)
	if err != nil {
		return nil, err
	}
	loc := clock.today.Location()
	if to.IsZero() {
		to = clock.today
	}
	if from.IsZero() {
		from = to.AddDate(-1, 0, 1)
	}
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	if to.Before(from) || to.Sub(from) > 400*24*time.Hour {
		return nil, ErrInvalidHabit{Field: "from/to", Reason: "the range must cover 1 to 400 days"}
	}

	histories, err := loadHabitHistories(s, clock, []*Habit{h})
	if err != nil {
		return nil, err
	}
	hh := histories[h.ID]

	stats := &HabitStats{
		HabitID:       h.ID,
		CurrentStreak: hh.currentStreak(),
		BestStreak:    hh.bestStreak(),
		StreakUnit:    "days",
		Days:          []*HabitStatsDay{},
		Months:        []*HabitStatsMonth{},
	}
	if h.ScheduleKind == HabitScheduleTimesPerWeek {
		stats.StreakUnit = "weeks"
	}
	for _, c := range hh.counts {
		stats.TotalCheckins += c
	}

	months := map[string]*HabitStatsMonth{}
	monthDays := map[string]int{}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		key := day.Format(time.DateOnly)
		entry := &HabitStatsDay{Date: key, Count: hh.counts[key], Done: hh.done(day)}
		stats.Days = append(stats.Days, entry)

		month := day.Format("2006-01")
		m, ok := months[month]
		if !ok {
			m = &HabitStatsMonth{Month: month}
			months[month] = m
			stats.Months = append(stats.Months, m)
		}
		if day.After(clock.today) || day.Before(hh.start) {
			continue
		}
		monthDays[month]++
		if entry.Done {
			m.DoneDays++
		}
		if h.ScheduleKind != HabitScheduleTimesPerWeek && h.scheduledOn(day.Weekday()) {
			m.DueDays++
		}
	}
	for _, m := range stats.Months {
		if h.ScheduleKind == HabitScheduleTimesPerWeek {
			m.DueDays = int(math.Round(float64(h.TimesPerWeek) * float64(monthDays[m.Month]) / 7))
		}
		if m.DueDays > 0 {
			m.Rate = math.Min(1, float64(m.DoneDays)/float64(m.DueDays))
		}
	}
	return stats, nil
}
