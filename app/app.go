package app

import (
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	app "github.com/nightexcessive/sc-intelligence"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

func init() {
	router := httprouter.New()
	router.PanicHandler = panicHandler

	app.RegisterHandlers(router)

	http.Handle("/", router)
}

var isDev = appengine.IsDevAppServer()

func panicHandler(w http.ResponseWriter, req *http.Request, err interface{}) {
	c := appengine.NewContext(req)

	stackTrace := buildStack(4)
	log.Criticalf(c, "%s\n\n%s", err, stackTrace)

	w.WriteHeader(500)
	if !isDev {
		io.WriteString(w, "An internal error occurred. It has been logged.")
	} else {
		fmt.Fprintf(w, "caught a panic: (type: %T)\n\n%s\n\n%s", err, err, stackTrace)
	}
}
