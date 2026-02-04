package api

import (
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/service/memo"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetProgress(c *gin.Context) {
	if memo.Total == 0 {
		c.JSON(http.StatusOK, model.ProgressResponse{
			Total:   0,
			Current: 0,
		})
		return
	}

	var currentMember model.Member
	if memo.LastID > 0 {
		if err := flow.DB.First(&currentMember, memo.LastID).Error; err == nil {
			c.JSON(http.StatusOK, model.ProgressResponse{
				Total:   memo.Total,
				Current: memo.LastID,
				Name:    currentMember.Name,
				Server:  currentMember.Server,
			})
			return
		}
	}

	c.JSON(http.StatusOK, model.ProgressResponse{
		Total:   memo.Total,
		Current: memo.LastID,
	})
}

func GetMemberProgress(c *gin.Context) {
	raw := c.Param("name")

	// parse name and server
	parts := strings.Split(raw, "@")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid member format, expect name@server"})
		return
	}
	name := strings.TrimSpace(parts[0])
	server := strings.TrimSpace(parts[1])

	// find member
	var member model.Member
	if err := flow.DB.Where("name = ? AND server = ?", name, server).
		First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "member not found"})
		return
	}

	c.JSON(http.StatusOK, model.MemberResponse{
		Name:         member.Name,
		Server:       member.Server,
		LastSyncTime: member.LogsSyncTime,
	})
}
