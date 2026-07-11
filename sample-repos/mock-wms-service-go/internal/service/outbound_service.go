package service

func (s *OutboundService) CreateOutbound(req CreateOutboundRequest) (*OutboundOrder, error) {
	order := &OutboundOrder{
		OutboundNo:    NextOutboundNo(),
		SourceOrderNo: req.SourceOrderNo,
		WarehouseCode: req.WarehouseCode,
		OwnerCode:     req.OwnerCode,
		SkuCode:       req.SkuCode,
		Qty:           req.Qty,
		Status:        OutboundStatusCreated,
	}
	return s.repo.Save(order)
}

func (s *OutboundService) AuditOutbound(outboundNo string) (*OutboundOrder, error) {
	order := s.repo.Get(outboundNo)
	order.Status = OutboundStatusAudited
	return s.repo.Save(order)
}

func (s *OutboundService) AllocateInventory(outboundNo string) (*OutboundOrder, error) {
	order := s.repo.Get(outboundNo)
	s.inventoryService.LockStock(order.WarehouseCode, order.SkuCode, order.Qty)
	order.Status = OutboundStatusAllocated
	return s.repo.Save(order)
}

func (s *OutboundService) ShipOutbound(outboundNo string) (*OutboundOrder, error) {
	order := s.repo.Get(outboundNo)
	order.Status = OutboundStatusShipped
	return s.repo.Save(order)
}
