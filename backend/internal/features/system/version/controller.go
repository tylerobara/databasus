package system_version

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type VersionController struct{}

func (c *VersionController) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/system/version", c.GetVersion)
}

// GetVersion
// @Summary Get application version
// @Description Returns the current application version
// @Tags system/version
// @Produce json
// @Success 200 {object} VersionResponse
// @Router /system/version [get]
func (c *VersionController) GetVersion(ctx *gin.Context) {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "3.26.0"
	}

	ctx.JSON(http.StatusOK, VersionResponse{Version: version})
}
