package bootstrap

import (
	"github.com/goravel/framework/contracts/queue"

	"github.com/president-tuychiyev/stashly/api/app/jobs"
)

func Jobs() []queue.Job {
	return []queue.Job{
		&jobs.ZipFolders{},
	}
}
