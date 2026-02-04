package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type StatusResponse struct {
	Status   string `json:"status"`
	ClientIP string `json:"client_ip"`
}

// Status godoc
// @Summary Get the status of the server
// @Description Get the current status of the server
// @Tags status
// @Success 200 {object} StatusResponse
// @Router /status [get]
func Status(c *gin.Context) {
	c.JSON(http.StatusOK, StatusResponse{
		Status:   "OK",
		ClientIP: c.ClientIP(),
	})
}
