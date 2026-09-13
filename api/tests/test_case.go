package tests

import (
	"github.com/goravel/framework/testing"

	"github.com/president-tuychiyev/stashly/api/bootstrap"
)

func init() {
	bootstrap.Boot()
}

type TestCase struct {
	testing.TestCase
}
