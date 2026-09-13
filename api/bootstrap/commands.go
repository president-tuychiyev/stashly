package bootstrap

import (
	"github.com/goravel/framework/contracts/console"

	"github.com/president-tuychiyev/stashly/api/app/console/commands"
)

func Commands() []console.Command {
	return []console.Command{
		commands.NewArchivesCleanup(),
		commands.NewStorageSync(),
	}
}
