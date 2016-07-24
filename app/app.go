package app

import (
	"fmt"
	"io"
	"net/http"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"github.com/julienschmidt/httprouter"

	app "github.com/nightexcessive/sc-intelligence"
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

	log.Criticalf(c, "caught panic: (%T) %s", err, err)

	w.WriteHeader(500)
	if !isDev {
		io.WriteString(w, "An internal error occurred. It has been logged.")
	} else {
		fmt.Fprintf(w, "caught a panic: (type: %T)\n\n%s", err, err)
	}
}
