package option

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/i18n"
	"github.com/yeying-community/router/internal/admin/model"
	optionsvc "github.com/yeying-community/router/internal/admin/service/option"
)

// GetOptions godoc
// @Summary Get options (root)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/option [get]
func GetOptions(c *gin.Context) {
	options := optionsvc.GetOptions()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
}

// UpdateOption godoc
// @Summary Update option (root)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OptionUpdateRequest true "Option payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/option [put]
func UpdateOption(c *gin.Context) {
	var option model.Option
	err := json.NewDecoder(c.Request.Body).Decode(&option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	// No special validation for options here.
	err = optionsvc.UpdateOption(option.Key, option.Value)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}
