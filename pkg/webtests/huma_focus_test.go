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
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/focus_sessions.yml): user1 owns finished sessions
// 1, 2, 3 and 5; session 4 is user2's. Only user2 saved timer settings.

func TestHumaFocus(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	t.Run("settings default and update via PATCH", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/focus/settings", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"focus_minutes":25`)

		rec = humaRequest(t, e, http.MethodPatch, "/api/v2/focus/settings", `{"focus_minutes":45}`, token, "application/merge-patch+json")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"focus_minutes":45`)
		assert.Contains(t, rec.Body.String(), `"short_break_minutes":5`, "PATCH keeps the other settings")

		rec = humaRequest(t, e, http.MethodPut, "/api/v2/focus/settings",
			`{"focus_minutes":0,"short_break_minutes":5,"long_break_minutes":15,"sessions_before_long_break":3,"auto_start_breaks":false,"sound":"bell"}`, token, "")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "out-of-range minutes are rejected by the schema")
	})

	t.Run("current is 204 when nothing runs", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/focus/sessions/current", "", token, "")
		assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		assert.Empty(t, rec.Body.String())
	})

	var sessionID int64
	t.Run("start, conflict, pause, resume, finish", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/focus/sessions", `{"task_id":1,"device":"web"}`, token, "")
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var started struct {
			ID             int64  `json:"id"`
			Status         string `json:"status"`
			PlannedSeconds int64  `json:"planned_seconds"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &started))
		sessionID = started.ID
		assert.Equal(t, "running", started.Status)
		assert.Equal(t, int64(45*60), started.PlannedSeconds, "planned length follows the saved settings")

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/focus/sessions/current", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"id":%d`, sessionID))

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/focus/sessions", `{}`, token, "")
		assert.Equal(t, http.StatusConflict, rec.Code, "only one running session per user")

		path := fmt.Sprintf("/api/v2/focus/sessions/%d", sessionID)
		rec = humaRequest(t, e, http.MethodPost, path+"/pause", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"status":"paused"`)

		rec = humaRequest(t, e, http.MethodPost, path+"/pause", "", token, "")
		assert.Equal(t, http.StatusConflict, rec.Code)

		rec = humaRequest(t, e, http.MethodPost, path+"/resume", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"status":"running"`)

		rec = humaRequest(t, e, http.MethodPost, path+"/finish", `{"status":"running"}`, token, "")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "finish only accepts completed or abandoned")

		rec = humaRequest(t, e, http.MethodPost, path+"/finish", `{"status":"abandoned"}`, token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"status":"abandoned"`)
	})

	t.Run("another user's session is 404", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/focus/sessions/4/pause", "", token, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("starting on a task you cannot read is 403", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/focus/sessions", `{"task_id":1}`, humaTokenFor(t, &testuser2), "")
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("list is the user's own sessions", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/focus/sessions?task_id=1", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var page struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
			Total int64 `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
		assert.Equal(t, int64(2), page.Total, "fixture session 1 and the one started above")
	})

	t.Run("stats", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/focus/stats?from=2026-09-30&to=2026-10-04", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"total_focus_seconds":3600`)
		assert.Contains(t, rec.Body.String(), `"title":"task #1"`)

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/focus/stats?from=yesterday", "", token, "")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("unauthenticated is 401", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/focus/settings", "", "", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
