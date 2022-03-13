package app

import "time"

// LocTime get local time
func LocTime() time.Time {
	return time.Now().Local()
}
