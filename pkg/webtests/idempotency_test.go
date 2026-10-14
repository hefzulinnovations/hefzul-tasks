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
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func idempotentRequest(t *testing.T, e *echo.Echo, method, path, body, token, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestIdempotencyKey(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	t.Run("a replayed create is applied once", func(t *testing.T) {
		first := idempotentRequest(t, e, http.MethodPost, "/api/v2/countdowns", `{"title":"Once","target_date":"2099-01-01"}`, token, "outbox-1")
		require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
		second := idempotentRequest(t, e, http.MethodPost, "/api/v2/countdowns", `{"title":"Once","target_date":"2099-01-01"}`, token, "outbox-1")
		require.Equal(t, http.StatusCreated, second.Code)
		assert.Equal(t, "true", second.Header().Get("Idempotent-Replayed"))

		var a, b struct {
			ID int64 `json:"id"`
		}
		require.NoError(t, json.Unmarshal(first.Body.Bytes(), &a))
		require.NoError(t, json.Unmarshal(second.Body.Bytes(), &b))
		assert.Equal(t, a.ID, b.ID, "the replay returns the first result")

		rec := humaRequest(t, e, http.MethodGet, "/api/v2/countdowns?q=Once", "", token, "")
		assert.Equal(t, 1, strings.Count(rec.Body.String(), `"title":"Once"`), "only one countdown was created")
	})

	t.Run("a key reused for another request is rejected", func(t *testing.T) {
		rec := idempotentRequest(t, e, http.MethodPost, "/api/v2/habits", `{"title":"Other"}`, token, "outbox-1")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("keys are per user", func(t *testing.T) {
		rec := idempotentRequest(t, e, http.MethodPost, "/api/v2/countdowns", `{"title":"Theirs","target_date":"2099-01-01"}`, humaTokenFor(t, &testuser2), "outbox-1")
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Empty(t, rec.Header().Get("Idempotent-Replayed"))
	})

	t.Run("v1 writes are covered too", func(t *testing.T) {
		body := `{"title":"Offline task"}`
		first := idempotentRequest(t, e, http.MethodPut, "/api/v1/projects/1/tasks", body, token, "outbox-v1")
		require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
		second := idempotentRequest(t, e, http.MethodPut, "/api/v1/projects/1/tasks", body, token, "outbox-v1")
		assert.Equal(t, "true", second.Header().Get("Idempotent-Replayed"))
		assert.JSONEq(t, first.Body.String(), second.Body.String())
	})

	t.Run("a 4xx answer is final and replayed", func(t *testing.T) {
		rec := idempotentRequest(t, e, http.MethodPost, "/api/v2/habits/999/checkins", `{}`, token, "outbox-missing")
		require.Equal(t, http.StatusNotFound, rec.Code)
		rec = idempotentRequest(t, e, http.MethodPost, "/api/v2/habits/999/checkins", `{}`, token, "outbox-missing")
		assert.Equal(t, "true", rec.Header().Get("Idempotent-Replayed"), "a 4xx answer is final and replayed")
	})
}
