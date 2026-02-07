package market

import (
	"time"
)

// Checker determines whether the US stock market is currently open.
// Based on NYSE regular trading hours: 09:30–16:00 America/New_York, Mon–Fri.
type Checker struct {
	loc      *time.Location
	holidays map[string]bool // keyed by "2006-01-02"
}

// NewChecker creates a market status checker with the ET timezone
// and a static set of known US market holidays for 2025–2027.
func NewChecker() *Checker {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback: UTC-5 fixed offset (no DST, but better than crashing)
		loc = time.FixedZone("EST", -5*3600)
	}

	// Known US market holidays (NYSE closed days).
	// Update annually or fetch from a calendar API.
	holidays := map[string]bool{
		// 2025
		"2025-01-01": true, // New Year's Day
		"2025-01-20": true, // MLK Day
		"2025-02-17": true, // Presidents' Day
		"2025-04-18": true, // Good Friday
		"2025-05-26": true, // Memorial Day
		"2025-06-19": true, // Juneteenth
		"2025-07-04": true, // Independence Day
		"2025-09-01": true, // Labor Day
		"2025-11-27": true, // Thanksgiving
		"2025-12-25": true, // Christmas
		// 2026
		"2026-01-01": true,
		"2026-01-19": true,
		"2026-02-16": true,
		"2026-04-03": true,
		"2026-05-25": true,
		"2026-06-19": true,
		"2026-07-03": true, // Independence Day observed
		"2026-09-07": true,
		"2026-11-26": true,
		"2026-12-25": true,
		// 2027
		"2027-01-01": true,
		"2027-01-18": true,
		"2027-02-15": true,
		"2027-03-26": true,
		"2027-05-31": true,
		"2027-06-18": true, // Juneteenth observed (falls on Sat)
		"2027-07-05": true, // Independence Day observed
		"2027-09-06": true,
		"2027-11-25": true,
		"2027-12-24": true, // Christmas observed
	}

	return &Checker{loc: loc, holidays: holidays}
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

// isTradingDay returns true if the given day is a weekday and not a holiday.
func (c *Checker) isTradingDay(t time.Time) bool {
	dow := t.Weekday()
	if dow == time.Saturday || dow == time.Sunday {
		return false
	}
	dateKey := t.Format("2006-01-02")
	return !c.holidays[dateKey]
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

	// Otherwise, scan forward up to 10 days to find the next trading day
	candidate := from.AddDate(0, 0, 1)
	for i := 0; i < 10; i++ {
		candidateDay := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, c.loc)
		if c.isTradingDay(candidateDay) {
			return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), MarketOpen.Hour, MarketOpen.Min, 0, 0, c.loc)
		}
		candidate = candidate.AddDate(0, 0, 1)
	}

	// Fallback: next weekday (shouldn't reach here)
	candidate = from.AddDate(0, 0, 1)
	for candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), MarketOpen.Hour, MarketOpen.Min, 0, 0, c.loc)
}
