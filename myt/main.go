package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sourcegraph/syntaxhighlight"
	"gopkg.in/kataras/iris.v6"
	"gopkg.in/kataras/iris.v6/adaptors/httprouter"
	"gopkg.in/kataras/iris.v6/adaptors/view"
	"gopkg.in/kataras/iris.v6/adaptors/websocket"
)

type timeline struct {
	Years  int
	Months int
	Days   int
}

type author struct {
	Username  string
	AvatarURI string
}

type gist struct {
	LastUpdate     timeline      // top-left body
	LastUpdateDate time.Time     // used internally
	Author         author        // bottom-right , after the footer
	Content        template.HTML // body
	RunTutorial    template.HTML
	Description    string // header
	Notes          string // footer
	Tree           template.HTML
	Source         string
	Chapter        string
}

// diff returns the number of years, months, and days between t1 and t2, inclusive.
func diff(t1, t2 time.Time) (years, months, days int) {
	t2 = t2.AddDate(0, 0, 1) // advance t2 to make the range inclusive

	for t1.AddDate(years, 0, 0).Before(t2) {
		years++
	}
	years--

	for t1.AddDate(years, months, 0).Before(t2) {
		months++
	}
	months--

	for t1.AddDate(years, months, days).Before(t2) {
		days++
	}
	days--

	return
}

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
	ws.OnConnection(handleWebsocket)
	app.Adapt(ws)
}

type (
	entry struct {
		key   []byte
		value []byte
	}
	memcache []entry
)

func (r *memcache) Set(key string, value []byte) {
	args := *r
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if string(kv.key) == key {
			kv.value = value
			return
		}
	}

	c := cap(args)
	if c > n {
		args = args[:n+1]
		kv := &args[n]
		kv.key = append(kv.key[:0], key...)
		kv.value = value
		*r = args
		return
	}

	kv := entry{}
	kv.key = append(kv.key[:0], key...)
	kv.value = value
	*r = append(args, kv)
}

func (r *memcache) Get(key string) []byte {
	args := *r
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if string(kv.key) == key {
			return kv.value
		}
	}
	return nil
}

func (r *memcache) Reset() {
	*r = (*r)[:0]
}

func main() {
	app.StaticWeb("/css", "./assets/css")
	var cache memcache // lock-free
	rootRepo := "https://github.com/iris-contrib/examples"

	h := func(ctx *iris.Context) {
		relPath := ctx.Param("example")
		if !strings.Contains(relPath, ".") {
			// if main.go missing append it
			relPath += "/main.go"
			relPath = strings.Replace(relPath, "//", "/", -1)
		}

		source := rootRepo + "/blob/master/" + relPath

		if body := cache.Get(source); len(body) > 0 {
			ctx.Write(body)
			println("body found")
			return
		}

		repl := map[string]string{
			"https://github.com": "https://raw.githubusercontent.com",
			"/blob":              "",
		}
		raw := source
		for k, v := range repl {
			raw = strings.Replace(raw, k, v, 1)
		}

		mainFile := source[strings.LastIndex(source, "/")+1:]

		doc, err := goquery.NewDocument(source)
		if err != nil {
			ctx.SetStatusCode(iris.StatusBadRequest)
			ctx.Writef(err.Error())
			return
		}

		g := gist{}
		lastUpdateElem := doc.Find("div.commit-tease .float-right relative-time")
		if lastCommitDate, found := lastUpdateElem.Attr("datetime"); found {
			if g.LastUpdateDate, err = time.Parse(time.RFC3339, lastCommitDate); err != nil {
				ctx.SetStatusCode(iris.StatusBadRequest)
				ctx.Writef(err.Error())
				return
			}
			// ft := time.Now().Format(time.RFC3339)
			// tm, _ := time.Parse(time.RFC3339, ft)
			years, months, days := diff(g.LastUpdateDate, time.Now())

			tl := timeline{Years: years, Months: months, Days: days}
			g.LastUpdate = tl
		}
		rawResource, err := http.DefaultClient.Get(raw)
		if err != nil {
			ctx.SetStatusCode(iris.StatusBadRequest)
			ctx.Writef(err.Error())
			return
		}

		body, err := ioutil.ReadAll(rawResource.Body)
		if err != nil {
			ctx.SetStatusCode(iris.StatusBadRequest)
			ctx.Writef(err.Error())
			return
		}

		findDescription := func(start string) string {
			pckMainDescStart := []byte(start)
			pckMainDescEnd := []byte("package main")
			// find the description of the form // Package .... does this and that
			// We will assume that the package has the name of 'main', in order to be runnable go needs that, so we assume that.
			if bytes.Contains(body, pckMainDescStart) {
				// take the content after the // Pakcage_$main_ until the lowercase package $main
				descriptionB := body[bytes.Index(body, pckMainDescStart):bytes.LastIndex(body, pckMainDescEnd)]
				body = bytes.Replace(body, descriptionB, []byte(""), 1)
				description := string(descriptionB[len(pckMainDescStart):])
				firstChar := string(description[0])
				description = strings.ToUpper(firstChar) + description[1:] // uppercase the first letter

				return description
			}
			return ""
		}

		g.Description = findDescription("// Package main ")
		if g.Description == "" {
			// try with lowercase
			g.Description = findDescription("// package main ")
			// if not found and here just skip the description
		}

		g.Author = author{}
		authorElem := doc.Find("img.avatar")

		if authorUsername, found := authorElem.Attr("alt"); found {
			g.Author.Username = authorUsername
		}
		if authorAvatarURI, found := authorElem.Attr("src"); found {
			g.Author.AvatarURI = authorAvatarURI
		}

		replacements := map[string]string{
			"https://": "",
			"http://":  "",
			"blob/":    "",
			"tree/":    "",
			"master/":  "",
			"v6/":      "",
			mainFile:   "",
		}
		gopathVirtual := source
		for k, v := range replacements {
			gopathVirtual = strings.Replace(gopathVirtual, k, v, 1)
		}
		gopathVirtual = "$GOPATH/src/" + gopathVirtual
		goRunVirtual := "$ go run " + mainFile

		parentSource := strings.Replace(source, mainFile, "", 1)

		parentDoc, err := goquery.NewDocument(parentSource)
		if err != nil {
			ctx.SetStatusCode(iris.StatusBadRequest)
			ctx.Writef(err.Error())
			return
		}
		tree := make([]string, 0)

		parentDoc.Find("table.js-navigation-container tr.js-navigation-item").Each(func(i int, s *goquery.Selection) {
			name := s.Find(".content a").Text()
			if name != "" {
				if name == "README.md" {
					return // break here and continue to the next element
				}

				if strings.Contains(name, "/") {
					// dir inside, split it
					/// TODO: do multiple splits ofc... here we are just testing things
					tree = append(tree, name[0:strings.Index(name, "/")]+"<br/>&nbsp;&nbsp;  └── "+name[strings.Index(name, "/")+1:])
				} else {
					resourceSource := strings.Replace(source, mainFile, name, 1)
					tree = append(tree, "<a target='_blank' href='"+resourceSource+"'>"+name+"</a>")
				}
			}

		})

		// first the files
		sort.Slice(tree, func(i int, j int) bool {
			return !strings.Contains(tree[i], "/")
		})

		treeVisual := "$ tree<br/>"
		for _, t := range tree {
			treeVisual += "> " + t + "<br/>"
		}

		g.RunTutorial = template.HTML("$ cd " + gopathVirtual + "<br/>" + treeVisual + goRunVirtual)

		// withOnlineViews := append([]byte("// "+strconv.Itoa(onlineViews)+" online views\n"), body...)
		withLastEdit := append([]byte("// edited "+strconv.Itoa(g.LastUpdate.Days)+" days ago\n"), body...)
		withAuthor := append([]byte("// author "+g.Author.Username+"\n"), withLastEdit...)
		withFile := append([]byte("// file "+mainFile+"\n"), withAuthor...)
		// withRunTutorial := append(withFile, []byte(runTutorialPrefix+"\n"+string(g.RunTutorial)+"\n//")...)
		// file main.go
		// author @kataras
		// edited 2 days ago
		h, err := syntaxhighlight.AsHTML(withFile)
		if err != nil {
			ctx.SetStatusCode(iris.StatusBadRequest)
			ctx.Writef(err.Error())
			return
		}
		g.Content = template.HTML(string(h))
		g.Source = source

		chapterName := source[0:strings.LastIndex(source, "/")]
		chapterName = chapterName[strings.LastIndex(chapterName, "/")+1:]
		chapterName = strings.Replace(chapterName, "_", " ", -1)
		chapterName = strings.Replace(chapterName, "-", " ", -1)
		chapterName = strings.ToUpper(string(chapterName[0])) + chapterName[1:] // uppercase the first letter
		g.Chapter = chapterName

		buff := &bytes.Buffer{}
		// use the app's render instead of context in order to write the result on another writer(bytes.Buffer).
		app.Render(buff, "gist.html", g)
		// set the cache
		cache.Set(source, buff.Bytes())
		// send the body to the client
		ctx.Write(buff.Bytes())
	}

	// http://localhost:8080/example/subdomains_1/main.go
	app.Get("/example/*example", h) //app.Cache(h, 6*time.Hour))
	app.Listen(":8080")
}

var onlineViews = 0

func handleWebsocket(c websocket.Connection) {

	var sources []string

	c.On("watch", func(pageSource string) {
		sources = append(sources, pageSource)
		// join the socket to a room linked with the page source
		c.Join(pageSource)
		onlineViews++
		c.To(pageSource).Emit("watch", onlineViews)
	})

	c.OnDisconnect(func() {
		onlineViews--
		for _, source := range sources {
			for _, conn := range ws.GetConnectionsByRoom(source) {
				conn.Emit("watch", onlineViews)
			}
		}

	})

}
