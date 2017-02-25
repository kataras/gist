package main

import (
	"gopkg.in/kataras/iris.v6"
	"gopkg.in/kataras/iris.v6/adaptors/httprouter"
	"gopkg.in/kataras/iris.v6/adaptors/view"
	"gopkg.in/kataras/iris.v6/adaptors/websocket"
)

var (
	app *iris.Framework
	ws  websocket.Server
)

func init() {
	// init the server instance
	app = iris.New()
	// adapt a logger in dev mode
	app.Adapt(iris.DevLogger())
	// adapt router
	app.Adapt(httprouter.New())
	// adapt templaes
	app.Adapt(view.HTML("./templates", ".html").Reload(true))
	// adapt websocket
	ws = websocket.New(websocket.Config{Endpoint: "/gist-realtime"})
	ws.OnConnection(HandleWebsocketConnection)
	app.Adapt(ws)
}

func main() {
	app.StaticWeb("/css", "./assets/css")
	rootRepo := "https://github.com/iris-contrib/examples"

	h := func(ctx *iris.Context) {
		examplePath := ctx.Param("example")
		if err := WriteGistTo(rootRepo, examplePath, ctx); err != nil {
			ctx.EmitError(iris.StatusInternalServerError)
			app.Log(iris.DevMode, err.Error())
			return
		}
		ctx.SetContentType("text/html; charset=" + app.Config.Charset)
	}

	// http://localhost:8080/example/subdomains_1/main.go
	app.Get("/example/*example", h) //app.Cache(h, 6*time.Hour))
	app.Listen(":8080")
}
