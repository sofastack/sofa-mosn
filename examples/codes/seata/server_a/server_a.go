package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Account struct {
	ID     int64 `json:"id"`
	Amount int64 `json:"Amount"`
}

type Inventory struct {
	ID  int64 `json:"id"`
	Qty int64 `json:"qty"`
}

func main() {
	r := gin.Default()

	r.POST("/service-a/begin", func(context *gin.Context) {
		xid := context.Request.Header.Get("x_seata_xid")
		account := &Account{
			ID: 1000024549,
			Amount: 200,
		}
		inv := &Inventory{
			ID: 1000000005,
			Qty: 2,
		}

		accountReq, err := json.Marshal(account)
		if err != nil {
			context.JSON(400, gin.H{ "success": false, "message": err.Error() })
			return
		}
		invReq, err := json.Marshal(inv)
		if err != nil {
			context.JSON(400, gin.H{ "success": false, "message": err.Error() })
			return
		}
		req1, err := http.NewRequest("POST", "http://localhost:2047/service-b/try", bytes.NewBuffer(accountReq))
		if err != nil {
			context.JSON(400, gin.H{ "success": false, "message": err.Error() })
			return
		}
		req1.Header.Set("x_seata_xid", xid)

		req2, err := http.NewRequest("POST", "http://localhost:2048/service-c/try", bytes.NewBuffer(invReq))
		if err != nil {
			context.JSON(400, gin.H{ "success": false, "message": err.Error() })
			return
		}
		req2.Header.Set("x_seata_xid", xid)

		client := &http.Client{}
		result1, err := client.Do(req1)
		if err != nil {
			context.JSON(400, gin.H{ "success": false, "message": err.Error() })
			return
		}
		if result1.StatusCode != http.StatusOK {
			result1.Write(context.Writer)
			return
		}

		result2, err := client.Do(req2)
		if err != nil {
			context.JSON(400, gin.H{ "success": false, "message": err.Error() })
			return
		}
		if result2.StatusCode == http.StatusOK {
			result2.Write(context.Writer)
			return
		}
	})

	r.Run(":8080")
}