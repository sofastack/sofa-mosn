package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

type Inventory struct {
	ID  int64 `json:"id"`
	Qty int64 `json:"qty"`
}

func main() {
	r := gin.Default()

	r.POST("/service-c/try", func(context *gin.Context) {
		inv := &Inventory{}
		err := context.BindJSON(inv)
		if err == nil {
			context.JSON(200, gin.H{
				"success": true,
				"message": fmt.Sprintf("inventory %d tried!", inv.ID),
			})
			return
		}
		context.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})
	})

	r.POST("/service-c/confirm", func(context *gin.Context) {
		inv := &Inventory{}
		err := context.BindJSON(inv)
		if err == nil {
			context.JSON(200, gin.H{
				"success": true,
				"message": fmt.Sprintf("inventory %d tried!", inv.ID),
			})
			return
		}
		context.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})
	})

	r.POST("/service-c/cancel", func(context *gin.Context) {
		inv := &Inventory{}
		err := context.BindJSON(inv)
		if err == nil {
			context.JSON(200, gin.H{
				"success": true,
				"message": fmt.Sprintf("inventory %d tried!", inv.ID),
			})
			return
		}
		context.JSON(400, gin.H{
			"success": false,
			"message": err.Error(),
		})
	})

	r.Run(":8082")
}
