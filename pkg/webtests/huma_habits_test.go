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

package webtests

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures: habits 1–4 and 6 (archived) belong to user1, habit 5 to user2;
// countdowns 1–3 belong to user1, countdown 4 to user2.

func TestHumaHabits(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	t.Run("list skips archived", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/habits", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var page struct {
			Total int64 `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
		assert.Equal(t, int64(4), page.Total)
		assert.Contains(t, rec.Body.String(), `"current_streak"`)

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/habits?include_archived=true", "", token, "")
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
		assert.Equal(t, int64(5), page.Total)
	})

	t.Run("create, patch, check in, undo, stats, delete", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/habits", `{"title":"Stretch","daily_target":2,"reminder_time":"07:00"}`, token, "")
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var created struct {
			ID int64 `json:"id"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		base := "/api/v2/habits/" + strconv.FormatInt(created.ID, 10)

		rec = humaRequest(t, e, http.MethodPatch, base, `{"title":"Stretch daily"}`, token, "application/merge-patch+json")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"title":"Stretch daily"`)
		assert.Contains(t, rec.Body.String(), `"daily_target":2`, "PATCH keeps the other fields")

		rec = humaRequest(t, e, http.MethodPost, base+"/checkins", `{}`, token, "")
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"count":1`)
		var checkin struct {
			Date string `json:"date"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &checkin))

		rec = humaRequest(t, e, http.MethodGet, base, "", token, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"today_count":1`)
		assert.Contains(t, rec.Body.String(), `"done_today":false`)

		rec = humaRequest(t, e, http.MethodDelete, base+"/checkins/"+checkin.Date, "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"count":0`)

		rec = humaRequest(t, e, http.MethodDelete, base+"/checkins/"+checkin.Date, "", token, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)

		rec = humaRequest(t, e, http.MethodGet, base+"/stats", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"streak_unit":"days"`)

		rec = humaRequest(t, e, http.MethodDelete, base, "", token, "")
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("invalid schedule is 400", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/habits", `{"title":"x","schedule_kind":"weekdays","weekdays":0}`, token, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("another user's habit is 404", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/habits/5", "", token, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		rec = humaRequest(t, e, http.MethodPost, "/api/v2/habits/5/checkins", `{}`, token, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHumaCountdowns(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	rec := humaRequest(t, e, http.MethodGet, "/api/v2/countdowns", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"days_remaining"`)
	assert.NotContains(t, rec.Body.String(), "Anniversary", "another user's countdown")

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/countdowns", `{"title":"Holiday","target_date":"2099-12-24","pinned":true}`, token, "")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"next_date":"2099-12-24"`)

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/countdowns", `{"title":"Bad","target_date":"24.12.2099"}`, token, "")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "the schema rejects a malformed date")

	rec = humaRequest(t, e, http.MethodGet, "/api/v2/countdowns/4", "", token, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = humaRequest(t, e, http.MethodPatch, "/api/v2/countdowns/1", `{"pinned":true}`, token, "application/merge-patch+json")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"pinned":true`)
	assert.Contains(t, rec.Body.String(), `"repeat_yearly":true`)

	rec = humaRequest(t, e, http.MethodDelete, "/api/v2/countdowns/3", "", token, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
