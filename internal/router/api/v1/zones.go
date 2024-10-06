package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mgp-systems/internal-fabrik8-api/internal/types"
	"github.com/mgp-systems/internal-fabrik8-api/pkg/google"
)

// Currently only needs to support google
func ListZonesForRegion(c *gin.Context) {
	var zonesListRequest types.ZonesListRequest
	err := c.Bind(&zonesListRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.JSONFailureResponse{
			Message: err.Error(),
		})
		return
	}

	googleConf := google.Configuration{
		Context: context.Background(),
		Project: zonesListRequest.GoogleAuth.ProjectID,
		Region:  zonesListRequest.CloudRegion,
		KeyFile: zonesListRequest.GoogleAuth.KeyFile,
	}

	var zonesListResponse types.ZonesListResponse

	zones, err := googleConf.GetZones()
	if err != nil {
		c.JSON(http.StatusBadRequest, types.JSONFailureResponse{
			Message: err.Error(),
		})
		return
	}

	zonesListResponse.Zones = zones

	c.JSON(http.StatusOK, zonesListResponse)
}
