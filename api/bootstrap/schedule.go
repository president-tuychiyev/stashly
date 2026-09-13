package bootstrap

import (
	"github.com/goravel/framework/contracts/schedule"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

// Schedule registers the recurring tasks of the service.
func Schedule() []schedule.Event {
	return []schedule.Event{
		facades.Schedule().Command("archives:cleanup").Hourly().SkipIfStillRunning(),
	}
}
