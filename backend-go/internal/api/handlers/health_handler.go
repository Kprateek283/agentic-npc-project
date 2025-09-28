package handlers

import "github.com/gin-gonic/gin"

// HealthHandler handles the health check endpoint.
func HealthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "UP",
	})
}
