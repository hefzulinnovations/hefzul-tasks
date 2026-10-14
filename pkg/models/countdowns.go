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
	"sort"
	"time"

	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Countdown counts the days to a date, such as a birthday or a deadline.
// Countdowns belong to one user only.
//
// v2-only: doc: tags are the schema's source of truth.
type Countdown struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this countdown."`
	UserID int64 `xorm:"bigint not null INDEX" json:"user_id" readOnly:"true" doc:"The owner. Set by the server."`

	Title        string `xorm:"varchar(250) not null" json:"title" minLength:"1" maxLength:"250" doc:"What the countdown is for, such as Mum's birthday."`
	TargetDate   string `xorm:"varchar(10) not null" json:"target_date" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"The date counted down to, YYYY-MM-DD. For a yearly countdown, the first occurrence (the year matters only for the years count)."`
	RepeatYearly bool   `xorm:"bool not null default false" json:"repeat_yearly" doc:"Repeat every year on the same day, such as a birthday or an anniversary."`
	HexColor     string `xorm:"varchar(6) null" json:"hex_color" valid:"hexcolor" maxLength:"7" doc:"The countdown's colour as a hex string, with or without the leading #."`
	Pinned       bool   `xorm:"bool not null default false" json:"pinned" doc:"Pinned countdowns always show on the dashboard, not only in their last 7 days."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this countdown was created."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this countdown was last changed."`

	// Computed for the owner's "today", not stored.
	NextDate      string `xorm:"-" json:"next_date" readOnly:"true" doc:"The next occurrence (YYYY-MM-DD); for a one-off countdown, the target date itself."`
	DaysRemaining int    `xorm:"-" json:"days_remaining" readOnly:"true" doc:"Days from today to next_date in your time zone: 0 is today, negative once a one-off date has passed."`
	Years         int    `xorm:"-" json:"years" readOnly:"true" doc:"For a yearly countdown, how many years next_date is after target_date (such as the age at a birthday); 0 otherwise."`

	// Filter-only (not persisted): set by the v2 list route.
	WithinDays int `xorm:"-" json:"-"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName is countdowns.
func (*Countdown) TableName() string {
	return "countdowns"
}

func (c *Countdown) validate() error {
	if c.Title == "" {
		return ErrInvalidCountdown{Field: "title", Reason: "must not be empty"}
	}
	if _, err := time.Parse(time.DateOnly, c.TargetDate); err != nil {
		return ErrInvalidCountdown{Field: "target_date", Reason: "must be a date in the form YYYY-MM-DD"}
	}
	c.HexColor = utils.NormalizeHex(c.HexColor)
	return nil
}

// nextOccurrence returns the date on or after today the countdown points at.
// A yearly countdown on 29 February falls on 28 February in other years.
func nextOccurrence(target time.Time, repeatYearly bool, today time.Time) (time.Time, int) {
	if !repeatYearly {
		return target, 0
	}
	for year := today.Year(); ; year++ {
		day := target.Day()
		if target.Month() == time.February && day == 29 && !isLeapYear(year) {
			day = 28
		}
		candidate := time.Date(year, target.Month(), day, 0, 0, 0, 0, today.Location())
		if !candidate.Before(today) {
			return candidate, max(0, year-target.Year())
		}
	}
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// addCountdownProgress fills next_date, days_remaining and years.
func addCountdownProgress(s *xorm.Session, userID int64, countdowns []*Countdown) error {
	if len(countdowns) == 0 {
		return nil
	}
	loc, err := focusUserLocation(s, userID)
	if err != nil {
		return err
	}
	today := dayStart(time.Now().In(loc))
	for _, c := range countdowns {
		c.TargetDate = calendarDate(c.TargetDate)
		target, err := time.ParseInLocation(time.DateOnly, c.TargetDate, loc)
		if err != nil {
			continue
		}
		next, years := nextOccurrence(target, c.RepeatYearly, today)
		c.NextDate = next.Format(time.DateOnly)
		c.Years = years
		// Round: a day across a DST change is 23 or 25 hours long.
		c.DaysRemaining = int((next.Sub(today).Hours() + 12) / 24)
		if next.Before(today) {
			c.DaysRemaining = -int((today.Sub(next).Hours() + 12) / 24)
		}
	}
	return nil
}

func getOwnCountdown(s *xorm.Session, a web.Auth, id int64) (*Countdown, error) {
	userID, ok := habitOwner(a)
	if !ok {
		return nil, ErrGenericForbidden{}
	}
	c := &Countdown{}
	exists, err := s.Where("id = ?", id).Get(c)
	if err != nil {
		return nil, err
	}
	if !exists || c.UserID != userID {
		return nil, ErrCountdownDoesNotExist{CountdownID: id}
	}
	return c, nil
}

// --- Permissions ---

// CanRead lets only the owner read a countdown.
func (c *Countdown) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	existing, err := getOwnCountdown(s, a, c.ID)
	if IsErrGenericForbidden(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	*c = *existing
	return true, int(PermissionAdmin), nil
}

// CanCreate lets any signed-in user create countdowns for themselves.
func (c *Countdown) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {
	_, ok := habitOwner(a)
	return ok, nil
}

// CanUpdate lets only the owner change a countdown.
func (c *Countdown) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	return c.canModify(s, a)
}

// CanDelete lets only the owner delete a countdown.
func (c *Countdown) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	return c.canModify(s, a)
}

func (c *Countdown) canModify(s *xorm.Session, a web.Auth) (bool, error) {
	_, err := getOwnCountdown(s, a, c.ID)
	if IsErrGenericForbidden(err) {
		return false, nil
	}
	return err == nil, err
}

// --- CRUDable ---

// Create stores a new countdown owned by the authenticated user.
func (c *Countdown) Create(s *xorm.Session, a web.Auth) error {
	if err := c.validate(); err != nil {
		return err
	}
	c.ID = 0
	c.UserID = a.GetID()
	if _, err := s.Insert(c); err != nil {
		return err
	}
	return addCountdownProgress(s, c.UserID, []*Countdown{c})
}

// ReadOne fills the computed fields; the countdown was loaded in CanRead.
func (c *Countdown) ReadOne(s *xorm.Session, _ web.Auth) error {
	return addCountdownProgress(s, c.UserID, []*Countdown{c})
}

// ReadAll returns the user's countdowns, soonest first. With WithinDays set,
// only pinned ones and those due within that many days (passed one-offs excluded).
func (c *Countdown) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, total int64, err error) {
	userID, ok := habitOwner(a)
	if !ok {
		return []*Countdown{}, 0, 0, nil
	}
	cond := builder.NewCond().And(builder.Eq{"user_id": userID})
	if search != "" {
		cond = cond.And(builder.Like{"title", search})
	}
	all := []*Countdown{}
	if err := s.Where(cond).Find(&all); err != nil {
		return nil, 0, 0, err
	}
	if err := addCountdownProgress(s, userID, all); err != nil {
		return nil, 0, 0, err
	}
	if c.WithinDays > 0 {
		filtered := all[:0]
		for _, cd := range all {
			if cd.Pinned || (cd.DaysRemaining >= 0 && cd.DaysRemaining <= c.WithinDays) {
				filtered = append(filtered, cd)
			}
		}
		all = filtered
	}
	// Upcoming first by days left, then passed one-offs, most recent first.
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i].DaysRemaining, all[j].DaysRemaining
		if (a < 0) != (b < 0) {
			return a >= 0
		}
		if a < 0 {
			return a > b
		}
		if a != b {
			return a < b
		}
		return all[i].ID < all[j].ID
	})
	total = int64(len(all))
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		if start > len(all) {
			start = len(all)
		}
		end := min(len(all), start+limit)
		all = all[start:end]
	}
	return all, len(all), total, nil
}

// Update replaces the editable fields of a countdown.
func (c *Countdown) Update(s *xorm.Session, _ web.Auth) error {
	if err := c.validate(); err != nil {
		return err
	}
	if _, err := s.ID(c.ID).Cols("title", "target_date", "repeat_yearly", "hex_color", "pinned").Update(c); err != nil {
		return err
	}
	updated := &Countdown{}
	if _, err := s.ID(c.ID).Get(updated); err != nil {
		return err
	}
	*c = *updated
	return addCountdownProgress(s, c.UserID, []*Countdown{c})
}

// Delete removes a countdown.
func (c *Countdown) Delete(s *xorm.Session, _ web.Auth) error {
	_, err := s.ID(c.ID).Delete(&Countdown{})
	return err
}
