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
	"time"

	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// FocusSessionKind is what a Pomodoro session is for: focused work or a break.
type FocusSessionKind string

const (
	FocusSessionKindFocus      FocusSessionKind = "focus"
	FocusSessionKindShortBreak FocusSessionKind = "short_break"
	FocusSessionKindLongBreak  FocusSessionKind = "long_break"
)

func (k FocusSessionKind) valid() bool {
	switch k {
	case FocusSessionKindFocus, FocusSessionKindShortBreak, FocusSessionKindLongBreak:
		return true
	}
	return false
}

// FocusSessionStatus is the lifecycle state of a focus session.
type FocusSessionStatus string

const (
	FocusSessionStatusRunning   FocusSessionStatus = "running"
	FocusSessionStatusPaused    FocusSessionStatus = "paused"
	FocusSessionStatusCompleted FocusSessionStatus = "completed"
	FocusSessionStatusAbandoned FocusSessionStatus = "abandoned"
)

func (st FocusSessionStatus) active() bool {
	return st == FocusSessionStatusRunning || st == FocusSessionStatusPaused
}

// FocusSession is one Pomodoro interval (focus or break) of one user. A user has
// at most one running or paused session at a time, enforced in StartFocusSession
// (MySQL has no partial unique index to do it in the schema).
//
// v2-only: doc: tags are the schema's source of truth.
type FocusSession struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this focus session."`
	UserID int64 `xorm:"bigint not null INDEX" json:"user_id" readOnly:"true" doc:"The user this session belongs to. Set by the server."`
	TaskID int64 `xorm:"bigint null INDEX" json:"task_id" doc:"The task worked on during this session, or 0 for none. You need read access to the task. Set to 0 when the task is deleted."`

	Kind           FocusSessionKind   `xorm:"varchar(20) not null" json:"kind" enum:"focus,short_break,long_break" doc:"focus for work, short_break or long_break for a break."`
	PlannedSeconds int64              `xorm:"bigint not null" json:"planned_seconds" minimum:"0" maximum:"14400" doc:"Length of the session in seconds. 0 or omitted uses the user's timer settings for this kind."`
	ActualSeconds  int64              `xorm:"bigint not null default 0" json:"actual_seconds" readOnly:"true" doc:"Seconds actually spent, excluding pauses. Set when the session ends."`
	Status         FocusSessionStatus `xorm:"varchar(20) not null INDEX" json:"status" readOnly:"true" enum:"running,paused,completed,abandoned" doc:"Lifecycle state. Changed only through the pause, resume and finish actions."`

	StartedAt     time.Time  `xorm:"not null INDEX" json:"started_at" readOnly:"true" doc:"When the session started (server time). Clients count down from this, not from their own clock."`
	EndedAt       *time.Time `xorm:"null" json:"ended_at" readOnly:"true" doc:"When the session was completed or abandoned. Null while it runs."`
	PausedAt      *time.Time `xorm:"null" json:"paused_at" readOnly:"true" doc:"When the current pause began. Null unless the session is paused."`
	PausedSeconds int64      `xorm:"bigint not null default 0" json:"paused_seconds" readOnly:"true" doc:"Seconds spent in earlier, already resumed pauses."`

	Device string `xorm:"varchar(100) null" json:"device" maxLength:"100" doc:"Free-form name of the device that started the session, such as web or android."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this session was created."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this session was last changed."`
}

// TableName is focus_sessions.
func (*FocusSession) TableName() string {
	return "focus_sessions"
}

// focusSeconds is the time spent in the session so far, pauses excluded.
func (fs *FocusSession) focusSeconds(now time.Time) int64 {
	end := now
	if fs.EndedAt != nil {
		end = *fs.EndedAt
	}
	if fs.PausedAt != nil && fs.PausedAt.Before(end) {
		end = *fs.PausedAt
	}
	elapsed := int64(end.Sub(fs.StartedAt)/time.Second) - fs.PausedSeconds
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func focusAuthUserID(a web.Auth) (int64, error) {
	// Link shares have no focus timer; their id is a share id, not a user id.
	if _, isShare := a.(*LinkSharing); isShare {
		return 0, ErrGenericForbidden{}
	}
	return a.GetID(), nil
}

// GetCurrentFocusSession returns the user's running or paused session, or nil.
func GetCurrentFocusSession(s *xorm.Session, a web.Auth) (*FocusSession, error) {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return nil, err
	}
	return currentFocusSessionForUser(s, userID)
}

func currentFocusSessionForUser(s *xorm.Session, userID int64) (*FocusSession, error) {
	current := &FocusSession{}
	exists, err := s.
		Where("user_id = ?", userID).
		And(builder.In("status", FocusSessionStatusRunning, FocusSessionStatusPaused)).
		OrderBy("started_at DESC").
		Get(current)
	if err != nil || !exists {
		return nil, err
	}
	return current, nil
}

// StartFocusSession starts fs for the authenticated user. A running session whose
// planned time has already run out (the app was closed before it could finish it)
// is completed first; any other running or paused session blocks the start.
func StartFocusSession(s *xorm.Session, a web.Auth, fs *FocusSession) error {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return err
	}

	if fs.Kind == "" {
		fs.Kind = FocusSessionKindFocus
	}
	if !fs.Kind.valid() {
		return ErrInvalidFocusSessionKind{Kind: fs.Kind}
	}

	if fs.TaskID != 0 {
		task := &Task{ID: fs.TaskID}
		can, _, err := task.CanRead(s, a)
		if err != nil {
			return err
		}
		if !can {
			return ErrGenericForbidden{}
		}
	}

	now := time.Now()
	current, err := currentFocusSessionForUser(s, userID)
	if err != nil {
		return err
	}
	if current != nil {
		if current.Status != FocusSessionStatusRunning || current.focusSeconds(now) < current.PlannedSeconds {
			return ErrFocusSessionAlreadyRunning{SessionID: current.ID}
		}
		if err := current.finish(s, FocusSessionStatusCompleted, now); err != nil {
			return err
		}
	}

	if fs.PlannedSeconds <= 0 {
		settings, err := focusSettingsForUser(s, userID)
		if err != nil {
			return err
		}
		fs.PlannedSeconds = settings.plannedSeconds(fs.Kind)
	}

	fs.ID = 0
	fs.UserID = userID
	fs.Status = FocusSessionStatusRunning
	fs.StartedAt = now
	fs.EndedAt = nil
	fs.PausedAt = nil
	fs.PausedSeconds = 0
	fs.ActualSeconds = 0
	if _, err := s.Insert(fs); err != nil {
		return err
	}

	events.DispatchOnCommit(s, &FocusSessionUpdatedEvent{Session: fs})
	return nil
}

// getOwnFocusSession loads a session of the authenticated user. Another user's
// session reads as nonexistent so its existence does not leak.
func getOwnFocusSession(s *xorm.Session, a web.Auth, id int64) (*FocusSession, error) {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return nil, err
	}
	fs := &FocusSession{}
	exists, err := s.Where("id = ? AND user_id = ?", id, userID).Get(fs)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrFocusSessionDoesNotExist{SessionID: id}
	}
	return fs, nil
}

// PauseFocusSession pauses a running session.
func PauseFocusSession(s *xorm.Session, a web.Auth, id int64) (*FocusSession, error) {
	fs, err := getOwnFocusSession(s, a, id)
	if err != nil {
		return nil, err
	}
	if fs.Status != FocusSessionStatusRunning {
		return nil, ErrFocusSessionInvalidState{SessionID: id, Status: fs.Status, Action: "pause"}
	}
	now := time.Now()
	fs.Status = FocusSessionStatusPaused
	fs.PausedAt = &now
	if _, err := s.ID(fs.ID).Cols("status", "paused_at").Update(fs); err != nil {
		return nil, err
	}
	events.DispatchOnCommit(s, &FocusSessionUpdatedEvent{Session: fs})
	return fs, nil
}

// ResumeFocusSession resumes a paused session; the pause is added to paused_seconds.
func ResumeFocusSession(s *xorm.Session, a web.Auth, id int64) (*FocusSession, error) {
	fs, err := getOwnFocusSession(s, a, id)
	if err != nil {
		return nil, err
	}
	if fs.Status != FocusSessionStatusPaused || fs.PausedAt == nil {
		return nil, ErrFocusSessionInvalidState{SessionID: id, Status: fs.Status, Action: "resume"}
	}
	now := time.Now()
	if paused := int64(now.Sub(*fs.PausedAt) / time.Second); paused > 0 {
		fs.PausedSeconds += paused
	}
	fs.Status = FocusSessionStatusRunning
	fs.PausedAt = nil
	if _, err := s.ID(fs.ID).Cols("status", "paused_at", "paused_seconds").Update(fs); err != nil {
		return nil, err
	}
	events.DispatchOnCommit(s, &FocusSessionUpdatedEvent{Session: fs})
	return fs, nil
}

// FinishFocusSession ends a running or paused session as completed or abandoned.
func FinishFocusSession(s *xorm.Session, a web.Auth, id int64, status FocusSessionStatus) (*FocusSession, error) {
	if status != FocusSessionStatusCompleted && status != FocusSessionStatusAbandoned {
		return nil, ErrFocusSessionInvalidState{SessionID: id, Status: status, Action: "be finished with this status"}
	}
	fs, err := getOwnFocusSession(s, a, id)
	if err != nil {
		return nil, err
	}
	if !fs.Status.active() {
		return nil, ErrFocusSessionInvalidState{SessionID: id, Status: fs.Status, Action: "finish"}
	}
	if err := fs.finish(s, status, time.Now()); err != nil {
		return nil, err
	}
	return fs, nil
}

func (fs *FocusSession) finish(s *xorm.Session, status FocusSessionStatus, now time.Time) error {
	actual := fs.focusSeconds(now)
	// A completed session never counts more than planned: the device may report
	// the end late, after the app slept past the alarm.
	if status == FocusSessionStatusCompleted && actual > fs.PlannedSeconds {
		actual = fs.PlannedSeconds
	}
	if fs.PausedAt != nil {
		if paused := int64(now.Sub(*fs.PausedAt) / time.Second); paused > 0 {
			fs.PausedSeconds += paused
		}
		fs.PausedAt = nil
	}
	fs.Status = status
	fs.EndedAt = &now
	fs.ActualSeconds = actual
	if _, err := s.ID(fs.ID).Cols("status", "ended_at", "actual_seconds", "paused_at", "paused_seconds").Update(fs); err != nil {
		return err
	}

	events.DispatchOnCommit(s, &FocusSessionUpdatedEvent{Session: fs})
	if status == FocusSessionStatusCompleted {
		u, err := user.GetUserByID(s, fs.UserID)
		if err != nil {
			return err
		}
		events.DispatchOnCommit(s, &FocusSessionCompletedEvent{Session: fs, User: u})
	}
	return nil
}

// FocusSessionFilter narrows a focus session listing. Zero values mean no filter.
type FocusSessionFilter struct {
	From      time.Time
	To        time.Time
	TaskID    int64
	ProjectID int64
}

// ListFocusSessions returns the authenticated user's sessions, newest first.
func ListFocusSessions(s *xorm.Session, a web.Auth, filter FocusSessionFilter, page, perPage int) ([]*FocusSession, int64, error) {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return nil, 0, err
	}

	cond := builder.NewCond().And(builder.Eq{"user_id": userID})
	if !filter.From.IsZero() {
		cond = cond.And(builder.Gte{"started_at": filter.From})
	}
	if !filter.To.IsZero() {
		cond = cond.And(builder.Lt{"started_at": filter.To})
	}
	if filter.TaskID > 0 {
		cond = cond.And(builder.Eq{"task_id": filter.TaskID})
	}
	if filter.ProjectID > 0 {
		cond = cond.And(builder.In("task_id",
			builder.Select("id").From("tasks").Where(builder.Eq{"project_id": filter.ProjectID})))
	}

	total, err := s.Where(cond).Count(&FocusSession{})
	if err != nil {
		return nil, 0, err
	}
	sessions := []*FocusSession{}
	err = s.Where(cond).
		OrderBy("started_at DESC, id DESC").
		Limit(getLimitFromPageIndex(page, perPage)).
		Find(&sessions)
	return sessions, total, err
}

// detachFocusSessionsFromTask keeps a deleted task's focus history but drops the link.
func detachFocusSessionsFromTask(s *xorm.Session, taskID int64) error {
	_, err := s.Where("task_id = ?", taskID).Cols("task_id").NoAutoTime().Update(&FocusSession{TaskID: 0})
	return err
}

// deleteFocusDataForUser removes all focus sessions and timer settings of a user.
func deleteFocusDataForUser(s *xorm.Session, userID int64) error {
	if _, err := s.Where("user_id = ?", userID).Delete(&FocusSession{}); err != nil {
		return err
	}
	_, err := s.Where("user_id = ?", userID).Delete(&FocusSettings{})
	return err
}
