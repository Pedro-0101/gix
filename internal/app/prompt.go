package app

import "time"

// localTimeHeader formats "now" the way every AI prompt needs it: a human stamp
// ("2006-01-02 15:04:05 (Monday)"), the zone name, and the UTC offset in whole
// hours. The capture, alert and chat prompts each injected this same block by
// hand (now.Zone() + now.Format(...)); centralizing it keeps the time contract
// — and any future field — in one place.
func localTimeHeader(now time.Time) (stamp, zone string, offsetHours int) {
	zoneName, offsetSec := now.Zone()
	return now.Format("2006-01-02 15:04:05 (Monday)"), zoneName, offsetSec / 3600
}
