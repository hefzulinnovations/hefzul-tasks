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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/web"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/ThreeDotsLabs/watermill/message"
	"xorm.io/xorm"
)

// PushSubscription is one browser or app installation that receives Web Push
// notifications for a user.
type PushSubscription struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The subscription's id."`
	UserID int64 `xorm:"bigint not null INDEX" json:"-"`

	Endpoint     string `xorm:"text not null" json:"endpoint" minLength:"1" maxLength:"2048" doc:"The push service URL from PushSubscription.endpoint."`
	EndpointHash string `xorm:"varchar(64) not null unique" json:"-"`
	P256dh       string `xorm:"varchar(255) not null" json:"p256dh" minLength:"1" maxLength:"255" doc:"The p256dh key from PushSubscription.getKey('p256dh'), base64url."`
	Auth         string `xorm:"varchar(255) not null" json:"auth" minLength:"1" maxLength:"255" doc:"The auth secret from PushSubscription.getKey('auth'), base64url."`

	IncludeContent bool   `xorm:"bool not null default false" json:"include_content" doc:"Show task and habit titles in notifications. Off by default: payloads then carry only ids and a generic text, so the push service never sees your content."`
	Device         string `xorm:"varchar(250) null" json:"device" maxLength:"250" doc:"Free-form device name, such as Firefox on Linux."`

	Created  time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When this device subscribed."`
	LastUsed time.Time `xorm:"datetime null" json:"last_used" readOnly:"true" doc:"When a push was last delivered to it."`
}

// TableName is push_subscriptions.
func (*PushSubscription) TableName() string {
	return "push_subscriptions"
}

// WebPushKeys holds the server's VAPID key pair, created on first start.
type WebPushKeys struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	PublicKey  string    `xorm:"varchar(255) not null"`
	PrivateKey string    `xorm:"varchar(255) not null"`
	Created    time.Time `xorm:"created not null"`
}

// TableName is web_push_keys.
func (*WebPushKeys) TableName() string {
	return "web_push_keys"
}

var vapidMu sync.Mutex

// GetVAPIDKeys returns the server's VAPID key pair, creating it if needed.
func GetVAPIDKeys(s *xorm.Session) (*WebPushKeys, error) {
	vapidMu.Lock()
	defer vapidMu.Unlock()

	keys := &WebPushKeys{}
	exists, err := s.OrderBy("id ASC").Get(keys)
	if err != nil {
		return nil, err
	}
	if exists {
		return keys, nil
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return nil, err
	}
	keys = &WebPushKeys{PublicKey: public, PrivateKey: private}
	if _, err := s.Insert(keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func endpointHash(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(sum[:])
}

func pushOwner(a web.Auth) (int64, error) {
	if _, isShare := a.(*LinkSharing); isShare {
		return 0, ErrGenericForbidden{}
	}
	return a.GetID(), nil
}

// SavePushSubscription stores the subscription for the authenticated user. The
// same endpoint subscribing again (or for another user) replaces the old row.
func SavePushSubscription(s *xorm.Session, a web.Auth, sub *PushSubscription) error {
	userID, err := pushOwner(a)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(sub.Endpoint, "https://") {
		return ErrInvalidPushSubscription{Reason: "endpoint must be an https URL"}
	}
	if sub.P256dh == "" || sub.Auth == "" {
		return ErrInvalidPushSubscription{Reason: "p256dh and auth are required"}
	}
	sub.UserID = userID
	sub.EndpointHash = endpointHash(sub.Endpoint)
	if _, err := s.Where("endpoint_hash = ?", sub.EndpointHash).Delete(&PushSubscription{}); err != nil {
		return err
	}
	sub.ID = 0
	_, err = s.Insert(sub)
	return err
}

// ListPushSubscriptions returns the authenticated user's subscriptions.
func ListPushSubscriptions(s *xorm.Session, a web.Auth) ([]*PushSubscription, error) {
	userID, err := pushOwner(a)
	if err != nil {
		return nil, err
	}
	subs := []*PushSubscription{}
	err = s.Where("user_id = ?", userID).OrderBy("id ASC").Find(&subs)
	return subs, err
}

// DeletePushSubscription removes one of the user's subscriptions by endpoint.
func DeletePushSubscription(s *xorm.Session, a web.Auth, endpoint string) error {
	userID, err := pushOwner(a)
	if err != nil {
		return err
	}
	n, err := s.Where("user_id = ? AND endpoint_hash = ?", userID, endpointHash(endpoint)).Delete(&PushSubscription{})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPushSubscriptionDoesNotExist{}
	}
	return nil
}

// PushMessage is the JSON the service worker receives. Without content
// opt-in it carries no titles, only ids and a generic text.
type PushMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
	Kind  string `json:"kind"`
	ID    int64  `json:"id,omitempty"`
}

// pushContent is what one event says, in a private and a content version.
type pushContent struct {
	kind        string
	id          int64
	url         string
	tag         string
	genericBody string
	title       string
	detail      string
}

func (c *pushContent) message(includeContent bool) *PushMessage {
	m := &PushMessage{Title: "Hefzul Tasks", Body: c.genericBody, URL: c.url, Tag: c.tag, Kind: c.kind, ID: c.id}
	if includeContent && c.detail != "" {
		m.Title = c.title
		m.Body = c.detail
	}
	return m
}

// pushSender is replaced in tests.
var pushSender = func(payload []byte, sub *PushSubscription, keys *WebPushKeys) (int, error) {
	subscriber := config.MailerFromEmail.GetString()
	if subscriber == "" {
		subscriber = config.ServicePublicURL.GetString()
	}
	if subscriber == "" {
		subscriber = "hefzul-tasks@localhost"
	}
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber:      subscriber,
		VAPIDPublicKey:  keys.PublicKey,
		VAPIDPrivateKey: keys.PrivateKey,
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
		Topic:           "",
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// sendPushToUser delivers the content to every subscription of the user. Gone
// subscriptions (404/410) are removed. Returns how many deliveries succeeded.
func sendPushToUser(s *xorm.Session, userID int64, content *pushContent) (int, error) {
	subs := []*PushSubscription{}
	if err := s.Where("user_id = ?", userID).Find(&subs); err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	keys, err := GetVAPIDKeys(s)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, sub := range subs {
		payload, err := json.Marshal(content.message(sub.IncludeContent))
		if err != nil {
			return sent, err
		}
		status, err := pushSender(payload, sub, keys)
		if err != nil {
			log.Warningf("Web Push to subscription %d failed: %s", sub.ID, err)
			continue
		}
		switch {
		case status == http.StatusNotFound || status == http.StatusGone:
			if _, err := s.ID(sub.ID).Delete(&PushSubscription{}); err != nil {
				return sent, err
			}
		case status >= 200 && status < 300:
			sent++
			sub.LastUsed = time.Now()
			if _, err := s.ID(sub.ID).Cols("last_used").Update(sub); err != nil {
				return sent, err
			}
		default:
			log.Warningf("Web Push to subscription %d returned status %d", sub.ID, status)
		}
	}
	return sent, nil
}

// SendTestPush sends a test notification to the authenticated user's devices.
func SendTestPush(s *xorm.Session, a web.Auth) (int, error) {
	userID, err := pushOwner(a)
	if err != nil {
		return 0, err
	}
	return sendPushToUser(s, userID, &pushContent{
		kind: "test", url: "/", tag: "test",
		genericBody: "Push notifications work on this device.",
	})
}

// notificationPushContent turns a stored notification into push content.
func notificationPushContent(n *notificationRow) *pushContent {
	var data map[string]any
	_ = json.Unmarshal(n.Notification, &data)

	c := &pushContent{kind: n.Name, id: n.ID, url: "/notifications", tag: fmt.Sprintf("notification-%d", n.ID)}
	if task, ok := data["task"].(map[string]any); ok {
		if id, ok := task["id"].(float64); ok && id > 0 {
			c.url = fmt.Sprintf("/tasks/%d", int64(id))
			c.tag = fmt.Sprintf("task-%d", int64(id))
		}
		if title, ok := task["title"].(string); ok {
			c.detail = title
		}
	}

	switch n.Name {
	case "task.reminder":
		c.genericBody, c.title = "You have a task reminder.", "Reminder"
	case "task.undone.overdue":
		c.genericBody, c.title = "A task is overdue.", "Overdue"
	case "task.comment":
		c.genericBody, c.title = "There is a new comment on a task.", "New comment"
	case "task.assigned":
		c.genericBody, c.title = "You were assigned a task.", "Assigned to you"
	case "task.mentioned":
		c.genericBody, c.title = "You were mentioned in a task.", "Mentioned"
	case "task.deleted":
		c.genericBody, c.title = "A task was deleted.", "Task deleted"
	default:
		c.genericBody, c.title = "You have a new notification.", "Notification"
	}
	return c
}

// notificationRow mirrors the columns of the notifications table the push needs.
type notificationRow struct {
	ID           int64           `xorm:"id"`
	NotifiableID int64           `xorm:"notifiable_id"`
	Notification json.RawMessage `xorm:"notification"`
	Name         string          `xorm:"name"`
}

// PushNotificationListener sends a Web Push for every new notification.
type PushNotificationListener struct{}

// Name defines the name for the PushNotificationListener listener
func (l *PushNotificationListener) Name() string { return "webpush.notification" }

// Handle is executed when a notification was stored.
func (l *PushNotificationListener) Handle(msg *message.Message) error {
	var event struct {
		NotificationID int64 `json:"notification_id"`
		UserID         int64 `json:"user_id"`
	}
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return err
	}
	s := db.NewSession()
	defer s.Close()

	row := &notificationRow{}
	exists, err := s.Table("notifications").Where("id = ?", event.NotificationID).Get(row)
	if err != nil || !exists {
		return err
	}
	if _, err := sendPushToUser(s, row.NotifiableID, notificationPushContent(row)); err != nil {
		_ = s.Rollback()
		return err
	}
	return s.Commit()
}

// deletePushSubscriptionsForUser removes a user's push subscriptions.
func deletePushSubscriptionsForUser(s *xorm.Session, userID int64) error {
	_, err := s.Where("user_id = ?", userID).Delete(&PushSubscription{})
	return err
}
