// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import "time"

// monthAbbrev maps a month to its short French label shown in the calendar.
var monthAbbrev = map[time.Month]string{
	time.January: "janv", time.February: "févr", time.March: "mars",
	time.April: "avr", time.May: "mai", time.June: "juin",
	time.July: "juil", time.August: "août", time.September: "sept",
	time.October: "oct", time.November: "nov", time.December: "déc",
}

// weekdayAbbrev maps a weekday to its short French label shown in the calendar
// (Monday = "lun").
var weekdayAbbrev = map[time.Weekday]string{
	time.Monday: "lun", time.Tuesday: "mar", time.Wednesday: "mer",
	time.Thursday: "jeu", time.Friday: "ven", time.Saturday: "sam",
	time.Sunday: "dim",
}

func translateMonth(m time.Month) string {
	if s, ok := monthAbbrev[m]; ok {
		return s
	}
	return "???"
}

func translateDay(d time.Weekday) string {
	if s, ok := weekdayAbbrev[d]; ok {
		return s
	}
	return "???"
}
