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
	"regexp"
	"time"

	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// HabitScheduleKind says on which days a habit is due.
type HabitScheduleKind string

const (
	HabitScheduleDaily        HabitScheduleKind = "daily"
	HabitScheduleWeekdays     HabitScheduleKind = "weekdays"
	HabitScheduleTimesPerWeek HabitScheduleKind = "times_per_week"
)

var habitReminderTimePattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// Habit is a recurring personal routine a user checks in on, such as reading
// or exercise. Habits belong to one user only.
//
// v2-only: doc: tags are the schema's source of truth.
type Habit struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this habit."`
	UserID int64 `xorm:"bigint not null INDEX" json:"user_id" readOnly:"true" doc:"The owner. Set by the server."`

	Title    string `xorm:"varchar(250) not null" json:"title" minLength:"1" maxLength:"250" doc:"What the habit is, such as Read 20 pages."`
	Icon     string `xorm:"varchar(50) null" json:"icon" maxLength:"50" doc:"An emoji or icon name shown next to the title."`
	HexColor string `xorm:"varchar(6) null" json:"hex_color" valid:"hexcolor" maxLength:"7" doc:"The habit's colour as a hex string, with or without the leading #."`

	ScheduleKind HabitScheduleKind `xorm:"varchar(20) not null default 'daily'" json:"schedule_kind" enum:"daily,weekdays,times_per_week" doc:"daily: every day. weekdays: only the days in weekdays. times_per_week: any times_per_week days of each week."`
	Weekdays     int               `xorm:"int not null default 0" json:"weekdays" minimum:"0" maximum:"127" doc:"Bitmask of due days for schedule_kind weekdays: 1=Sunday, 2=Monday, 4=Tuesday … 64=Saturday."`
	TimesPerWeek int               `xorm:"int not null default 0" json:"times_per_week" minimum:"0" maximum:"7" doc:"How many days a week the habit is due, for schedule_kind times_per_week."`
	DailyTarget  int               `xorm:"int not null default 1" json:"daily_target" minimum:"0" maximum:"100" doc:"Check-ins needed for a day to count as done, such as 8 for 8 glasses of water. 0 is treated as 1."`
	ReminderTime string            `xorm:"varchar(5) null" json:"reminder_time" pattern:"^(([01][0-9]|2[0-3]):[0-5][0-9])?$" doc:"Time of day (HH:MM, in your time zone) to be reminded if the habit is not done yet; empty for no reminder."`

	Archived bool  `xorm:"bool not null default false" json:"archived" doc:"Archived habits are hidden from the list but keep their history."`
	Position int64 `xorm:"bigint not null default 0" json:"position" doc:"Sort position in the habit list, ascending."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this habit was created."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this habit was last changed."`

	// Computed for the authenticated user's "today", not stored.
	TodayCount    int64 `xorm:"-" json:"today_count" readOnly:"true" doc:"Check-ins today, in your time zone."`
	DoneToday     bool  `xorm:"-" json:"done_today" readOnly:"true" doc:"True when today's check-ins reach daily_target."`
	DueToday      bool  `xorm:"-" json:"due_today" readOnly:"true" doc:"True when the schedule asks for the habit today. Always true for times_per_week until the week's target is met."`
	CurrentStreak int   `xorm:"-" json:"current_streak" readOnly:"true" doc:"Current streak: days in a row for daily and weekdays habits, weeks in a row for times_per_week habits."`

	// Filter-only (not persisted): set by the v2 list route.
	IncludeArchived bool `xorm:"-" json:"-"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName is habits.
func (*Habit) TableName() string {
	return "habits"
}

// HabitCheckin counts the check-ins of one habit on one calendar day. The date
// is the user's local date, kept as text so it does not shift with time zones.
type HabitCheckin struct {
	ID      int64     `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The check-in row's id."`
	HabitID int64     `xorm:"bigint not null unique(habit_date)" json:"habit_id" readOnly:"true" doc:"The habit."`
	Date    string    `xorm:"varchar(10) not null unique(habit_date) INDEX" json:"date" doc:"Local calendar date, YYYY-MM-DD."`
	Count   int64     `xorm:"bigint not null default 0" json:"count" doc:"Check-ins on that day."`
	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When the day's first check-in was made."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"When the day's count last changed."`
}

// TableName is habit_checkins.
func (*HabitCheckin) TableName() string {
	return "habit_checkins"
}

// calendarDate trims a stored YYYY-MM-DD value that a database or fixture
// loader returned with a time part attached.
func calendarDate(v string) string {
	if len(v) > 10 {
		return v[:10]
	}
	return v
}

func (h *Habit) target() int64 {
	if h.DailyTarget <= 0 {
		return 1
	}
	return int64(h.DailyTarget)
}

// scheduledOn reports whether a daily or weekdays habit is due on that weekday.
func (h *Habit) scheduledOn(day time.Weekday) bool {
	if h.ScheduleKind == HabitScheduleWeekdays {
		return h.Weekdays&(1<<uint(day)) != 0
	}
	return true
}

func (h *Habit) validate() error {
	switch h.ScheduleKind {
	case "":
		h.ScheduleKind = HabitScheduleDaily
	case HabitScheduleDaily:
	case HabitScheduleWeekdays:
		if h.Weekdays <= 0 || h.Weekdays > 127 {
			return ErrInvalidHabit{Field: "weekdays", Reason: "pick at least one day"}
		}
	case HabitScheduleTimesPerWeek:
		if h.TimesPerWeek < 1 || h.TimesPerWeek > 7 {
			return ErrInvalidHabit{Field: "times_per_week", Reason: "must be between 1 and 7"}
		}
	default:
		return ErrInvalidHabit{Field: "schedule_kind", Reason: "must be daily, weekdays or times_per_week"}
	}
	if h.ReminderTime != "" && !habitReminderTimePattern.MatchString(h.ReminderTime) {
		return ErrInvalidHabit{Field: "reminder_time", Reason: "must be HH:MM"}
	}
	if h.Title == "" {
		return ErrInvalidHabit{Field: "title", Reason: "must not be empty"}
	}
	if h.DailyTarget <= 0 {
		h.DailyTarget = 1
	}
	h.HexColor = utils.NormalizeHex(h.HexColor)
	return nil
}

func habitOwner(a web.Auth) (int64, bool) {
	if _, isShare := a.(*LinkSharing); isShare {
		return 0, false
	}
	return a.GetID(), true
}

func getHabitByID(s *xorm.Session, id int64) (*Habit, error) {
	h := &Habit{}
	exists, err := s.Where("id = ?", id).Get(h)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrHabitDoesNotExist{HabitID: id}
	}
	return h, nil
}

// getOwnHabit loads a habit of the authenticated user; another user's habit
// reads as nonexistent.
func getOwnHabit(s *xorm.Session, a web.Auth, id int64) (*Habit, error) {
	userID, ok := habitOwner(a)
	if !ok {
		return nil, ErrGenericForbidden{}
	}
	h, err := getHabitByID(s, id)
	if err != nil {
		return nil, err
	}
	if h.UserID != userID {
		return nil, ErrHabitDoesNotExist{HabitID: id}
	}
	return h, nil
}

// --- Permissions ---

// CanRead lets only the owner read a habit.
func (h *Habit) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	existing, err := getOwnHabit(s, a, h.ID)
	if err != nil {
		if IsErrGenericForbidden(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	*h = *existing
	return true, int(PermissionAdmin), nil
}

// CanCreate lets any signed-in user create habits for themselves.
func (h *Habit) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {
	_, ok := habitOwner(a)
	return ok, nil
}

// CanUpdate lets only the owner change a habit.
func (h *Habit) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	return h.canModify(s, a)
}

// CanDelete lets only the owner delete a habit.
func (h *Habit) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	return h.canModify(s, a)
}

func (h *Habit) canModify(s *xorm.Session, a web.Auth) (bool, error) {
	_, err := getOwnHabit(s, a, h.ID)
	if IsErrGenericForbidden(err) {
		return false, nil
	}
	return err == nil, err
}

// --- CRUDable ---

// Create stores a new habit owned by the authenticated user.
func (h *Habit) Create(s *xorm.Session, a web.Auth) error {
	if err := h.validate(); err != nil {
		return err
	}
	h.ID = 0
	h.UserID = a.GetID()
	if _, err := s.Insert(h); err != nil {
		return err
	}
	return addHabitProgress(s, h.UserID, []*Habit{h})
}

// ReadOne fills the computed progress fields; the habit was loaded in CanRead.
func (h *Habit) ReadOne(s *xorm.Session, _ web.Auth) error {
	return addHabitProgress(s, h.UserID, []*Habit{h})
}

// ReadAll returns the user's habits by position, archived ones only on request.
func (h *Habit) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, total int64, err error) {
	userID, ok := habitOwner(a)
	if !ok {
		return []*Habit{}, 0, 0, nil
	}
	cond := builder.NewCond().And(builder.Eq{"user_id": userID})
	if !h.IncludeArchived {
		cond = cond.And(builder.Eq{"archived": false})
	}
	if search != "" {
		cond = cond.And(builder.Like{"title", search})
	}
	total, err = s.Where(cond).Count(&Habit{})
	if err != nil {
		return nil, 0, 0, err
	}
	habits := []*Habit{}
	err = s.Where(cond).OrderBy("position ASC, id ASC").Limit(getLimitFromPageIndex(page, perPage)).Find(&habits)
	if err != nil {
		return nil, 0, 0, err
	}
	if err := addHabitProgress(s, userID, habits); err != nil {
		return nil, 0, 0, err
	}
	return habits, len(habits), total, nil
}

// Update replaces the editable fields of a habit.
func (h *Habit) Update(s *xorm.Session, _ web.Auth) error {
	if err := h.validate(); err != nil {
		return err
	}
	existing, err := getHabitByID(s, h.ID)
	if err != nil {
		return err
	}
	_, err = s.ID(h.ID).
		Cols("title", "icon", "hex_color", "schedule_kind", "weekdays", "times_per_week", "daily_target", "reminder_time", "archived", "position").
		Update(h)
	if err != nil {
		return err
	}
	updated, err := getHabitByID(s, h.ID)
	if err != nil {
		return err
	}
	*h = *updated
	return addHabitProgress(s, existing.UserID, []*Habit{h})
}

// Delete removes a habit and all its check-ins.
func (h *Habit) Delete(s *xorm.Session, _ web.Auth) error {
	if _, err := s.Where("habit_id = ?", h.ID).Delete(&HabitCheckin{}); err != nil {
		return err
	}
	_, err := s.ID(h.ID).Delete(&Habit{})
	return err
}

// --- Check-ins ---

// CheckInHabit adds one check-in for the date (YYYY-MM-DD, default today in the
// user's time zone) and returns that day's row.
func CheckInHabit(s *xorm.Session, a web.Auth, habitID int64, date string) (*HabitCheckin, error) {
	h, err := getOwnHabit(s, a, habitID)
	if err != nil {
		return nil, err
	}
	date, err = habitDateOrToday(s, h.UserID, date)
	if err != nil {
		return nil, err
	}

	checkin := &HabitCheckin{}
	exists, err := s.Where("habit_id = ? AND date = ?", habitID, date).Get(checkin)
	if err != nil {
		return nil, err
	}
	if exists {
		checkin.Count++
		if _, err := s.ID(checkin.ID).Cols("count").Update(checkin); err != nil {
			return nil, err
		}
	} else {
		checkin = &HabitCheckin{HabitID: habitID, Date: date, Count: 1}
		if _, err := s.Insert(checkin); err != nil {
			return nil, err
		}
	}

	u, err := user.GetUserByID(s, h.UserID)
	if err != nil {
		return nil, err
	}
	events.DispatchOnCommit(s, &HabitCheckinCreatedEvent{Habit: h, Checkin: checkin, User: u})
	return checkin, nil
}

// UndoHabitCheckin removes the last check-in of the date; the day's row goes
// away when its count reaches zero. Returns the row, with count 0 if removed.
func UndoHabitCheckin(s *xorm.Session, a web.Auth, habitID int64, date string) (*HabitCheckin, error) {
	h, err := getOwnHabit(s, a, habitID)
	if err != nil {
		return nil, err
	}
	date, err = habitDateOrToday(s, h.UserID, date)
	if err != nil {
		return nil, err
	}
	checkin := &HabitCheckin{}
	exists, err := s.Where("habit_id = ? AND date = ?", habitID, date).Get(checkin)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrHabitCheckinDoesNotExist{HabitID: habitID, Date: date}
	}
	checkin.Count--
	if checkin.Count <= 0 {
		checkin.Count = 0
		_, err = s.ID(checkin.ID).Delete(&HabitCheckin{})
		return checkin, err
	}
	_, err = s.ID(checkin.ID).Cols("count").Update(checkin)
	return checkin, err
}

func habitDateOrToday(s *xorm.Session, userID int64, date string) (string, error) {
	if date == "" {
		loc, err := focusUserLocation(s, userID)
		if err != nil {
			return "", err
		}
		return time.Now().In(loc).Format(time.DateOnly), nil
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return "", ErrInvalidHabit{Field: "date", Reason: "must be YYYY-MM-DD"}
	}
	return date, nil
}

// deleteHabitDataForUser removes a user's habits, their check-ins and countdowns.
func deleteHabitDataForUser(s *xorm.Session, userID int64) error {
	ids := []int64{}
	if err := s.Table("habits").Where("user_id = ?", userID).Cols("id").Find(&ids); err != nil {
		return err
	}
	if len(ids) > 0 {
		if _, err := s.In("habit_id", ids).Delete(&HabitCheckin{}); err != nil {
			return err
		}
	}
	if _, err := s.Where("user_id = ?", userID).Delete(&Habit{}); err != nil {
		return err
	}
	_, err := s.Where("user_id = ?", userID).Delete(&Countdown{})
	return err
}
