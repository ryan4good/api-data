package router

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *OutboundHandler, sku *SkuHandler, inv *InventoryHandler) {
	r.POST("/api/sku/create", sku.CreateSku)
	r.POST("/api/inventory/add", inv.AddInventory)
	r.POST("/api/outbound/create", h.CreateOutbound)
	r.POST("/api/outbound/audit", h.AuditOutbound)
	r.POST("/api/outbound/allocate", h.AllocateInventory)
	r.POST("/api/outbound/ship", h.ShipOutbound)
	r.GET("/api/outbound/detail/:outboundNo", h.Detail)
	r.GET("/api/outbound/by-source-order/:sourceOrderNo", h.FindBySourceOrder)
}
