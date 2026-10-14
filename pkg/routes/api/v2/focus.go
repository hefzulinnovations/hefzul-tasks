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

package apiv2

import (
	"context"
	"net/http"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"

	"github.com/danielgtaylor/huma/v2"
	"xorm.io/xorm"
)

// The focus (Pomodoro) endpoints are scoped to the authenticated user: a user
// only ever sees and changes their own sessions and settings, so the handlers
// own their sessions and call the model functions directly.

type focusSessionListBody struct {
	Body Paginated[*models.FocusSession]
}

type focusCurrentBody struct {
	Status int
	Body   *models.FocusSession
}

type focusFinishInput struct {
	ID   int64 `path:"id"`
	Body struct {
		Status models.FocusSessionStatus `json:"status" enum:"completed,abandoned" doc:"completed when the session ran its full length, abandoned when it was stopped early."`
	}
}

// RegisterFocusRoutes wires the Pomodoro focus timer endpoints onto the Huma API.
func RegisterFocusRoutes(api huma.API) {
	tags := []string{"focus"}

	Register(api, huma.Operation{
		OperationID: "focus-settings-read",
		Summary:     "Get the focus timer settings",
		Description: "Returns the authenticated user's Pomodoro settings, or the defaults (25/5/15 minutes, long break after 3 sessions) if they never saved any.",
		Method:      http.MethodGet,
		Path:        "/focus/settings",
		Tags:        tags,
	}, focusSettingsRead)

	Register(api, huma.Operation{
		OperationID: "focus-settings-update",
		Summary:     "Update the focus timer settings",
		Description: "PUT replaces all settings; use PATCH to change only some. Durations are in minutes.",
		Method:      http.MethodPut,
		Path:        "/focus/settings",
		Tags:        tags,
	}, focusSettingsUpdate)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-current",
		Summary:     "Get the running focus session",
		Description: "Returns the running or paused session, whichever device started it. Returns 204 with no body when there is none.",
		Method:      http.MethodGet,
		Path:        "/focus/sessions/current",
		Tags:        tags,
	}, focusSessionsCurrent)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-list",
		Summary:     "List focus sessions",
		Description: "Returns your focus and break sessions, newest first, paginated. from and to filter on the start time.",
		Method:      http.MethodGet,
		Path:        "/focus/sessions",
		Tags:        tags,
	}, focusSessionsList)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-start",
		Summary:     "Start a focus session",
		Description: "Starts a focus or break session, optionally linked to a task you can read. planned_seconds defaults to your timer settings for the kind. Returns 409 if another session is running or paused; a running session whose time is already up is completed automatically instead.",
		Method:      http.MethodPost,
		Path:        "/focus/sessions",
		Tags:        tags,
	}, focusSessionsStart)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-pause",
		Summary:     "Pause a focus session",
		Description: "Pauses a running session. Returns 409 if it is not running.",
		Method:      http.MethodPost,
		Path:        "/focus/sessions/{id}/pause",
		// Changes the session's state, creates nothing.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, focusSessionsPause)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-resume",
		Summary:     "Resume a focus session",
		Description: "Resumes a paused session; the pause is excluded from its focus time. Returns 409 if it is not paused.",
		Method:      http.MethodPost,
		Path:        "/focus/sessions/{id}/resume",
		// Changes the session's state, creates nothing.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, focusSessionsResume)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-finish",
		Summary:     "Finish a focus session",
		Description: "Ends a running or paused session as completed or abandoned and stores the time actually spent. A completed session counts at most its planned length. Completing one fires the focus.session.completed webhook event.",
		Method:      http.MethodPost,
		Path:        "/focus/sessions/{id}/finish",
		// Changes the session's state, creates nothing.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, focusSessionsFinish)

	Register(api, huma.Operation{
		OperationID: "focus-stats",
		Summary:     "Get focus statistics",
		Description: "Totals of focus sessions (breaks excluded) per day, task and project for the days from..to, counted in your time zone. Defaults to the last 7 days; at most 366 days per request.",
		Method:      http.MethodGet,
		Path:        "/focus/stats",
		Tags:        tags,
	}, focusStats)
}

func init() { AddRouteRegistrar(RegisterFocusRoutes) }

// inFocusTx runs fn in a transaction and dispatches its events after commit.
func inFocusTx(ctx context.Context, fn func(s *xorm.Session, a web.Auth) error) error {
	a, err := authFromCtx(ctx)
	if err != nil {
		return err
	}
	s := db.NewSession()
	defer s.Close()
	if err := fn(s, a); err != nil {
		_ = s.Rollback()
		events.CleanupPending(s)
		return translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		events.CleanupPending(s)
		return translateDomainError(err)
	}
	events.DispatchPending(ctx, s)
	return nil
}

func focusSettingsRead(ctx context.Context, _ *struct{}) (*singleBody[models.FocusSettings], error) {
	var settings *models.FocusSettings
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		settings, err = models.GetFocusSettings(s, a)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.FocusSettings]{Body: settings}, nil
}

func focusSettingsUpdate(ctx context.Context, in *struct {
	Body models.FocusSettings
}) (*singleBody[models.FocusSettings], error) {
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) error {
		return models.SaveFocusSettings(s, a, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.FocusSettings]{Body: &in.Body}, nil
}

func focusSessionsCurrent(ctx context.Context, _ *struct{}) (*focusCurrentBody, error) {
	var current *models.FocusSession
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		current, err = models.GetCurrentFocusSession(s, a)
		return err
	})
	if err != nil {
		return nil, err
	}
	if current == nil {
		return &focusCurrentBody{Status: http.StatusNoContent}, nil
	}
	return &focusCurrentBody{Status: http.StatusOK, Body: current}, nil
}

func focusSessionsList(ctx context.Context, in *struct {
	ListParams
	From      time.Time `query:"from" doc:"Only sessions that started at or after this time (RFC 3339)."`
	To        time.Time `query:"to" doc:"Only sessions that started before this time (RFC 3339)."`
	TaskID    int64     `query:"task_id" doc:"Only sessions linked to this task."`
	ProjectID int64     `query:"project_id" doc:"Only sessions linked to a task in this project."`
}) (*focusSessionListBody, error) {
	var (
		items []*models.FocusSession
		total int64
	)
	filter := models.FocusSessionFilter{From: in.From, To: in.To, TaskID: in.TaskID, ProjectID: in.ProjectID}
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		items, total, err = models.ListFocusSessions(s, a, filter, in.Page, in.PerPage)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &focusSessionListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

func focusSessionsStart(ctx context.Context, in *struct {
	Body models.FocusSession
}) (*singleBody[models.FocusSession], error) {
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) error {
		return models.StartFocusSession(s, a, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.FocusSession]{Body: &in.Body}, nil
}

func focusSessionAction(ctx context.Context, id int64, action func(*xorm.Session, web.Auth, int64) (*models.FocusSession, error)) (*singleBody[models.FocusSession], error) {
	var fs *models.FocusSession
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		fs, err = action(s, a, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.FocusSession]{Body: fs}, nil
}

func focusSessionsPause(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*singleBody[models.FocusSession], error) {
	return focusSessionAction(ctx, in.ID, models.PauseFocusSession)
}

func focusSessionsResume(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*singleBody[models.FocusSession], error) {
	return focusSessionAction(ctx, in.ID, models.ResumeFocusSession)
}

func focusSessionsFinish(ctx context.Context, in *focusFinishInput) (*singleBody[models.FocusSession], error) {
	return focusSessionAction(ctx, in.ID, func(s *xorm.Session, a web.Auth, id int64) (*models.FocusSession, error) {
		return models.FinishFocusSession(s, a, id, in.Body.Status)
	})
}

func focusStats(ctx context.Context, in *struct {
	From string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"First day, YYYY-MM-DD. Defaults to 6 days before to."`
	To   string `query:"to" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"Last day (inclusive), YYYY-MM-DD. Defaults to today."`
}) (*singleBody[models.FocusStats], error) {
	from, err := parseFocusDate(in.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseFocusDate(in.To, "to")
	if err != nil {
		return nil, err
	}
	var stats *models.FocusStats
	err = inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		stats, err = models.GetFocusStats(s, a, from, to)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.FocusStats]{Body: stats}, nil
}

func parseFocusDate(value, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, huma.Error422UnprocessableEntity(field + " must be a date in the form YYYY-MM-DD")
	}
	return t, nil
}
