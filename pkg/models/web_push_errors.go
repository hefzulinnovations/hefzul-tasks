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
	"errors"
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/web"
)

// ErrInvalidPushSubscription represents an error where a push subscription is malformed.
type ErrInvalidPushSubscription struct {
	Reason string
}

// IsErrInvalidPushSubscription checks if an error is ErrInvalidPushSubscription.
func IsErrInvalidPushSubscription(err error) bool {
	var errInvalidPushSubscription ErrInvalidPushSubscription
	ok := errors.As(err, &errInvalidPushSubscription)
	return ok
}

func (err ErrInvalidPushSubscription) Error() string {
	return fmt.Sprintf("Invalid push subscription [Reason: %s]", err.Reason)
}

// ErrCodeInvalidPushSubscription holds the unique world-error code of this error
const ErrCodeInvalidPushSubscription = 20301

// HTTPError holds the http error description
func (err ErrInvalidPushSubscription) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeInvalidPushSubscription, Message: "Invalid push subscription: " + err.Reason + "."}
}

// ErrPushSubscriptionDoesNotExist represents an error where a push subscription to remove was not found.
type ErrPushSubscriptionDoesNotExist struct{}

// IsErrPushSubscriptionDoesNotExist checks if an error is ErrPushSubscriptionDoesNotExist.
func IsErrPushSubscriptionDoesNotExist(err error) bool {
	var errPushSubscriptionDoesNotExist ErrPushSubscriptionDoesNotExist
	ok := errors.As(err, &errPushSubscriptionDoesNotExist)
	return ok
}

func (err ErrPushSubscriptionDoesNotExist) Error() string {
	return "Push subscription does not exist"
}

// ErrCodePushSubscriptionDoesNotExist holds the unique world-error code of this error
const ErrCodePushSubscriptionDoesNotExist = 20302

// HTTPError holds the http error description
func (err ErrPushSubscriptionDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusNotFound, Code: ErrCodePushSubscriptionDoesNotExist, Message: "This device is not subscribed."}
}
