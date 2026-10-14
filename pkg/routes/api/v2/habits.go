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
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
	"xorm.io/xorm"
)

type habitListBody struct {
	Body Paginated[*models.Habit]
}

type habitReadBody struct {
	models.Habit
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The requesting user's permission on this habit; always 2 (admin) since habits are private."`
}

// RegisterHabitRoutes wires the habit tracker endpoints onto the Huma API.
func RegisterHabitRoutes(api huma.API) {
	tags := []string{"habits"}

	Register(api, huma.Operation{
		OperationID: "habits-list",
		Summary:     "List habits",
		Description: "Returns your habits by position with today's progress and the current streak, computed in your time zone. Archived habits are left out unless include_archived is true.",
		Method:      http.MethodGet, Path: "/habits", Tags: tags,
	}, habitsList)
	Register(api, huma.Operation{
		OperationID: "habits-read",
		Summary:     "Get a habit",
		Description: "Returns one of your habits with today's progress. Other users' habits read as 404.",
		Method:      http.MethodGet, Path: "/habits/{id}", Tags: tags,
	}, habitsRead)
	Register(api, huma.Operation{
		OperationID: "habits-create",
		Summary:     "Create a habit",
		Method:      http.MethodPost, Path: "/habits", Tags: tags,
	}, habitsCreate)
	Register(api, huma.Operation{
		OperationID: "habits-update",
		Summary:     "Update a habit",
		Description: "PUT replaces all editable fields; use PATCH for a partial update. Check-ins are kept when the schedule changes.",
		Method:      http.MethodPut, Path: "/habits/{id}", Tags: tags,
	}, habitsUpdate)
	Register(api, huma.Operation{
		OperationID: "habits-delete",
		Summary:     "Delete a habit",
		Description: "Deletes the habit and all its check-ins. Archive it instead to keep the history.",
		Method:      http.MethodDelete, Path: "/habits/{id}", Tags: tags,
	}, habitsDelete)
	Register(api, huma.Operation{
		OperationID: "habits-checkin",
		Summary:     "Check in on a habit",
		Description: "Adds one check-in for the date (default today in your time zone) and returns that day's total. Fires the habit.checkin.created webhook event.",
		Method:      http.MethodPost, Path: "/habits/{id}/checkins", Tags: tags,
	}, habitsCheckin)
	Register(api, huma.Operation{
		OperationID: "habits-checkin-undo",
		Summary:     "Undo a check-in",
		Description: "Removes the last check-in of that date and returns the day's new total (0 when none are left). 404 when the date has no check-ins.",
		Method:      http.MethodDelete, Path: "/habits/{id}/checkins/{date}",
		// Returns the day's remaining count, not the wrapper's DELETE default 204.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, habitsCheckinUndo)
	Register(api, huma.Operation{
		OperationID: "habits-stats",
		Summary:     "Get habit statistics",
		Description: "Current and best streak, every day from..to for a calendar heatmap (default the last 12 months, at most 400 days) and completion per month.",
		Method:      http.MethodGet, Path: "/habits/{id}/stats", Tags: tags,
	}, habitsStats)
}

func init() { AddRouteRegistrar(RegisterHabitRoutes) }

func habitsList(ctx context.Context, in *struct {
	ListParams
	IncludeArchived bool `query:"include_archived" doc:"Also return archived habits."`
}) (*habitListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.Habit{IncludeArchived: in.IncludeArchived}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.Habit)
	if !ok {
		return nil, fmt.Errorf("habits.ReadAll returned unexpected type %T", result)
	}
	return &habitListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

func habitsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[habitReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	h := &models.Habit{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, h, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &habitReadBody{Habit: *h, MaxPermission: models.Permission(maxPermission)}
	// Progress changes with check-ins, not only with updated; never answer 304.
	return &singleReadBody[habitReadBody]{Body: body}, nil
}

func habitsCreate(ctx context.Context, in *struct {
	Body models.Habit
}) (*singleBody[models.Habit], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Habit]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func habitsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body habitReadBody
}) (*singleBody[models.Habit], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	h := &in.Body.Habit
	h.ID = in.ID
	if err := handler.DoUpdate(ctx, h, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Habit]{Body: h}, nil
}

func habitsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.Habit{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

func habitsCheckin(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body struct {
		Date string `json:"date,omitempty" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"Local date, YYYY-MM-DD. Empty for today."`
	}
}) (*singleBody[models.HabitCheckin], error) {
	var checkin *models.HabitCheckin
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		checkin, err = models.CheckInHabit(s, a, in.ID, in.Body.Date)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.HabitCheckin]{Body: checkin}, nil
}

func habitsCheckinUndo(ctx context.Context, in *struct {
	ID   int64  `path:"id"`
	Date string `path:"date" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"Local date, YYYY-MM-DD."`
}) (*singleBody[models.HabitCheckin], error) {
	var checkin *models.HabitCheckin
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		checkin, err = models.UndoHabitCheckin(s, a, in.ID, in.Date)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.HabitCheckin]{Body: checkin}, nil
}

func habitsStats(ctx context.Context, in *struct {
	ID   int64  `path:"id"`
	From string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"First day, YYYY-MM-DD. Defaults to a year before to."`
	To   string `query:"to" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"Last day (inclusive), YYYY-MM-DD. Defaults to today."`
}) (*singleBody[models.HabitStats], error) {
	from, err := parseFocusDate(in.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseFocusDate(in.To, "to")
	if err != nil {
		return nil, err
	}
	var stats *models.HabitStats
	err = inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		stats, err = models.GetHabitStats(s, a, in.ID, from, to)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.HabitStats]{Body: stats}, nil
}
