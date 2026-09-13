package main

import (
	"github.com/president-tuychiyev/stashly/api/bootstrap"
)

func main() {
	app := bootstrap.Boot()

	app.Start()
}
