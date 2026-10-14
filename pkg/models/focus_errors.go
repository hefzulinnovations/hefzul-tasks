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

// ErrFocusSessionDoesNotExist represents an error where a focus session does not exist
// or belongs to another user.
type ErrFocusSessionDoesNotExist struct {
	SessionID int64
}

// IsErrFocusSessionDoesNotExist checks if an error is ErrFocusSessionDoesNotExist.
func IsErrFocusSessionDoesNotExist(err error) bool {
	var errFocusSessionDoesNotExist ErrFocusSessionDoesNotExist
	ok := errors.As(err, &errFocusSessionDoesNotExist)
	return ok
}

func (err ErrFocusSessionDoesNotExist) Error() string {
	return fmt.Sprintf("Focus session does not exist [SessionID: %d]", err.SessionID)
}

// ErrCodeFocusSessionDoesNotExist holds the unique world-error code of this error
const ErrCodeFocusSessionDoesNotExist = 20001

// HTTPError holds the http error description
func (err ErrFocusSessionDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeFocusSessionDoesNotExist,
		Message:  "This focus session does not exist.",
	}
}

// ErrFocusSessionAlreadyRunning represents an error where a user starts a focus
// session while another one is still running or paused.
type ErrFocusSessionAlreadyRunning struct {
	SessionID int64
}

// IsErrFocusSessionAlreadyRunning checks if an error is ErrFocusSessionAlreadyRunning.
func IsErrFocusSessionAlreadyRunning(err error) bool {
	var errFocusSessionAlreadyRunning ErrFocusSessionAlreadyRunning
	ok := errors.As(err, &errFocusSessionAlreadyRunning)
	return ok
}

func (err ErrFocusSessionAlreadyRunning) Error() string {
	return fmt.Sprintf("A focus session is already running [SessionID: %d]", err.SessionID)
}

// ErrCodeFocusSessionAlreadyRunning holds the unique world-error code of this error
const ErrCodeFocusSessionAlreadyRunning = 20002

// HTTPError holds the http error description
func (err ErrFocusSessionAlreadyRunning) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusConflict,
		Code:     ErrCodeFocusSessionAlreadyRunning,
		Message:  fmt.Sprintf("You already have a running focus session (id %d). Finish or abandon it first.", err.SessionID),
	}
}

// ErrFocusSessionInvalidState represents an error where an action does not fit the
// session's current status, such as pausing a session that is already paused.
type ErrFocusSessionInvalidState struct {
	SessionID int64
	Status    FocusSessionStatus
	Action    string
}

// IsErrFocusSessionInvalidState checks if an error is ErrFocusSessionInvalidState.
func IsErrFocusSessionInvalidState(err error) bool {
	var errFocusSessionInvalidState ErrFocusSessionInvalidState
	ok := errors.As(err, &errFocusSessionInvalidState)
	return ok
}

func (err ErrFocusSessionInvalidState) Error() string {
	return fmt.Sprintf("Focus session cannot %s while %s [SessionID: %d]", err.Action, err.Status, err.SessionID)
}

// ErrCodeFocusSessionInvalidState holds the unique world-error code of this error
const ErrCodeFocusSessionInvalidState = 20003

// HTTPError holds the http error description
func (err ErrFocusSessionInvalidState) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusConflict,
		Code:     ErrCodeFocusSessionInvalidState,
		Message:  fmt.Sprintf("A %s focus session cannot %s.", err.Status, err.Action),
	}
}

// ErrInvalidFocusSettings represents an error where a focus timer setting is out of range.
type ErrInvalidFocusSettings struct {
	Field string
	Min   int
	Max   int
}

// IsErrInvalidFocusSettings checks if an error is ErrInvalidFocusSettings.
func IsErrInvalidFocusSettings(err error) bool {
	var errInvalidFocusSettings ErrInvalidFocusSettings
	ok := errors.As(err, &errInvalidFocusSettings)
	return ok
}

func (err ErrInvalidFocusSettings) Error() string {
	return fmt.Sprintf("Invalid focus setting [Field: %s, Min: %d, Max: %d]", err.Field, err.Min, err.Max)
}

// ErrCodeInvalidFocusSettings holds the unique world-error code of this error
const ErrCodeInvalidFocusSettings = 20004

// HTTPError holds the http error description
func (err ErrInvalidFocusSettings) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeInvalidFocusSettings,
		Message:  fmt.Sprintf("%s must be between %d and %d.", err.Field, err.Min, err.Max),
	}
}

// ErrInvalidFocusSessionKind represents an error where a focus session has an unknown kind.
type ErrInvalidFocusSessionKind struct {
	Kind FocusSessionKind
}

// IsErrInvalidFocusSessionKind checks if an error is ErrInvalidFocusSessionKind.
func IsErrInvalidFocusSessionKind(err error) bool {
	var errInvalidFocusSessionKind ErrInvalidFocusSessionKind
	ok := errors.As(err, &errInvalidFocusSessionKind)
	return ok
}

func (err ErrInvalidFocusSessionKind) Error() string {
	return fmt.Sprintf("Invalid focus session kind [Kind: %s]", err.Kind)
}

// ErrCodeInvalidFocusSessionKind holds the unique world-error code of this error
const ErrCodeInvalidFocusSessionKind = 20005

// HTTPError holds the http error description
func (err ErrInvalidFocusSessionKind) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeInvalidFocusSessionKind,
		Message:  "kind must be one of focus, short_break or long_break.",
	}
}
