package fyneapp

import "github.com/yuwen002/meteor-ch-workbench/internal/app"

var backend = app.New()

func Backend() *app.App { return backend }

func Shutdown() {
	if backend != nil {
		backend.Shutdown()
	}
}