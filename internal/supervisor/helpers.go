package supervisor

import "time"

func timeNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
