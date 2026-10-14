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

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// FocusSettings holds one user's Pomodoro timer settings. The row is created the
// first time the user saves settings; until then the defaults apply.
type FocusSettings struct {
	UserID int64 `xorm:"bigint not null pk" json:"-"`

	FocusMinutes            int    `xorm:"int not null default 25" json:"focus_minutes" minimum:"1" maximum:"180" doc:"Length of a focus session in minutes."`
	ShortBreakMinutes       int    `xorm:"int not null default 5" json:"short_break_minutes" minimum:"1" maximum:"60" doc:"Length of a short break in minutes."`
	LongBreakMinutes        int    `xorm:"int not null default 15" json:"long_break_minutes" minimum:"1" maximum:"120" doc:"Length of a long break in minutes."`
	SessionsBeforeLongBreak int    `xorm:"int not null default 3" json:"sessions_before_long_break" minimum:"1" maximum:"12" doc:"Completed focus sessions before the next break is a long one."`
	AutoStartBreaks         bool   `xorm:"bool not null default false" json:"auto_start_breaks" doc:"Start the break right after a focus session completes."`
	Sound                   string `xorm:"varchar(50) not null default 'bell'" json:"sound" maxLength:"50" doc:"Sound played when a session ends; none for silence."`

	Created time.Time `xorm:"created not null" json:"-"`
	Updated time.Time `xorm:"updated not null" json:"-"`
}

// TableName is focus_settings.
func (*FocusSettings) TableName() string {
	return "focus_settings"
}

func defaultFocusSettings(userID int64) *FocusSettings {
	return &FocusSettings{
		UserID:                  userID,
		FocusMinutes:            25,
		ShortBreakMinutes:       5,
		LongBreakMinutes:        15,
		SessionsBeforeLongBreak: 3,
		Sound:                   "bell",
	}
}

func (fs *FocusSettings) plannedSeconds(kind FocusSessionKind) int64 {
	switch kind {
	case FocusSessionKindShortBreak:
		return int64(fs.ShortBreakMinutes) * 60
	case FocusSessionKindLongBreak:
		return int64(fs.LongBreakMinutes) * 60
	case FocusSessionKindFocus:
		return int64(fs.FocusMinutes) * 60
	}
	return int64(fs.FocusMinutes) * 60
}

func (fs *FocusSettings) validate() error {
	checks := []struct {
		field    string
		value    int
		min, max int
	}{
		{"focus_minutes", fs.FocusMinutes, 1, 180},
		{"short_break_minutes", fs.ShortBreakMinutes, 1, 60},
		{"long_break_minutes", fs.LongBreakMinutes, 1, 120},
		{"sessions_before_long_break", fs.SessionsBeforeLongBreak, 1, 12},
	}
	for _, c := range checks {
		if c.value < c.min || c.value > c.max {
			return ErrInvalidFocusSettings{Field: c.field, Min: c.min, Max: c.max}
		}
	}
	if len(fs.Sound) > 50 {
		return ErrInvalidFocusSettings{Field: "sound", Min: 0, Max: 50}
	}
	if fs.Sound == "" {
		fs.Sound = "bell"
	}
	return nil
}

func focusSettingsForUser(s *xorm.Session, userID int64) (*FocusSettings, error) {
	settings := &FocusSettings{}
	exists, err := s.Where("user_id = ?", userID).Get(settings)
	if err != nil {
		return nil, err
	}
	if !exists {
		return defaultFocusSettings(userID), nil
	}
	return settings, nil
}

// GetFocusSettings returns the authenticated user's timer settings, or the defaults.
func GetFocusSettings(s *xorm.Session, a web.Auth) (*FocusSettings, error) {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return nil, err
	}
	return focusSettingsForUser(s, userID)
}

// SaveFocusSettings replaces the authenticated user's timer settings.
func SaveFocusSettings(s *xorm.Session, a web.Auth, settings *FocusSettings) error {
	userID, err := focusAuthUserID(a)
	if err != nil {
		return err
	}
	if err := settings.validate(); err != nil {
		return err
	}
	settings.UserID = userID

	exists, err := s.Where("user_id = ?", userID).Exist(&FocusSettings{})
	if err != nil {
		return err
	}
	if !exists {
		_, err = s.Insert(settings)
		return err
	}
	_, err = s.Where("user_id = ?", userID).
		Cols("focus_minutes", "short_break_minutes", "long_break_minutes", "sessions_before_long_break", "auto_start_breaks", "sound").
		Update(settings)
	return err
}
