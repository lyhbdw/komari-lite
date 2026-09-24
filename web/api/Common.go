package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Respond sends a standardized JSON response.
func Respond(c *gin.Context, httpStatus int, status string, message string, data interface{}) {
	c.JSON(httpStatus, Response{Status: status, Message: message, Data: data})
}

// RespondSuccess sends a success response with data.
func RespondSuccess(c *gin.Context, data interface{}) {
	Respond(c, http.StatusOK, "success", "", data)
}

// RespondError sends an error response with message.
func RespondError(c *gin.Context, httpStatus int, message string) {
	Respond(c, httpStatus, "error", message, nil)
}
