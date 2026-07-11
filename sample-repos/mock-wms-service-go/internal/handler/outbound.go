package handler

type OutboundHandler struct {
	service *OutboundService
}

func (h *OutboundHandler) CreateOutbound(req CreateOutboundRequest) (*OutboundOrder, error) {
	return h.service.CreateOutbound(req)
}

func (h *OutboundHandler) AuditOutbound(req AuditOutboundRequest) (*OutboundOrder, error) {
	return h.service.AuditOutbound(req.OutboundNo)
}

func (h *OutboundHandler) AllocateInventory(req AllocateInventoryRequest) (*OutboundOrder, error) {
	return h.service.AllocateInventory(req.OutboundNo)
}

func (h *OutboundHandler) ShipOutbound(req ShipOutboundRequest) (*OutboundOrder, error) {
	return h.service.ShipOutbound(req.OutboundNo)
}

func (h *OutboundHandler) Detail(outboundNo string) (*OutboundOrder, error) {
	return h.service.Detail(outboundNo)
}
