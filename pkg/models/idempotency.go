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
	"time"

	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"

	"xorm.io/xorm"
)

// IdempotencyKeyTTL is how long a stored result answers repeats of a write.
const IdempotencyKeyTTL = 7 * 24 * time.Hour

// IdempotencyKey stores the first result of a write request sent with an
// Idempotency-Key header, so an offline client replaying its outbox never
// applies the same change twice.
type IdempotencyKey struct {
	ID             int64     `xorm:"bigint autoincr not null unique pk"`
	UserID         int64     `xorm:"bigint not null unique(user_key)"`
	Key            string    `xorm:"varchar(255) not null unique(user_key)"`
	MethodAndPath  string    `xorm:"varchar(1000) not null"`
	ResponseStatus int       `xorm:"int not null"`
	ContentType    string    `xorm:"varchar(255) null"`
	ResponseBody   string    `xorm:"longtext null"`
	Created        time.Time `xorm:"created not null INDEX"`
}

// TableName is idempotency_keys.
func (*IdempotencyKey) TableName() string {
	return "idempotency_keys"
}

// GetIdempotencyKey returns the stored result for the user's key, or nil.
func GetIdempotencyKey(s *xorm.Session, userID int64, key string) (*IdempotencyKey, error) {
	// A struct condition lets xorm quote the column: key is reserved in MySQL.
	stored := &IdempotencyKey{UserID: userID, Key: key}
	exists, err := s.Get(stored)
	if err != nil || !exists {
		return nil, err
	}
	if time.Since(stored.Created) > IdempotencyKeyTTL {
		return nil, nil
	}
	return stored, nil
}

// SaveIdempotencyKey stores a request's result. A concurrent duplicate that
// stored first wins; the error is then ignored by the caller.
func SaveIdempotencyKey(s *xorm.Session, record *IdempotencyKey) error {
	_, err := s.Insert(record)
	return err
}

// RegisterIdempotencyKeyCleanupCron prunes stored results older than seven days.
func RegisterIdempotencyKeyCleanupCron() {
	err := cron.Schedule("17 * * * *", func() {
		s := db.NewSession()
		defer s.Close()
		n, err := s.Where("created < ?", time.Now().Add(-IdempotencyKeyTTL)).Delete(&IdempotencyKey{})
		if err != nil {
			log.Errorf("[Idempotency] Could not prune old keys: %s", err)
			_ = s.Rollback()
			return
		}
		if err := s.Commit(); err != nil {
			log.Errorf("[Idempotency] Could not prune old keys: %s", err)
			return
		}
		if n > 0 {
			log.Debugf("[Idempotency] Pruned %d old keys", n)
		}
	})
	if err != nil {
		log.Fatalf("Could not register idempotency key cleanup cron: %s", err)
	}
}
