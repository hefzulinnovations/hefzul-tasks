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

// Mirrors models.PushSubscription. The endpoint can be long, so its unique
// index is on a sha256 hash of it.
type pushSubscriptions20261014110423 struct {
	ID             int64     `xorm:"bigint autoincr not null unique pk"`
	UserID         int64     `xorm:"bigint not null INDEX"`
	Endpoint       string    `xorm:"text not null"`
	EndpointHash   string    `xorm:"varchar(64) not null unique"`
	P256dh         string    `xorm:"varchar(255) not null"`
	Auth           string    `xorm:"varchar(255) not null"`
	IncludeContent bool      `xorm:"bool not null default false"`
	Device         string    `xorm:"varchar(250) null"`
	Created        time.Time `xorm:"created not null"`
	LastUsed       time.Time `xorm:"datetime null"`
}

func (pushSubscriptions20261014110423) TableName() string {
	return "push_subscriptions"
}

// Mirrors models.WebPushKeys: the server's VAPID key pair.
type webPushKeys20261014110423 struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	PublicKey  string    `xorm:"varchar(255) not null"`
	PrivateKey string    `xorm:"varchar(255) not null"`
	Created    time.Time `xorm:"created not null"`
}

func (webPushKeys20261014110423) TableName() string {
	return "web_push_keys"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261014110423",
		Description: "Add push_subscriptions and web_push_keys tables",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync(pushSubscriptions20261014110423{}, webPushKeys20261014110423{}) //nolint:forbidigo // brand-new tables, nothing to drop
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(pushSubscriptions20261014110423{}, webPushKeys20261014110423{})
		},
	})
}
