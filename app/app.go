package app

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	app "github.com/nightexcessive/sc-intelligence"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

func init() {
	router := gin.New()
	router.Use(panicHandler)

	app.RegisterHandlers(router)

	http.Handle("/", router)
}

var isDev = appengine.IsDevAppServer()

func panicHandler(ginContext *gin.Context) {
	defer func() {
		panicErr := recover()
		if panicErr == nil {
			return
		}

		c := appengine.NewContext(ginContext.Request)

		stackTrace := buildStack(2)
		log.Criticalf(c, "%s\n\n%s", panicErr, stackTrace)

		ginContext.AbortWithStatus(500)
		if !isDev {
			io.WriteString(ginContext.Writer, "An internal error occurred. It has been logged.")
		} else {
			fmt.Fprintf(ginContext.Writer, "caught a panic: (type: %T)\n\n%s\n\n%s", panicErr, panicErr, stackTrace)
		}
	}()

	ginContext.Next()
}
