package scintelligence

import (
	"github.com/julienschmidt/httprouter"

	"github.com/nightexcessive/sc-intelligence/crawl"
)

func RegisterHandlers(router *httprouter.Router) {
	crawl.RegisterHandlers(router)
}
