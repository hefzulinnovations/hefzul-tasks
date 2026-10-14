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

// Mirrors models.Habit.
type habits20261014102430 struct {
	ID           int64     `xorm:"bigint autoincr not null unique pk"`
	UserID       int64     `xorm:"bigint not null INDEX"`
	Title        string    `xorm:"varchar(250) not null"`
	Icon         string    `xorm:"varchar(50) null"`
	HexColor     string    `xorm:"varchar(6) null"`
	ScheduleKind string    `xorm:"varchar(20) not null default 'daily'"`
	Weekdays     int       `xorm:"int not null default 0"`
	TimesPerWeek int       `xorm:"int not null default 0"`
	DailyTarget  int       `xorm:"int not null default 1"`
	ReminderTime string    `xorm:"varchar(5) null"`
	Archived     bool      `xorm:"bool not null default false"`
	Position     int64     `xorm:"bigint not null default 0"`
	Created      time.Time `xorm:"created not null"`
	Updated      time.Time `xorm:"updated not null"`
}

func (habits20261014102430) TableName() string {
	return "habits"
}

// Mirrors models.HabitCheckin. date is the user's local calendar date as
// YYYY-MM-DD text, so it cannot shift with time zones.
type habitCheckins20261014102430 struct {
	ID      int64     `xorm:"bigint autoincr not null unique pk"`
	HabitID int64     `xorm:"bigint not null unique(habit_date)"`
	Date    string    `xorm:"varchar(10) not null unique(habit_date) INDEX"`
	Count   int64     `xorm:"bigint not null default 0"`
	Created time.Time `xorm:"created not null"`
	Updated time.Time `xorm:"updated not null"`
}

func (habitCheckins20261014102430) TableName() string {
	return "habit_checkins"
}

// Mirrors models.Countdown.
type countdowns20261014102430 struct {
	ID           int64     `xorm:"bigint autoincr not null unique pk"`
	UserID       int64     `xorm:"bigint not null INDEX"`
	Title        string    `xorm:"varchar(250) not null"`
	TargetDate   string    `xorm:"varchar(10) not null"`
	RepeatYearly bool      `xorm:"bool not null default false"`
	HexColor     string    `xorm:"varchar(6) null"`
	Pinned       bool      `xorm:"bool not null default false"`
	Created      time.Time `xorm:"created not null"`
	Updated      time.Time `xorm:"updated not null"`
}

func (countdowns20261014102430) TableName() string {
	return "countdowns"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261014102430",
		Description: "Add habits, habit_checkins and countdowns tables",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync(habits20261014102430{}, habitCheckins20261014102430{}, countdowns20261014102430{}) //nolint:forbidigo // brand-new tables, nothing to drop
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(habits20261014102430{}, habitCheckins20261014102430{}, countdowns20261014102430{})
		},
	})
}
