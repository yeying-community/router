package publisher

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

func GetProfile(c *gin.Context) {
	row, err := model.GetPublisherForUser(c.GetString(ctxkey.Id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": nil})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func UpdateProfile(c *gin.Context) {
	input := model.PublisherProfileInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.EnsurePublisherProfile(c.GetString(ctxkey.Id), input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func SubmitApplication(c *gin.Context) {
	row, err := model.SubmitPublisherApplication(c.GetString(ctxkey.Id))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ListServices(c *gin.Context) {
	rows, err := model.ListPublisherServices(c.GetString(ctxkey.Id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []model.PublisherService{}})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func GetService(c *gin.Context) {
	rows, err := model.ListPublisherServices(c.GetString(ctxkey.Id))
	if err != nil {
		respondError(c, err)
		return
	}
	for i := range rows {
		if rows[i].Id == strings.TrimSpace(c.Param("id")) {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows[i]})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": false, "message": "模型服务不存在或无权访问", "code": "publisher_service_not_found"})
}

func CreateService(c *gin.Context) {
	input := model.PublisherServiceInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.CreatePublisherService(c.GetString(ctxkey.Id), input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func CreateServiceFromPersonalConnection(c *gin.Context) {
	userID := c.GetString(ctxkey.Id)
	connection, err := model.GetPersonalProviderConnection(userID, c.Param("connection_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	input := model.PublisherServiceInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	input.SourcePersonalConnectionId = connection.Id
	if strings.TrimSpace(input.Name) == "" {
		input.Name = connection.Name
	}
	if strings.TrimSpace(input.Protocol) == "" {
		input.Protocol = connection.Protocol
	}
	if strings.TrimSpace(input.BaseURL) == "" {
		input.BaseURL = connection.BaseURL
	}
	row, err := model.CreatePublisherService(userID, input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func UpdateService(c *gin.Context) {
	input := model.PublisherServiceInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.UpdatePublisherService(c.GetString(ctxkey.Id), c.Param("id"), input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func DeleteService(c *gin.Context) {
	if err := model.DeletePublisherService(c.GetString(ctxkey.Id), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func VerifyService(c *gin.Context) {
	row, err := model.VerifyPublisherService(c.GetString(ctxkey.Id), c.Param("id"))
	if err != nil {
		if row != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": row})
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ListOffers(c *gin.Context) {
	rows, err := model.ListServiceOffers(c.GetString(ctxkey.Id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []model.ServiceOffer{}})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

// ListPublishedOffers is a consumer-facing read-only directory. Published
// offers are still not relay candidates in this release; the directory makes
// the supply and its declared data boundary discoverable before billing and
// routing are introduced.
func ListPublishedOffers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	rows, err := model.ListPublishedServiceOfferCatalog(c.Query("model"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func GetPublishedOffer(c *gin.Context) {
	row, err := model.GetPublishedServiceOfferCatalogItem(c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ListSettlements(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	rows, err := model.ListPublisherOfferSettlements(c.GetString(ctxkey.Id), limit)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []model.PublisherOfferSettlement{}})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func ListGroupAssignments(c *gin.Context) {
	rows, err := model.ListPublisherGroupAssignments(c.GetString(ctxkey.Id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []model.PublisherGroupAssignment{}})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func CreateOffer(c *gin.Context) {
	input := model.ServiceOfferInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.CreateServiceOffer(c.GetString(ctxkey.Id), input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func UpdateOffer(c *gin.Context) {
	input := model.ServiceOfferInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.UpdateServiceOffer(c.GetString(ctxkey.Id), c.Param("id"), input)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func DeleteOffer(c *gin.Context) {
	if err := model.DeleteServiceOffer(c.GetString(ctxkey.Id), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func SubmitOffer(c *gin.Context) {
	row, err := model.SubmitServiceOffer(c.GetString(ctxkey.Id), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ListPublishers(c *gin.Context) {
	rows, err := model.ListPublishersForAdmin(c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

type reviewInput struct {
	Note string `json:"note"`
}

func ApprovePublisher(c *gin.Context)  { reviewPublisher(c, true) }
func RestrictPublisher(c *gin.Context) { reviewPublisher(c, false) }

func reviewPublisher(c *gin.Context, approve bool) {
	input := reviewInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.ReviewPublisher(c.GetString(ctxkey.Id), c.Param("id"), approve, input.Note)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ListAdminServices(c *gin.Context) {
	rows, err := model.ListPublisherServicesForAdmin(c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func SuspendService(c *gin.Context) {
	input := reviewInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.SuspendPublisherService(c.GetString(ctxkey.Id), c.Param("id"), input.Note)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ListAdminOffers(c *gin.Context) {
	rows, err := model.ListServiceOffersForAdmin(c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func ListAdminSettlements(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	rows, err := model.ListPublisherOfferSettlementsForAdmin(c.Query("status"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func ListAdminSettlementDeliveries(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	rows, err := model.ListPublisherSettlementDeliveriesForAdmin(c.Query("status"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func RetryAdminSettlementDelivery(c *gin.Context) {
	requestLogID := strings.TrimSpace(c.Param("request_log_id"))
	if _, err := model.MarkPublisherSettlementDeliveryReady(requestLogID); err != nil {
		respondError(c, err)
		return
	}
	row, err := model.DeliverPublisherSettlement(requestLogID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

type cancelSettlementDeliveryInput struct {
	Reason string `json:"reason"`
}

func CancelAdminSettlementDelivery(c *gin.Context) {
	input := cancelSettlementDeliveryInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.CancelPublisherSettlementDelivery(c.Param("request_log_id"), c.GetString(ctxkey.Id), input.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

type resolveSettlementDeliveryExceptionInput struct {
	Resolution string `json:"resolution"`
}

func ResolveAdminSettlementDeliveryException(c *gin.Context) {
	input := resolveSettlementDeliveryExceptionInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.ResolvePublisherSettlementDeliveryException(c.Param("request_log_id"), c.GetString(ctxkey.Id), input.Resolution)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func ApproveOffer(c *gin.Context) { reviewOffer(c, true) }
func RejectOffer(c *gin.Context)  { reviewOffer(c, false) }

func reviewOffer(c *gin.Context, approve bool) {
	input := reviewInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row, err := model.ReviewServiceOffer(c.GetString(ctxkey.Id), c.Param("id"), approve, input.Note)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func respondError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "发布者资源不存在或无权访问", "code": "publisher_resource_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
}
