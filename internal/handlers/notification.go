package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/franzego/apigateway/internal/models"
	"github.com/franzego/apigateway/internal/queue"
	"github.com/franzego/apigateway/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Notification struct {
	RabbitMq        queue.Queuer
	TemplateService services.TemplateServicer
	UserService     services.UserServicer
	Redis           *redis.Client
}

func NewNotificationService(
	rabbitmq queue.Queuer,
	templateService services.TemplateServicer,
	userService services.UserServicer,
	redis *redis.Client,
) *Notification {
	return &Notification{
		RabbitMq:        rabbitmq,
		TemplateService: templateService,
		UserService:     userService,
		Redis:           redis,
	}
}

// The Post Email Notification Endpoint
func (no *Notification) SendEmail(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	now := time.Now()
	defer cancel()
	correlation, _ := c.Get("correlationID")
	correlationId := correlation.(string)

	var req models.SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "Bad Request",
			Error:   "Invalid request",
		})
		return
	}
	exist, err := no.CheckIdempotency(ctx, correlationId)
	if err != nil {
		log.Printf("idempotecy check has failed: %v", err)
	}
	if exist {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "Request has been gotten",
			Error:   "Notification is being processed",
		})
		return
	}
	notificationID := uuid.New().String()
	validUser, err := no.UserService.ValidateUser(ctx, req.UserID)
	if err != nil || !validUser {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "User not found or unavailable",
			Error:   err.Error(),
		})
		return
	}
	validTemp, err := no.TemplateService.ValidateTemplate(ctx, req.TemplateID)
	if err != nil || !validTemp {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "Template not found or unavailable",
			Error:   err.Error(),
		})
		return
	}
	message := models.NotificationMessage{
		ID:           notificationID,
		Type:         "email",
		UserID:       req.UserID,
		TemplateID:   req.TemplateID,
		ScheduledFor: &now,
		Timestamp:    time.Now(),
	}
	if err = no.RabbitMq.PublishEmail(message); err != nil {
		log.Printf("failed to qqueue to rabbitmq")
		c.JSON(http.StatusInternalServerError, models.ApiResponse{
			Success: false,
			Message: "Failed to publish to queue",
			Error:   err.Error(),
		})
		return
	}
	if err := no.storeNotificationStatus(ctx, notificationID, "email", "processing"); err != nil {
		log.Printf("error in storing notification status")
		c.JSON(http.StatusInternalServerError, models.ApiResponse{
			Success: false,
			Message: "failed to store the status",
			Error:   "internal server error",
		})
		return
	}
	// finalMessage := models.NotificationResponse{
	// 	NotificationID: notificationID,
	// 	Status:         "processing",
	// 	QueuedAt:       time.Now(),
	// }
	c.JSON(http.StatusOK, models.ApiResponse{
		Success: true,
		Message: "Email sent successfully",
		Data: models.NotificationResponse{
			NotificationID: notificationID,
			Status:         "processing",
			QueuedAt:       time.Now(),
		},
	})

}

// The Post Push Notification Endpoint
func (no *Notification) SendPush(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, exists := c.Get("correlationID")
	now := time.Now()
	if !exists {
		log.Print("Correlation id not found")
		return
	}
	correlation_Id := id.(string)
	defer cancel()
	var req models.SendPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "Bad Request",
			Error:   "Invalid request",
		})
		return
	}
	isDuplicate, err := no.CheckIdempotency(ctx, correlation_Id)
	if err != nil {
		log.Print("An error occurred while checking for idempotency")
	}
	if isDuplicate {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "Notification is already being processed",
			Error:   "Request already being processed",
		})
		return
	}
	notificationId := uuid.New().String()
	isExist, err := no.UserService.ValidateUser(c.Request.Context(), req.UserID)
	if err != nil || !isExist {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "User not found or unavailable",
			Error:   err.Error(),
		})
		return
	}
	isTemplate, err := no.TemplateService.ValidateTemplate(c.Request.Context(), req.TemplateID)
	if err != nil || !isTemplate {
		c.JSON(http.StatusBadRequest, models.ApiResponse{
			Success: false,
			Message: "Template not found or unavailable",
			Error:   err.Error(),
		})
		return
	}
	message := models.NotificationMessage{
		ID:           notificationId,
		Type:         "push",
		UserID:       req.UserID,
		TemplateID:   req.TemplateID,
		ScheduledFor: &now,
		Timestamp:    time.Now(),
	}
	if err := no.RabbitMq.PublishPush(message); err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiResponse{
			Success: false,
			Message: "Problem queing to RabbitMQ",
			Error:   err.Error(),
		})
		return
	}
	if err := no.storeNotificationStatus(ctx, notificationId, "processing", "push"); err != nil {
		c.JSON(http.StatusInternalServerError, models.ApiResponse{
			Success: false,
			Message: "Couldn't store notification status",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, models.ApiResponse{
		Success: true,
		Message: "Notification sent successfully",
		Data: models.NotificationResponse{
			NotificationID: notificationId,
			Status:         "Processing",
			QueuedAt:       time.Now(),
		},
	})
}

// The correlation id gottenfrom the middleware is used to cehck for idempotency
func (no *Notification) CheckIdempotency(ctx context.Context, correlationId string) (bool, error) {
	exists, err := no.Redis.Exists(ctx, fmt.Sprintf("correlationid: %s", correlationId)).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	}
	err = no.Redis.Set(ctx, fmt.Sprintf("correlationid: %s", correlationId), "processing", 5*time.Minute).Err()
	return false, err
}

func (no *Notification) storeNotificationStatus(ctx context.Context, notificationID, currentStatus, notificationType string) error {
	status := &models.NotificationStatus{
		ID:        notificationID,
		Type:      notificationType,
		Status:    currentStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// we need to set it in redis.
	body, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("there was an error marshalling")
	}
	key := fmt.Sprintf("notification:%s", notificationID)
	return no.Redis.Set(ctx, key, body, 5*time.Second).Err()
}
