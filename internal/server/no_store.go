package server

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func noStoreAPIResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}
