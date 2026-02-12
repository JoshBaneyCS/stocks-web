package market

import (
	"time"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// Eastern time zone for NYSE
var eastern *time.Location

func init() {
	var err error
	eastern, err = time.LoadLocation("America/New_York")
	if err != nil {
		panic("failed to load America/New_York timezone: " + err.Error())
	}
}

// Checker computes NYSE market status.
type Checker struct{}

// NewChecker creates a new market status Checker.
func NewChecker() *Checker {
	return &Checker{}
}

// IsMarketOpen returns true if the NYSE is currently in regular trading hours.
func (c *Checker) IsMarketOpen() bool {
	return isNYSEOpen(time.Now())
}

// GetMarketStatus returns the full market status including next open/close times.
func (c *Checker) GetMarketStatus() models.MarketStatus {
	now := time.Now()
	open := isNYSEOpen(now)

	status := models.MarketStatus{
		IsOpen: open,
	}

	if open {
		closeTime := todayClose(now)
		status.NextClose = &closeTime
		status.Message = "Market is open"
	} else {
		nextOpen := nextMarketOpen(now)
		status.NextOpen = &nextOpen
		status.Message = "Market is closed"
	}

	return status
}

// isNYSEOpen checks if the given time falls within NYSE regular trading hours.
func isNYSEOpen(t time.Time) bool {
	et := t.In(eastern)

	// Weekend check
	day := et.Weekday()
	if day == time.Saturday || day == time.Sunday {
		return false
	}

	// Holiday check
	if isNYSEHoliday(et) {
		return false
	}

	// Trading hours: 9:30 AM - 4:00 PM ET
	openTime := time.Date(et.Year(), et.Month(), et.Day(), 9, 30, 0, 0, eastern)
	closeTime := time.Date(et.Year(), et.Month(), et.Day(), 16, 0, 0, 0, eastern)

	return !et.Before(openTime) && et.Before(closeTime)
}

// todayClose returns 4:00 PM ET on the same day as t.
func todayClose(t time.Time) time.Time {
	et := t.In(eastern)
	return time.Date(et.Year(), et.Month(), et.Day(), 16, 0, 0, 0, eastern)
}

// nextMarketOpen finds the next time the market will open after t.
func nextMarketOpen(t time.Time) time.Time {
	et := t.In(eastern)
	candidate := et

	// If we're before today's open and today is a trading day, return today's open
	todayOpen := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 9, 30, 0, 0, eastern)
	if candidate.Before(todayOpen) && candidate.Weekday() != time.Saturday && candidate.Weekday() != time.Sunday && !isNYSEHoliday(candidate) {
		return todayOpen
	}

	// Otherwise advance to the next day and search
	candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day()+1, 9, 30, 0, 0, eastern)
	for i := 0; i < 14; i++ { // search up to 14 days ahead
		if candidate.Weekday() != time.Saturday && candidate.Weekday() != time.Sunday && !isNYSEHoliday(candidate) {
			return candidate
		}
		candidate = candidate.AddDate(0, 0, 1)
	}

	return candidate
}

// isNYSEHoliday checks if the given date is a US stock exchange holiday.
// Holidays observed by NYSE:
//   - New Year's Day (Jan 1)
//   - Martin Luther King Jr. Day (3rd Monday in January)
//   - Presidents' Day (3rd Monday in February)
//   - Good Friday
//   - Memorial Day (last Monday in May)
//   - Juneteenth (June 19)
//   - Independence Day (July 4)
//   - Labor Day (1st Monday in September)
//   - Thanksgiving (4th Thursday in November)
//   - Christmas (December 25)
func isNYSEHoliday(t time.Time) bool {
	year := t.Year()
	month := t.Month()
	day := t.Day()
	weekday := t.Weekday()

	// Fixed holidays with weekend adjustment
	if isObservedFixedHoliday(t, time.January, 1) { // New Year's Day
		return true
	}
	if isObservedFixedHoliday(t, time.June, 19) { // Juneteenth
		return true
	}
	if isObservedFixedHoliday(t, time.July, 4) { // Independence Day
		return true
	}
	if isObservedFixedHoliday(t, time.December, 25) { // Christmas
		return true
	}

	// MLK Day: 3rd Monday in January
	if month == time.January && weekday == time.Monday && day >= 15 && day <= 21 {
		return true
	}

	// Presidents' Day: 3rd Monday in February
	if month == time.February && weekday == time.Monday && day >= 15 && day <= 21 {
		return true
	}

	// Good Friday (Friday before Easter Sunday)
	gf := goodFriday(year)
	if month == gf.Month() && day == gf.Day() {
		return true
	}

	// Memorial Day: last Monday in May
	if month == time.May && weekday == time.Monday && day >= 25 {
		return true
	}

	// Labor Day: 1st Monday in September
	if month == time.September && weekday == time.Monday && day <= 7 {
		return true
	}

	// Thanksgiving: 4th Thursday in November
	if month == time.November && weekday == time.Thursday && day >= 22 && day <= 28 {
		return true
	}

	return false
}

// isObservedFixedHoliday checks if t is the observed date for a fixed holiday.
// If the holiday falls on Saturday, it's observed on Friday.
// If it falls on Sunday, it's observed on Monday.
func isObservedFixedHoliday(t time.Time, holidayMonth time.Month, holidayDay int) bool {
	if t.Month() != holidayMonth {
		// Check adjacent months for observed dates
		holiday := time.Date(t.Year(), holidayMonth, holidayDay, 0, 0, 0, 0, eastern)
		observed := observedDate(holiday)
		return t.Month() == observed.Month() && t.Day() == observed.Day()
	}

	holiday := time.Date(t.Year(), holidayMonth, holidayDay, 0, 0, 0, 0, eastern)
	observed := observedDate(holiday)
	return t.Day() == observed.Day() && t.Month() == observed.Month()
}

// observedDate adjusts a holiday date for weekend observation.
func observedDate(holiday time.Time) time.Time {
	switch holiday.Weekday() {
	case time.Saturday:
		return holiday.AddDate(0, 0, -1) // Friday
	case time.Sunday:
		return holiday.AddDate(0, 0, 1) // Monday
	default:
		return holiday
	}
}

// goodFriday computes the date of Good Friday for a given year
// using the Anonymous Gregorian algorithm for Easter.
func goodFriday(year int) time.Time {
	easter := computeEaster(year)
	return easter.AddDate(0, 0, -2)
}

// computeEaster calculates Easter Sunday using the Anonymous Gregorian algorithm.
func computeEaster(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, eastern)
}
