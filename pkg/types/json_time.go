package types

import (
	"time"
)

const dateTimeLayout = `"2006-01-02 15:04:05"`
const dateOnlyLayout = `"2006-01-02"`
const timeOnlyLayout = `"15:04:05"`

type DateTime time.Time

func (t DateTime) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(t).Format(dateTimeLayout)), nil
}

func (t *DateTime) UnmarshalJSON(b []byte) error {
	parsed, err := time.Parse(dateTimeLayout, string(b))
	if err != nil {
		return err
	}
	*t = DateTime(parsed)
	return nil
}

type DateOnly time.Time

func (d DateOnly) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(d).Format(dateOnlyLayout)), nil
}

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	parsed, err := time.Parse(dateOnlyLayout, string(b))
	if err != nil {
		return err
	}
	*d = DateOnly(parsed)
	return nil
}

type TimeOnly time.Time

func (t TimeOnly) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(t).Format(timeOnlyLayout)), nil
}

func (t *TimeOnly) UnmarshalJSON(b []byte) error {
	parsed, err := time.Parse(timeOnlyLayout, string(b))
	if err != nil {
		return err
	}
	*t = TimeOnly(parsed)
	return nil
}
