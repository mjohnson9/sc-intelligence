package scintelligence

import (
	"github.com/gin-gonic/gin"
	"github.com/nightexcessive/sc-intelligence/crawl"
)

func RegisterHandlers(router *gin.Engine) {
	crawl.RegisterHandlers(router)
}
