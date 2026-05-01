package service

import (
	"fmt"
	"time"
)

const (
	TimezoneSingapore   = "Asia/Singapore"
	TimeZoneUSA         = "America/New_York"
	TimeZoneIndia       = "Asia/Kolkata"
	DateTimeFormatMySQL = "2006-01-02 15:04:05"
	DateTimeFormat      = "2006-01-02"
	DateTimeFormat2     = "2006/01/02"
)

type Time struct {
	Location *time.Location
}

func NewTime(location *time.Location) *Time {
	return &Time{
		Location: location,
	}
}

func (p *Time) Parse(format, value string) (time.Time, error) {
	t, err := time.ParseInLocation(format, value, p.Location)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse value %v in format %v: %w", value, format, err)
	}

	return t, nil
}

func (p *Time) CurrentTime() time.Time {
	return time.Now().In(p.Location)
}

func (p *Time) GetLocation(timeZone string) *time.Location {
	timeLocation, err := time.LoadLocation(timeZone)
	if err != nil {
		fmt.Errorf("Cannot load time location %v: %v", TimeZoneUSA, err)
	}

	return timeLocation
}
