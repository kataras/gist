package gist

import (
	"gopkg.in/kataras/iris.v6"
	"gopkg.in/kataras/iris.v6/adaptors/httprouter"
)

var (
	// App is the server's Iris' instance.
	App *iris.Framework
)

func init() {
	App = iris.New()
	App.Adapt(iris.DevLogger())
	App.Adapt(httprouter.New())
}
