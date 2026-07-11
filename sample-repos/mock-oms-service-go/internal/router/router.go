package router

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *OrderHandler) {
	r.POST("/api/order/create", h.CreateOrder)
	r.POST("/api/order/mark-wms-pushed", h.MarkWmsPushed)
	r.GET("/api/order/detail/:orderNo", h.Detail)
}
