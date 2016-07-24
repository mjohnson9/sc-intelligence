package crawl

import "github.com/julienschmidt/httprouter"

// RegisterHandlers registers all handles for the crawl module
func RegisterHandlers(router *httprouter.Router) {
	router.POST("/task/crawl/citizen", crawlCitizen)
	router.POST("/task/crawl/org", crawlOrg)
}
