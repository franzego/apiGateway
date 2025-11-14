package middleware

import (
	"net/http"
	"strings"

	"github.com/franzego/apigateway/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		c.Set("correlationID", correlationID)
		c.Header("X-Correlation-ID", correlationID)
		c.Next()
	}
}

func AuthenticationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusBadRequest, models.ApiResponse{
				Success: false,
				Message: "Unauthorized",
				Error:   "Authorization header required",
			})
			c.Abort()
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, models.ApiResponse{
				Success: false,
				Message: "Unauthorized",
				Error:   "Invalid Authorization Header",
			})
		}
		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Get secret from config
			return []byte("your-secret-key"), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, models.ApiResponse{
				Success: false,
				Message: "Unauthorized",
				Error:   "Invalid Token",
			})
			c.Abort()
			return
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["user_id"])
		}
		c.Next()
	}
}
