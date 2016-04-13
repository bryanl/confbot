package slack

// JSONTime exists so that we can have a String method converting the date
import (
	"fmt"
	"time"
)

// JSONTime is time in JSON (as an int64)
type JSONTime int64

// String converts the unix timestamp into a string
func (t JSONTime) String() string {
	tm := t.Time()
	return fmt.Sprintf("\"%s\"", tm.Format("Mon Jan _2"))
}

// Time returns a `time.Time` representation of this value.
func (t JSONTime) Time() time.Time {
	return time.Unix(int64(t), 0)
}
