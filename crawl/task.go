package crawl

import "github.com/gin-gonic/gin"

// RegisterHandlers registers all handles for the crawl module
func RegisterHandlers(router *gin.Engine) {
	router.POST("/task/crawl/citizen", crawlCitizen)
	router.POST("/task/crawl/org", crawlOrg)
}
