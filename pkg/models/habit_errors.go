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

// ErrHabitDoesNotExist represents an error where a habit does not exist or belongs to someone else.
type ErrHabitDoesNotExist struct {
	HabitID int64
}

// IsErrHabitDoesNotExist checks if an error is ErrHabitDoesNotExist.
func IsErrHabitDoesNotExist(err error) bool {
	var errHabitDoesNotExist ErrHabitDoesNotExist
	ok := errors.As(err, &errHabitDoesNotExist)
	return ok
}

func (err ErrHabitDoesNotExist) Error() string {
	return fmt.Sprintf("Habit does not exist [HabitID: %d]", err.HabitID)
}

// ErrCodeHabitDoesNotExist holds the unique world-error code of this error
const ErrCodeHabitDoesNotExist = 20101

// HTTPError holds the http error description
func (err ErrHabitDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusNotFound, Code: ErrCodeHabitDoesNotExist, Message: "This habit does not exist."}
}

// ErrInvalidHabit represents an error where a habit field has an invalid value.
type ErrInvalidHabit struct {
	Field  string
	Reason string
}

// IsErrInvalidHabit checks if an error is ErrInvalidHabit.
func IsErrInvalidHabit(err error) bool {
	var errInvalidHabit ErrInvalidHabit
	ok := errors.As(err, &errInvalidHabit)
	return ok
}

func (err ErrInvalidHabit) Error() string {
	return fmt.Sprintf("Invalid habit [Field: %s, Reason: %s]", err.Field, err.Reason)
}

// ErrCodeInvalidHabit holds the unique world-error code of this error
const ErrCodeInvalidHabit = 20102

// HTTPError holds the http error description
func (err ErrInvalidHabit) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeInvalidHabit, Message: fmt.Sprintf("%s %s.", err.Field, err.Reason)}
}

// ErrHabitCheckinDoesNotExist represents an error where there is no check-in to undo.
type ErrHabitCheckinDoesNotExist struct {
	HabitID int64
	Date    string
}

// IsErrHabitCheckinDoesNotExist checks if an error is ErrHabitCheckinDoesNotExist.
func IsErrHabitCheckinDoesNotExist(err error) bool {
	var errHabitCheckinDoesNotExist ErrHabitCheckinDoesNotExist
	ok := errors.As(err, &errHabitCheckinDoesNotExist)
	return ok
}

func (err ErrHabitCheckinDoesNotExist) Error() string {
	return fmt.Sprintf("Habit has no check-in on this date [HabitID: %d, Date: %s]", err.HabitID, err.Date)
}

// ErrCodeHabitCheckinDoesNotExist holds the unique world-error code of this error
const ErrCodeHabitCheckinDoesNotExist = 20103

// HTTPError holds the http error description
func (err ErrHabitCheckinDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusNotFound, Code: ErrCodeHabitCheckinDoesNotExist, Message: "There is no check-in to undo on this date."}
}

// ErrCountdownDoesNotExist represents an error where a countdown does not exist or belongs to someone else.
type ErrCountdownDoesNotExist struct {
	CountdownID int64
}

// IsErrCountdownDoesNotExist checks if an error is ErrCountdownDoesNotExist.
func IsErrCountdownDoesNotExist(err error) bool {
	var errCountdownDoesNotExist ErrCountdownDoesNotExist
	ok := errors.As(err, &errCountdownDoesNotExist)
	return ok
}

func (err ErrCountdownDoesNotExist) Error() string {
	return fmt.Sprintf("Countdown does not exist [CountdownID: %d]", err.CountdownID)
}

// ErrCodeCountdownDoesNotExist holds the unique world-error code of this error
const ErrCodeCountdownDoesNotExist = 20201

// HTTPError holds the http error description
func (err ErrCountdownDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusNotFound, Code: ErrCodeCountdownDoesNotExist, Message: "This countdown does not exist."}
}

// ErrInvalidCountdown represents an error where a countdown field has an invalid value.
type ErrInvalidCountdown struct {
	Field  string
	Reason string
}

// IsErrInvalidCountdown checks if an error is ErrInvalidCountdown.
func IsErrInvalidCountdown(err error) bool {
	var errInvalidCountdown ErrInvalidCountdown
	ok := errors.As(err, &errInvalidCountdown)
	return ok
}

func (err ErrInvalidCountdown) Error() string {
	return fmt.Sprintf("Invalid countdown [Field: %s, Reason: %s]", err.Field, err.Reason)
}

// ErrCodeInvalidCountdown holds the unique world-error code of this error
const ErrCodeInvalidCountdown = 20202

// HTTPError holds the http error description
func (err ErrInvalidCountdown) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeInvalidCountdown, Message: fmt.Sprintf("%s %s.", err.Field, err.Reason)}
}
