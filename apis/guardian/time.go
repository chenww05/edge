package guardian

import (
	"fmt"
	"time"
)

const GuardianTimeFormat = "2006-01-02T15:04:05.999999"

// GuardianTime format json time field by myself
type GuardianTime struct {
	Time time.Time
}

func (t *GuardianTime) UnmarshalJSON(data []byte) (err error) {
	if len(data) == 2 {
		*t = GuardianTime{Time: time.Time{}}
		return
	}
	loc, _ := time.LoadLocation("UTC")
	now, err := time.ParseInLocation(`"`+GuardianTimeFormat+`"`, string(data), loc)
	*t = GuardianTime{Time: now}
	return
}

// MarshalJSON on JSONTime format Time field with Y-m-d H:i:s
func (t *GuardianTime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	formatted := fmt.Sprintf("\"%s\"", t.Time.Format(GuardianTimeFormat))
	return []byte(formatted), nil
}
