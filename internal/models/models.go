package models

type TimeRange struct {
	Period string
	Number int
}

const (
	DailyIndex    = 6
	HourlyIndex   = 7
	MinutelyIndex = 8
	MillisecondsInDay    = 86400000
	MillisecondsInHour   = 3600000
	MillisecondsInMinute = 60000
	DaysInMonth   = 31
	DaysInYear    = 365
	HoursInDay    = 24
	MinutesInHour = 60
)
