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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHumaPush(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser2)

	rec := humaRequest(t, e, http.MethodGet, "/api/v2/push/public-key", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"public_key":"`)

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/push/test", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"delivered":0`, "no devices yet")

	sub := `{"endpoint":"https://push.example.com/send/user2","p256dh":"key","auth":"secret","include_content":true,"device":"Firefox"}`
	rec = humaRequest(t, e, http.MethodPost, "/api/v2/push/subscriptions", sub, token, "")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "user_id")

	rec = humaRequest(t, e, http.MethodGet, "/api/v2/push/subscriptions", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "https://push.example.com/send/user2")
	assert.NotContains(t, rec.Body.String(), "user1-device", "only your own devices")

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/push/subscriptions", `{"endpoint":"http://plain.example.com","p256dh":"k","auth":"a"}`, token, "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/push/subscriptions/unsubscribe", `{"endpoint":"https://push.example.com/send/user2"}`, token, "")
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = humaRequest(t, e, http.MethodPost, "/api/v2/push/subscriptions/unsubscribe", `{"endpoint":"https://push.example.com/send/user2"}`, token, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
