package controller

import (
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

var sharedModelCodePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,119}$`)

func SharedModelsCatalog(c *gin.Context) {
	var models []model.SharedModel
	q := model.DB.Order("id")
	if !strings.HasPrefix(c.FullPath(), "/api/shared/admin/") {
		q = q.Where("enabled = ?", true)
	}
	if err := q.Find(&models).Error; err != nil {
		shareError(c, 500, "model catalog unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": models})
}

func SharedAdminCreateModel(c *gin.Context) {
	var data model.SharedModel
	if err := c.ShouldBindJSON(&data); err != nil {
		shareError(c, 400, "invalid model")
		return
	}
	if !sharedModelCodePattern.MatchString(data.Code) || len(strings.TrimSpace(data.Name)) == 0 || len(data.Name) > 120 ||
		(data.Protocol != "chat" && data.Protocol != "responses" && data.Protocol != "anthropic" && data.Protocol != "gemini") ||
		data.MaxOutput < 16 || data.MaxOutput > 131072 || !sharedValidPrice(data.InputPrice, data.OutputPrice) {
		shareError(c, 400, "invalid model ID, protocol, output limit or price")
		return
	}
	data.ID = 0
	data.Enabled = true
	if err := model.DB.Create(&data).Error; err != nil {
		shareError(c, 409, "model already exists")
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}
func sharedValidPrice(input, output float64) bool {
	return !math.IsNaN(input) && !math.IsNaN(output) && !math.IsInf(input, 0) && !math.IsInf(output, 0) &&
		input >= 0 && output >= 0 && input <= 10000 && output <= 10000
}
func SharedAdminUpdateModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shareError(c, 400, "invalid ID")
		return
	}
	var patch struct {
		Name        string  `json:"name"`
		Enabled     *bool   `json:"enabled"`
		InputPrice  float64 `json:"input_price"`
		OutputPrice float64 `json:"output_price"`
		MaxOutput   int     `json:"max_output"`
	}
	if err := c.ShouldBindJSON(&patch); err != nil || patch.Enabled == nil || len(strings.TrimSpace(patch.Name)) == 0 ||
		len(patch.Name) > 120 || !sharedValidPrice(patch.InputPrice, patch.OutputPrice) || patch.MaxOutput < 16 || patch.MaxOutput > 131072 {
		shareError(c, 400, "invalid model settings")
		return
	}
	// Active contributions are not altered silently when a model is disabled.
	if !*patch.Enabled {
		var count int64
		var links []model.SharedProviderModel
		model.DB.Where("model_id = ? AND enabled = ?", id, true).Find(&links)
		for _, link := range links {
			var n int64
			model.DB.Model(&model.SharedContribution{}).Where("provider_id = ? AND enabled = ?", link.ProviderID, true).Count(&n)
			count += n
		}
		if count > 0 {
			shareError(c, 409, "pause affected contributions before disabling model")
			return
		}
	}
	values := map[string]any{"name": patch.Name, "enabled": *patch.Enabled, "input_price": patch.InputPrice,
		"output_price": patch.OutputPrice, "max_output": patch.MaxOutput}
	result := model.DB.Model(&model.SharedModel{}).Where("id = ?", id).Updates(values)
	if result.Error != nil || result.RowsAffected != 1 {
		shareError(c, 404, "model not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}
func SharedAdminBindModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shareError(c, 400, "invalid provider")
		return
	}
	var req struct {
		ModelID uint `json:"model_id"`
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ModelID == 0 {
		shareError(c, 400, "invalid model binding")
		return
	}
	if err := model.SharedBindProviderModel(uint(id), req.ModelID, req.Enabled); err != nil {
		shareError(c, 409, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}
