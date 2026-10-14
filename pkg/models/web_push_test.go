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
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixture push_subscriptions.yml: user1 has one subscription (id 1), without
// content opt-in.

type capturedPush struct {
	endpoint string
	message  PushMessage
}

func stubPushSender(t *testing.T, status int) *[]capturedPush {
	t.Helper()
	sent := &[]capturedPush{}
	original := pushSender
	pushSender = func(payload []byte, sub *PushSubscription, _ *WebPushKeys) (int, error) {
		var m PushMessage
		require.NoError(t, json.Unmarshal(payload, &m))
		*sent = append(*sent, capturedPush{endpoint: sub.Endpoint, message: m})
		return status, nil
	}
	t.Cleanup(func() { pushSender = original })
	return sent
}

func TestGetVAPIDKeys(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	first, err := GetVAPIDKeys(s)
	require.NoError(t, err)
	assert.NotEmpty(t, first.PublicKey)
	assert.NotEmpty(t, first.PrivateKey)
	second, err := GetVAPIDKeys(s)
	require.NoError(t, err)
	assert.Equal(t, first.PublicKey, second.PublicKey, "keys are created once and reused")
}

func TestPushSubscriptions(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	err := SavePushSubscription(s, u, &PushSubscription{Endpoint: "http://insecure.example.com", P256dh: "x", Auth: "y"})
	assert.True(t, IsErrInvalidPushSubscription(err))

	sub := &PushSubscription{Endpoint: "https://push.example.com/send/new", P256dh: "key", Auth: "secret", IncludeContent: true}
	require.NoError(t, SavePushSubscription(s, u, sub))
	again := &PushSubscription{Endpoint: "https://push.example.com/send/new", P256dh: "key", Auth: "secret"}
	require.NoError(t, SavePushSubscription(s, &user.User{ID: 2}, again))

	subs, err := ListPushSubscriptions(s, u)
	require.NoError(t, err)
	assert.Len(t, subs, 1, "re-subscribing the endpoint for another user moves it")

	err = DeletePushSubscription(s, u, "https://push.example.com/send/new")
	assert.True(t, IsErrPushSubscriptionDoesNotExist(err), "it now belongs to user 2")
	require.NoError(t, DeletePushSubscription(s, &user.User{ID: 2}, "https://push.example.com/send/new"))

	_, err = ListPushSubscriptions(s, &LinkSharing{ID: 1})
	assert.True(t, IsErrGenericForbidden(err))
}

func TestSendPush_PrivacyAndCleanup(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	sent := stubPushSender(t, http.StatusCreated)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, SavePushSubscription(s, &user.User{ID: 1}, &PushSubscription{
		Endpoint: "https://push.example.com/send/with-content", P256dh: "k", Auth: "a", IncludeContent: true,
	}))

	row := &notificationRow{ID: 7, NotifiableID: 1, Name: "task.reminder",
		Notification: json.RawMessage(`{"task":{"id":42,"title":"Pay electricity bill"}}`)}
	n, err := sendPushToUser(s, 1, notificationPushContent(row))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	require.Len(t, *sent, 2)
	for _, p := range *sent {
		assert.Equal(t, "/tasks/42", p.message.URL)
		if p.endpoint == "https://push.example.com/send/with-content" {
			assert.Equal(t, "Pay electricity bill", p.message.Body)
		} else {
			assert.NotContains(t, p.message.Body, "electricity", "no titles without the opt-in")
			assert.Equal(t, "Hefzul Tasks", p.message.Title)
		}
	}

	gone := stubPushSender(t, http.StatusGone)
	_, err = sendPushToUser(s, 1, &pushContent{genericBody: "x"})
	require.NoError(t, err)
	assert.Len(t, *gone, 2)
	subs, err := ListPushSubscriptions(s, &user.User{ID: 1})
	require.NoError(t, err)
	assert.Empty(t, subs, "a 410 Gone removes the subscription")
}

func TestSendHabitReminders(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	sent := stubPushSender(t, http.StatusCreated)
	s := db.NewSession()
	defer s.Close()

	// Habit 1 (user1) reminds at 21:00 in the user's time zone.
	loc, err := focusUserLocation(s, 1)
	require.NoError(t, err)
	local := time.Now().In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(), 21, 0, 30, 0, loc)

	require.NoError(t, sendHabitReminders(s, at.Add(-time.Minute)))
	assert.Empty(t, *sent, "not before the reminder time")

	require.NoError(t, sendHabitReminders(s, at))
	require.Len(t, *sent, 1)
	assert.Equal(t, "habit.reminder", (*sent)[0].message.Kind)
	assert.NotContains(t, (*sent)[0].message.Body, "Read 20 pages")

	require.NoError(t, sendHabitReminders(s, at))
	assert.Len(t, *sent, 1, "each reminder is sent once a day")
}

func TestSendFocusEndPushes(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	sent := stubPushSender(t, http.StatusCreated)
	s := db.NewSession()
	defer s.Close()

	started := time.Now().Add(-26 * time.Minute)
	fs := &FocusSession{UserID: 1, Kind: FocusSessionKindFocus, PlannedSeconds: 25 * 60, Status: FocusSessionStatusRunning, StartedAt: started}
	_, err := s.Insert(fs)
	require.NoError(t, err)

	require.NoError(t, sendFocusEndPushes(s, started.Add(25*time.Minute+5*time.Second)))
	assert.Empty(t, *sent, "an open app gets a few seconds to finish the session itself")

	require.NoError(t, sendFocusEndPushes(s, started.Add(26*time.Minute)))
	require.Len(t, *sent, 1)
	assert.Equal(t, "focus.ended", (*sent)[0].message.Kind)

	require.NoError(t, sendFocusEndPushes(s, started.Add(27*time.Minute)))
	assert.Len(t, *sent, 1, "sent once per session")
}
