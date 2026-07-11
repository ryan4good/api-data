package model

const (
	OutboundStatusCreated   = "CREATED"
	OutboundStatusAudited   = "AUDITED"
	OutboundStatusAllocated = "ALLOCATED"
	OutboundStatusShipped   = "SHIPPED"
)

type OutboundOrder struct {
	OutboundNo    string
	SourceOrderNo string
	WarehouseCode string
	OwnerCode     string
	SkuCode       string
	Qty           int
	Status        string
}

type CreateOutboundRequest struct {
	SourceOrderNo string
	WarehouseCode string
	OwnerCode     string
	SkuCode       string
	Qty           int
}

type AuditOutboundRequest struct {
	OutboundNo string
}

type AllocateInventoryRequest struct {
	OutboundNo string
}

type ShipOutboundRequest struct {
	OutboundNo string
}
