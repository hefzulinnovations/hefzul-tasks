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
	"fmt"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// focusEndGrace gives an open app the chance to finish the session itself
// (and ring locally) before the server pushes to the user's other devices.
const focusEndGrace = 10 * time.Second

// sentPushes remembers which reminders went out, so each is sent once.
type sentPushes struct {
	mu   sync.Mutex
	keys map[string]time.Time
}

func (sp *sentPushes) firstTime(key string, now time.Time) bool {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.keys == nil {
		sp.keys = map[string]time.Time{}
	}
	if _, sent := sp.keys[key]; sent {
		return false
	}
	sp.keys[key] = now
	for k, at := range sp.keys {
		if now.Sub(at) > 48*time.Hour {
			delete(sp.keys, k)
		}
	}
	return true
}

var (
	habitRemindersSent sentPushes
	focusEndsSent      sentPushes
)

// RegisterWebPushJobs creates the VAPID keys if needed and starts the habit
// reminder cron and the focus-session end check.
func RegisterWebPushJobs() {
	s := db.NewSession()
	if _, err := GetVAPIDKeys(s); err != nil {
		_ = s.Rollback()
		s.Close()
		log.Errorf("Could not create Web Push keys: %s", err)
		return
	}
	if err := s.Commit(); err != nil {
		log.Errorf("Could not store Web Push keys: %s", err)
	}
	s.Close()

	if err := cron.Schedule("* * * * *", func() { runPushJob("habit reminders", sendHabitReminders) }); err != nil {
		log.Fatalf("Could not register habit reminder cron: %s", err)
	}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			runPushJob("focus session ends", sendFocusEndPushes)
		}
	}()
}

func runPushJob(name string, job func(*xorm.Session, time.Time) error) {
	s := db.NewSession()
	defer s.Close()
	if err := job(s, time.Now()); err != nil {
		_ = s.Rollback()
		log.Errorf("[Web Push] %s: %s", name, err)
		return
	}
	if err := s.Commit(); err != nil {
		log.Errorf("[Web Push] %s: %s", name, err)
	}
}

// sendHabitReminders pushes, at each habit's reminder time in its owner's time
// zone, a reminder for habits that are due today and not done yet.
func sendHabitReminders(s *xorm.Session, now time.Time) error {
	habits := []*Habit{}
	err := s.Where(builder.Neq{"reminder_time": ""}).
		And(builder.NotNull{"reminder_time"}).
		And(builder.Eq{"archived": false}).
		And(builder.In("user_id", builder.Select("user_id").From("push_subscriptions"))).
		Find(&habits)
	if err != nil || len(habits) == 0 {
		return err
	}

	byUser := map[int64][]*Habit{}
	for _, h := range habits {
		byUser[h.UserID] = append(byUser[h.UserID], h)
	}
	for userID, userHabits := range byUser {
		loc, err := focusUserLocation(s, userID)
		if err != nil {
			return err
		}
		local := now.In(loc)
		hhmm := local.Format("15:04")
		due := []*Habit{}
		for _, h := range userHabits {
			if h.ReminderTime == hhmm {
				due = append(due, h)
			}
		}
		if len(due) == 0 {
			continue
		}
		if err := addHabitProgress(s, userID, due); err != nil {
			return err
		}
		for _, h := range due {
			if !h.DueToday || h.DoneToday {
				continue
			}
			if !habitRemindersSent.firstTime(fmt.Sprintf("%d-%s", h.ID, local.Format(time.DateOnly)), now) {
				continue
			}
			if _, err := sendPushToUser(s, userID, &pushContent{
				kind: "habit.reminder", id: h.ID, url: "/habits", tag: fmt.Sprintf("habit-%d", h.ID),
				genericBody: "Time for one of your habits.", title: "Habit reminder", detail: h.Title,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// sendFocusEndPushes notifies about running sessions whose time is up but that
// no open app has finished, such as when the phone app was closed.
func sendFocusEndPushes(s *xorm.Session, now time.Time) error {
	sessions := []*FocusSession{}
	err := s.Where(builder.Eq{"status": FocusSessionStatusRunning}).
		And(builder.In("user_id", builder.Select("user_id").From("push_subscriptions"))).
		Find(&sessions)
	if err != nil {
		return err
	}
	for _, fs := range sessions {
		overdue := time.Duration(fs.focusSeconds(now)-fs.PlannedSeconds) * time.Second
		if overdue < focusEndGrace {
			continue
		}
		if !focusEndsSent.firstTime(fmt.Sprintf("%d", fs.ID), now) {
			continue
		}
		content := &pushContent{kind: "focus.ended", id: fs.ID, url: "/focus", tag: "focus"}
		if fs.Kind == FocusSessionKindFocus {
			content.genericBody, content.title, content.detail = "Focus session complete. Time for a break.", "Focus session complete", "Time for a break."
		} else {
			content.genericBody, content.title, content.detail = "Break is over. Ready to focus again?", "Break is over", "Ready to focus again?"
		}
		if _, err := sendPushToUser(s, fs.UserID, content); err != nil {
			return err
		}
	}
	return nil
}
