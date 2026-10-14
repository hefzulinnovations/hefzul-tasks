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

package migration

import (
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

// Mirrors models.IdempotencyKey: the first result of a write sent with an
// Idempotency-Key header, kept for seven days.
type idempotencyKeys20261014112007 struct {
	ID             int64     `xorm:"bigint autoincr not null unique pk"`
	UserID         int64     `xorm:"bigint not null unique(user_key)"`
	Key            string    `xorm:"varchar(255) not null unique(user_key)"`
	MethodAndPath  string    `xorm:"varchar(1000) not null"`
	ResponseStatus int       `xorm:"int not null"`
	ContentType    string    `xorm:"varchar(255) null"`
	ResponseBody   string    `xorm:"longtext null"`
	Created        time.Time `xorm:"created not null INDEX"`
}

func (idempotencyKeys20261014112007) TableName() string {
	return "idempotency_keys"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261014112007",
		Description: "Add idempotency_keys table for replayed offline writes",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync(idempotencyKeys20261014112007{}) //nolint:forbidigo // brand-new table, nothing to drop
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(idempotencyKeys20261014112007{})
		},
	})
}
