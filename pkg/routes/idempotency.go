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

package routes

import (
	"bytes"
	"net/http"
	"strings"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth"
	"code.vikunja.io/api/pkg/modules/humabridge"
	"code.vikunja.io/api/pkg/web"

	"github.com/labstack/echo/v5"
)

// IdempotencyKeyHeader is the request header an offline client sets on every
// write it queues, so a replay after a lost response is applied only once.
const IdempotencyKeyHeader = "Idempotency-Key"

// ErrCodeIdempotencyKeyReused is returned when a key comes back with another request.
const ErrCodeIdempotencyKeyReused = 20401

// captureWriter keeps a copy of the response body while writing it through.
type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *captureWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// idempotencyMiddleware answers a repeated write that carries the same
// Idempotency-Key with the stored first result instead of running it again.
// Only results below 500 are stored, so a server error can be retried.
func idempotencyMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			key := strings.TrimSpace(c.Request().Header.Get(IdempotencyKeyHeader))
			if key == "" || !isWriteMethod(c.Request().Method) {
				return next(c)
			}
			// AutoPatch's internal GET/PUT legs belong to the client's PATCH.
			if _, internal := humabridge.InternalDispatchRoute(c.Request().Context()); internal {
				return next(c)
			}
			if len(key) > 255 {
				return c.JSON(http.StatusBadRequest, web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeIdempotencyKeyReused, Message: "Idempotency-Key must be at most 255 characters."})
			}
			a, err := auth.GetAuthFromClaims(c)
			if err != nil || a == nil {
				return next(c)
			}
			if _, isShare := a.(*models.LinkSharing); isShare {
				return next(c)
			}
			userID := a.GetID()
			methodAndPath := c.Request().Method + " " + c.Request().URL.Path

			s := db.NewSession()
			stored, err := models.GetIdempotencyKey(s, userID, key)
			s.Close()
			if err != nil {
				return err
			}
			if stored != nil {
				if stored.MethodAndPath != methodAndPath {
					return c.JSON(http.StatusUnprocessableEntity, web.HTTPError{
						HTTPCode: http.StatusUnprocessableEntity,
						Code:     ErrCodeIdempotencyKeyReused,
						Message:  "This Idempotency-Key was already used for a different request.",
					})
				}
				c.Response().Header().Set("Idempotent-Replayed", "true")
				if stored.ContentType != "" {
					c.Response().Header().Set(echo.HeaderContentType, stored.ContentType)
				}
				c.Response().WriteHeader(stored.ResponseStatus)
				_, err := c.Response().Write([]byte(stored.ResponseBody))
				return err
			}

			writer := &captureWriter{ResponseWriter: c.Response()}
			c.SetResponse(writer)
			handlerErr := next(c)
			c.SetResponse(writer.ResponseWriter)
			if handlerErr != nil || writer.status == 0 || writer.status >= 500 {
				return handlerErr
			}

			s = db.NewSession()
			defer s.Close()
			err = models.SaveIdempotencyKey(s, &models.IdempotencyKey{
				UserID:         userID,
				Key:            key,
				MethodAndPath:  methodAndPath,
				ResponseStatus: writer.status,
				ContentType:    writer.Header().Get(echo.HeaderContentType),
				ResponseBody:   writer.body.String(),
			})
			if err == nil {
				err = s.Commit()
			}
			if err != nil {
				// A concurrent duplicate stored its result first; this one is answered already.
				_ = s.Rollback()
				log.Debugf("[Idempotency] Could not store key for user %d: %s", userID, err)
			}
			return nil
		}
	}
}
