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
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

type countdownListBody struct {
	Body Paginated[*models.Countdown]
}

type countdownReadBody struct {
	models.Countdown
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The requesting user's permission on this countdown; always 2 (admin) since countdowns are private."`
}

// RegisterCountdownRoutes wires the countdown endpoints onto the Huma API.
func RegisterCountdownRoutes(api huma.API) {
	tags := []string{"countdowns"}

	Register(api, huma.Operation{
		OperationID: "countdowns-list",
		Summary:     "List countdowns",
		Description: "Returns your countdowns, soonest first, with days remaining in your time zone; passed one-off dates come last. within_days limits the list to pinned countdowns and those due within that many days, for a dashboard card.",
		Method:      http.MethodGet, Path: "/countdowns", Tags: tags,
	}, countdownsList)
	Register(api, huma.Operation{
		OperationID: "countdowns-read",
		Summary:     "Get a countdown",
		Method:      http.MethodGet, Path: "/countdowns/{id}", Tags: tags,
	}, countdownsRead)
	Register(api, huma.Operation{
		OperationID: "countdowns-create",
		Summary:     "Create a countdown",
		Method:      http.MethodPost, Path: "/countdowns", Tags: tags,
	}, countdownsCreate)
	Register(api, huma.Operation{
		OperationID: "countdowns-update",
		Summary:     "Update a countdown",
		Description: "PUT replaces all editable fields; use PATCH for a partial update.",
		Method:      http.MethodPut, Path: "/countdowns/{id}", Tags: tags,
	}, countdownsUpdate)
	Register(api, huma.Operation{
		OperationID: "countdowns-delete",
		Summary:     "Delete a countdown",
		Method:      http.MethodDelete, Path: "/countdowns/{id}", Tags: tags,
	}, countdownsDelete)
}

func init() { AddRouteRegistrar(RegisterCountdownRoutes) }

func countdownsList(ctx context.Context, in *struct {
	ListParams
	WithinDays int `query:"within_days" minimum:"0" maximum:"366" doc:"Only pinned countdowns and those due within this many days. 0 returns all."`
}) (*countdownListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.Countdown{WithinDays: in.WithinDays}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.Countdown)
	if !ok {
		return nil, fmt.Errorf("countdowns.ReadAll returned unexpected type %T", result)
	}
	return &countdownListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

func countdownsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[countdownReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c := &models.Countdown{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, c, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	// days_remaining changes daily, not only with updated; never answer 304.
	return &singleReadBody[countdownReadBody]{Body: &countdownReadBody{Countdown: *c, MaxPermission: models.Permission(maxPermission)}}, nil
}

func countdownsCreate(ctx context.Context, in *struct {
	Body models.Countdown
}) (*singleBody[models.Countdown], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Countdown]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func countdownsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body countdownReadBody
}) (*singleBody[models.Countdown], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c := &in.Body.Countdown
	c.ID = in.ID
	if err := handler.DoUpdate(ctx, c, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Countdown]{Body: c}, nil
}

func countdownsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.Countdown{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
