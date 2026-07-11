package model

const (
	OrderStatusCreated   = "ORDER_CREATED"
	OrderStatusWmsPushed = "WMS_PUSHED"
)

type SalesOrder struct {
	OrderNo      string
	CustomerCode string
	SkuCode      string
	Qty          int
	Status       string
	OutboundNo   string
}

type CreateOrderRequest struct {
	CustomerCode string
	SkuCode      string
	Qty          int
}

type MarkWmsPushedRequest struct {
	OrderNo    string
	OutboundNo string
}
