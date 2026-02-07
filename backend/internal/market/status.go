package market

import (
	"time"
)

// Checker determines whether the US stock market is currently open.
// Based on NYSE regular trading hours: 09:30–16:00 America/New_York, Mon–Fri.
// Holidays are computed algorithmically — no static date lists required.
type Checker struct {
	loc *time.Location
}

// NewChecker creates a market status checker with the ET timezone.
func NewChecker() *Checker {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("EST", -5*3600)
	}
	return &Checker{loc: loc}
}

// Status holds the current market state.
type Status struct {
	IsOpen      bool       `json:"is_open"`
	CurrentTime time.Time  `json:"current_time"`
	NextOpen    *time.Time `json:"next_open,omitempty"`
	NextClose   *time.Time `json:"next_close,omitempty"`
	Timezone    string     `json:"timezone"`
}

// MarketOpen is 09:30 ET.
var MarketOpen = timeOfDay{Hour: 9, Min: 30}

// MarketClose is 16:00 ET.
var MarketClose = timeOfDay{Hour: 16, Min: 0}

type timeOfDay struct {
	Hour int
	Min  int
}

// Check returns the current market status.
func (c *Checker) Check() Status {
	return c.checkAt(time.Now())
}

// checkAt returns market status for a given time (useful for testing).
func (c *Checker) checkAt(now time.Time) Status {
	et := now.In(c.loc)

	s := Status{
		CurrentTime: et,
		Timezone:    "America/New_York",
	}

	if c.isTradingDay(et) && c.isDuringHours(et) {
		s.IsOpen = true
		closeTime := time.Date(et.Year(), et.Month(), et.Day(), MarketClose.Hour, MarketClose.Min, 0, 0, c.loc)
		s.NextClose = &closeTime
	} else {
		s.IsOpen = false
		nextOpen := c.findNextOpen(et)
		s.NextOpen = &nextOpen
	}

	return s
}

// IsOpen returns true if the market is currently open.
func (c *Checker) IsOpen() bool {
	return c.Check().IsOpen
}

// isTradingDay returns true if the given day is a weekday and not a market holiday.
func (c *Checker) isTradingDay(t time.Time) bool {
	dow := t.Weekday()
	if dow == time.Saturday || dow == time.Sunday {
		return false
	}
	return !isNYSEHoliday(t)
}

// isDuringHours returns true if the time is between 09:30 and 16:00 ET.
func (c *Checker) isDuringHours(t time.Time) bool {
	hour, min, _ := t.Clock()
	minuteOfDay := hour*60 + min
	openMinute := MarketOpen.Hour*60 + MarketOpen.Min
	closeMinute := MarketClose.Hour*60 + MarketClose.Min
	return minuteOfDay >= openMinute && minuteOfDay < closeMinute
}

// findNextOpen finds the next market open time from the given time.
func (c *Checker) findNextOpen(from time.Time) time.Time {
	// If today is a trading day and we're before market open, next open is today
	if c.isTradingDay(from) {
		todayOpen := time.Date(from.Year(), from.Month(), from.Day(), MarketOpen.Hour, MarketOpen.Min, 0, 0, c.loc)
		if from.Before(todayOpen) {
			return todayOpen
		}
	}

	// Scan forward up to 14 days (covers worst case: holiday + weekend combos)
	candidate := from.AddDate(0, 0, 1)
	for i := 0; i < 14; i++ {
		candidateDay := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, c.loc)
		if c.isTradingDay(candidateDay) {
			return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), MarketOpen.Hour, MarketOpen.Min, 0, 0, c.loc)
		}
		candidate = candidate.AddDate(0, 0, 1)
	}

	// Fallback: shouldn't reach here
	candidate = from.AddDate(0, 0, 1)
	for candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), MarketOpen.Hour, MarketOpen.Min, 0, 0, c.loc)
}

// ─── Algorithmic NYSE Holiday Computation ────────────────────────────
//
// NYSE observes 10 holidays. All are computed from date math alone:
//
//   1. New Year's Day          — January 1 (with observed shift)
//   2. Martin Luther King Day  — 3rd Monday of January
//   3. Presidents' Day         — 3rd Monday of February
//   4. Good Friday             — Friday before Easter (Computus algorithm)
//   5. Memorial Day            — Last Monday of May
//   6. Juneteenth              — June 19 (with observed shift, NYSE adopted 2022)
//   7. Independence Day        — July 4 (with observed shift)
//   8. Labor Day               — 1st Monday of September
//   9. Thanksgiving Day        — 4th Thursday of November
//  10. Christmas Day           — December 25 (with observed shift)
//
// Observed shift for fixed-date holidays:
//   - Falls on Saturday → observed preceding Friday
//   - Falls on Sunday   → observed following Monday
//
// Special case: if Jan 1 of the NEXT year falls on Saturday, NYSE also
// closes on Dec 31 of the current year (observed Friday for next year's
// New Year). This is checked explicitly.

// isNYSEHoliday returns true if the given date (in ET) is an NYSE-observed holiday.
func isNYSEHoliday(t time.Time) bool {
	year := t.Year()
	month := t.Month()
	day := t.Day()

	// Check all holidays for the current year
	for _, h := range nyseHolidaysForYear(year) {
		if h.Month() == month && h.Day() == day {
			return true
		}
	}

	// Special case: Dec 31 is observed when next year's Jan 1 falls on Saturday
	if month == time.December && day == 31 {
		nextNewYear := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)
		if nextNewYear.Weekday() == time.Saturday {
			return true
		}
	}

	return false
}

// nyseHolidaysForYear computes all NYSE-observed holiday dates for a given year.
// Returns dates in UTC (only month/day are compared, not timezone).
func nyseHolidaysForYear(year int) []time.Time {
	holidays := make([]time.Time, 0, 10)

	// 1. New Year's Day — January 1 (observed)
	holidays = append(holidays, observedDate(year, time.January, 1))

	// 2. MLK Day — 3rd Monday of January
	holidays = append(holidays, nthWeekday(year, time.January, time.Monday, 3))

	// 3. Presidents' Day — 3rd Monday of February
	holidays = append(holidays, nthWeekday(year, time.February, time.Monday, 3))

	// 4. Good Friday — Friday before Easter Sunday
	holidays = append(holidays, goodFriday(year))

	// 5. Memorial Day — Last Monday of May
	holidays = append(holidays, lastWeekday(year, time.May, time.Monday))

	// 6. Juneteenth — June 19 (observed, NYSE adopted 2022+)
	if year >= 2022 {
		holidays = append(holidays, observedDate(year, time.June, 19))
	}

	// 7. Independence Day — July 4 (observed)
	holidays = append(holidays, observedDate(year, time.July, 4))

	// 8. Labor Day — 1st Monday of September
	holidays = append(holidays, nthWeekday(year, time.September, time.Monday, 1))

	// 9. Thanksgiving — 4th Thursday of November
	holidays = append(holidays, nthWeekday(year, time.November, time.Thursday, 4))

	// 10. Christmas — December 25 (observed)
	holidays = append(holidays, observedDate(year, time.December, 25))

	return holidays
}

// observedDate applies the standard NYSE observation rule to a fixed-date holiday:
//   - Saturday → preceding Friday
//   - Sunday   → following Monday
func observedDate(year int, month time.Month, day int) time.Time {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	switch t.Weekday() {
	case time.Saturday:
		return t.AddDate(0, 0, -1) // Friday
	case time.Sunday:
		return t.AddDate(0, 0, 1) // Monday
	default:
		return t
	}
}

// nthWeekday returns the nth occurrence of a weekday in a given month/year.
// Example: nthWeekday(2026, January, Monday, 3) → 3rd Monday of Jan 2026.
func nthWeekday(year int, month time.Month, weekday time.Weekday, n int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Days until the first occurrence of the target weekday
	daysUntil := int(weekday - first.Weekday())
	if daysUntil < 0 {
		daysUntil += 7
	}

	// Advance to the nth occurrence
	day := 1 + daysUntil + (n-1)*7
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// lastWeekday returns the last occurrence of a weekday in a given month/year.
// Example: lastWeekday(2026, May, Monday) → last Monday of May 2026.
func lastWeekday(year int, month time.Month, weekday time.Weekday) time.Time {
	// Last day of the month: day 0 of the next month
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)

	// Walk backward to find the target weekday
	daysBack := int(lastDay.Weekday() - weekday)
	if daysBack < 0 {
		daysBack += 7
	}

	return lastDay.AddDate(0, 0, -daysBack)
}

// goodFriday computes Good Friday for a given year.
// Good Friday is the Friday before Easter Sunday.
func goodFriday(year int) time.Time {
	easter := easterSunday(year)
	return easter.AddDate(0, 0, -2)
}

// easterSunday computes Easter Sunday using the Anonymous Gregorian algorithm.
// Valid for any year in the Gregorian calendar (1583+).
//
// Reference: https://en.wikipedia.org/wiki/Date_of_Easter#Anonymous_Gregorian_algorithm
//
// The algorithm uses only integer arithmetic on the year to produce the
// month (3=March or 4=April) and day of Easter. It has been proven correct
// for all years in the Gregorian calendar.
func easterSunday(year int) time.Time {
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
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
