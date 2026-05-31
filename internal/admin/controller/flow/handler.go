package flow

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	usersvc "github.com/yeying-community/router/internal/admin/service/user"
)

const maxFlowPageSize = 100

type flowListData[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func parseFlowPageParams(c *gin.Context) (page int, pageSize int, keyword string, status string) {
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = config.ItemsPerPage
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxFlowPageSize {
		pageSize = maxFlowPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	status = strings.TrimSpace(c.Query("status"))
	return page, pageSize, keyword, status
}

func writeFlowList[T any](c *gin.Context, rows []T, total int64, page int, pageSize int) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": flowListData[T]{
			Items:    rows,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func writeFlowError(c *gin.Context, err error) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": err.Error(),
	})
}

func GetTopupOrderRecords(c *gin.Context) {
	page, pageSize, keyword, status := parseFlowPageParams(c)
	rows, total, err := model.ListAdminTopupOrderRecordsPageWithDB(model.DB, page, pageSize, keyword, status)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	writeFlowList(c, rows, total, page, pageSize)
}

func GetTopupOrderRecord(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	row, err := model.GetAdminTopupOrderRecordByIDWithDB(model.DB, id)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    row,
	})
}

func GetTopupReconcileRecords(c *gin.Context) {
	page, pageSize, keyword, status := parseFlowPageParams(c)
	rows, total, err := model.ListAdminTopupReconcileRecordsPageWithDB(model.DB, page, pageSize, keyword, status)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	writeFlowList(c, rows, total, page, pageSize)
}

func GetTopupReconcileRecord(c *gin.Context) {
	orderID := strings.TrimSpace(c.Param("id"))
	order, err := model.GetTopupOrderByIDForAdminWithDB(model.DB, orderID)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	if strings.TrimSpace(order.Source) != model.TopupOrderSourceTopUpAPI {
		writeFlowError(c, fmt.Errorf("该订单不属于支付记录"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    order,
	})
}

func RefreshTopupReconcileRecord(c *gin.Context) {
	orderID := strings.TrimSpace(c.Param("id"))
	order, err := model.GetTopupOrderByIDForAdminWithDB(model.DB, orderID)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	refreshedOrder, err := model.RefreshTopupOrderStatusWithDB(model.DB, order.Id, order.UserID)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    refreshedOrder,
	})
}

func FulfillTopupReconcileRecord(c *gin.Context) {
	orderID := strings.TrimSpace(c.Param("id"))
	order, err := model.GetTopupOrderByIDForAdminWithDB(model.DB, orderID)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	if strings.TrimSpace(order.Source) != model.TopupOrderSourceTopUpAPI {
		writeFlowError(c, fmt.Errorf("该订单不属于支付记录"))
		return
	}
	fulfilledOrder, fulfilledNow, err := model.FulfillTopupOrderWithDB(model.DB, order.Id)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	if fulfilledNow && fulfilledOrder.BusinessType == model.TopupOrderBusinessBalance && fulfilledOrder.Quota > 0 {
		usersvc.RecordTopupLog(
			c.Request.Context(),
			fulfilledOrder.UserID,
			"管理员补偿外部支付充值 "+strconv.FormatInt(fulfilledOrder.Quota, 10)+" 额度",
			int(fulfilledOrder.Quota),
		)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    fulfilledOrder,
	})
}

func GetPackageRecords(c *gin.Context) {
	page, pageSize, keyword, status := parseFlowPageParams(c)
	statusCode := 0
	if status != "" {
		if parsed, err := strconv.Atoi(status); err == nil && parsed > 0 {
			statusCode = parsed
		}
	}
	rows, total, err := model.ListAdminUserPackageRecordsPageWithDB(model.DB, page, pageSize, keyword, statusCode)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	writeFlowList(c, rows, total, page, pageSize)
}

func GetPackageRecord(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	row, err := model.GetAdminUserPackageRecordByIDWithDB(model.DB, id)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    row,
	})
}

func GetRedemptionRecords(c *gin.Context) {
	page, pageSize, keyword, _ := parseFlowPageParams(c)
	rows, total, err := model.ListAdminRedemptionRecordsPageWithDB(model.DB, page, pageSize, keyword)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	writeFlowList(c, rows, total, page, pageSize)
}

func GetRedemptionRecord(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	row, err := model.GetAdminRedemptionRecordByIDWithDB(model.DB, id)
	if err != nil {
		writeFlowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    row,
	})
}
