package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

type Account struct {
	ID     int64 `json:"id"`
	Amount int64 `json:"Amount"`
}

func main() {
	r := gin.Default()

	r.POST("/service-b/try", func(context *gin.Context) {
		account := &Account{}
		err := context.BindJSON(account)
		if err == nil {
			context.JSON(200, gin.H{
				"success": true,
				"message": fmt.Sprintf("account %d tried!", account.ID),
			})
			return
		}
		context.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})
	})

	r.POST("/service-b/confirm", func(context *gin.Context) {
		account := &Account{}
		err := context.BindJSON(account)
		if err == nil {
			context.JSON(200, gin.H{
				"success": true,
				"message": fmt.Sprintf("account %d confirmed!", account.ID),
			})
			return
		}
		context.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})
	})

	r.POST("/service-b/cancel", func(context *gin.Context) {
		account := &Account{}
		err := context.BindJSON(account)
		if err == nil {
			context.JSON(200, gin.H{
				"success": true,
				"message": fmt.Sprintf("account %d canceled!", account.ID),
			})
			return
		}
		context.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})
	})

	r.Run(":8081")
}