package handler

type OrderHandler struct {
	service *OrderService
}

func (h *OrderHandler) CreateOrder(req CreateOrderRequest) (*SalesOrder, error) {
	return h.service.CreateOrder(req)
}

func (h *OrderHandler) MarkWmsPushed(req MarkWmsPushedRequest) (*SalesOrder, error) {
	return h.service.MarkWmsPushed(req.OrderNo, req.OutboundNo)
}

func (h *OrderHandler) Detail(orderNo string) (*SalesOrder, error) {
	return h.service.Detail(orderNo)
}
