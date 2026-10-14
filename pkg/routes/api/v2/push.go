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

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"

	"github.com/danielgtaylor/huma/v2"
	"xorm.io/xorm"
)

type pushPublicKeyBody struct {
	Body struct {
		PublicKey string `json:"public_key" doc:"The server's VAPID public key, base64url; pass it as applicationServerKey to PushManager.subscribe()."`
	}
}

type pushSubscriptionListBody struct {
	Body struct {
		Items []*models.PushSubscription `json:"items" doc:"Your subscribed devices."`
	}
}

type pushTestBody struct {
	Body struct {
		Delivered int `json:"delivered" doc:"Devices the push service accepted the test notification for."`
	}
}

// RegisterPushRoutes wires the Web Push subscription endpoints onto the Huma API.
func RegisterPushRoutes(api huma.API) {
	tags := []string{"push"}

	Register(api, huma.Operation{
		OperationID: "push-public-key",
		Summary:     "Get the Web Push public key",
		Description: "The VAPID public key a browser needs to subscribe to push notifications from this server. The key pair is created on first start.",
		Method:      http.MethodGet, Path: "/push/public-key", Tags: tags,
	}, pushPublicKey)
	Register(api, huma.Operation{
		OperationID: "push-subscriptions-list",
		Summary:     "List your push devices",
		Method:      http.MethodGet, Path: "/push/subscriptions", Tags: tags,
	}, pushSubscriptionsList)
	Register(api, huma.Operation{
		OperationID: "push-subscriptions-create",
		Summary:     "Subscribe a device to push notifications",
		Description: "Stores the browser's PushSubscription for you. Subscribing the same endpoint again replaces the old entry, so this also changes include_content. Notifications carry only ids and a generic text unless include_content is true.",
		Method:      http.MethodPost, Path: "/push/subscriptions", Tags: tags,
	}, pushSubscriptionsCreate)
	Register(api, huma.Operation{
		OperationID: "push-subscriptions-delete",
		Summary:     "Unsubscribe a device",
		Description: "Removes your subscription with this endpoint. 404 when this device is not subscribed.",
		Method:      http.MethodPost, Path: "/push/subscriptions/unsubscribe",
		// Removes a subscription identified by its endpoint in the body; nothing is created.
		DefaultStatus: http.StatusNoContent,
		Tags:          tags,
	}, pushSubscriptionsDelete)
	Register(api, huma.Operation{
		OperationID: "push-test",
		Summary:     "Send a test push",
		Description: "Sends a test notification to all your subscribed devices and returns how many the push services accepted.",
		Method:      http.MethodPost, Path: "/push/test",
		// Sends a notification, creates nothing.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, pushTest)
}

func init() { AddRouteRegistrar(RegisterPushRoutes) }

func pushPublicKey(ctx context.Context, _ *struct{}) (*pushPublicKeyBody, error) {
	out := &pushPublicKeyBody{}
	err := inFocusTx(ctx, func(s *xorm.Session, _ web.Auth) error {
		keys, err := models.GetVAPIDKeys(s)
		if err != nil {
			return err
		}
		out.Body.PublicKey = keys.PublicKey
		return nil
	})
	return out, err
}

func pushSubscriptionsList(ctx context.Context, _ *struct{}) (*pushSubscriptionListBody, error) {
	out := &pushSubscriptionListBody{}
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		out.Body.Items, err = models.ListPushSubscriptions(s, a)
		return err
	})
	return out, err
}

func pushSubscriptionsCreate(ctx context.Context, in *struct {
	Body models.PushSubscription
}) (*singleBody[models.PushSubscription], error) {
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) error {
		return models.SavePushSubscription(s, a, &in.Body)
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[models.PushSubscription]{Body: &in.Body}, nil
}

func pushSubscriptionsDelete(ctx context.Context, in *struct {
	Body struct {
		Endpoint string `json:"endpoint" minLength:"1" maxLength:"2048" doc:"The endpoint of the subscription to remove."`
	}
}) (*emptyBody, error) {
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) error {
		return models.DeletePushSubscription(s, a, in.Body.Endpoint)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}

func pushTest(ctx context.Context, _ *struct{}) (*pushTestBody, error) {
	out := &pushTestBody{}
	err := inFocusTx(ctx, func(s *xorm.Session, a web.Auth) (err error) {
		out.Body.Delivered, err = models.SendTestPush(s, a)
		return err
	})
	return out, err
}
