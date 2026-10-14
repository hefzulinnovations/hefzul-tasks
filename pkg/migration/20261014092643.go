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

// Mirrors models.FocusSettings.
type focusSettings20261014092643 struct {
	UserID                  int64     `xorm:"bigint not null pk"`
	FocusMinutes            int       `xorm:"int not null default 25"`
	ShortBreakMinutes       int       `xorm:"int not null default 5"`
	LongBreakMinutes        int       `xorm:"int not null default 15"`
	SessionsBeforeLongBreak int       `xorm:"int not null default 3"`
	AutoStartBreaks         bool      `xorm:"bool not null default false"`
	Sound                   string    `xorm:"varchar(50) not null default 'bell'"`
	Created                 time.Time `xorm:"created not null"`
	Updated                 time.Time `xorm:"updated not null"`
}

func (focusSettings20261014092643) TableName() string {
	return "focus_settings"
}

// Mirrors models.FocusSession. No partial unique index for the one-active-session
// rule: MySQL has no filtered indexes, so the model enforces it.
type focusSessions20261014092643 struct {
	ID             int64      `xorm:"bigint autoincr not null unique pk"`
	UserID         int64      `xorm:"bigint not null INDEX"`
	TaskID         int64      `xorm:"bigint null INDEX"`
	Kind           string     `xorm:"varchar(20) not null"`
	PlannedSeconds int64      `xorm:"bigint not null"`
	ActualSeconds  int64      `xorm:"bigint not null default 0"`
	Status         string     `xorm:"varchar(20) not null INDEX"`
	StartedAt      time.Time  `xorm:"not null INDEX"`
	EndedAt        *time.Time `xorm:"null"`
	PausedAt       *time.Time `xorm:"null"`
	PausedSeconds  int64      `xorm:"bigint not null default 0"`
	Device         string     `xorm:"varchar(100) null"`
	Created        time.Time  `xorm:"created not null"`
	Updated        time.Time  `xorm:"updated not null"`
}

func (focusSessions20261014092643) TableName() string {
	return "focus_sessions"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261014092643",
		Description: "Add focus_settings and focus_sessions tables for the Pomodoro timer",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync(focusSettings20261014092643{}, focusSessions20261014092643{}) //nolint:forbidigo // brand-new tables, nothing to drop
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(focusSettings20261014092643{}, focusSessions20261014092643{})
		},
	})
}
