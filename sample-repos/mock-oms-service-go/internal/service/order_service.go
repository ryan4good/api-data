package service

func (s *OrderService) CreateOrder(req CreateOrderRequest) (*SalesOrder, error) {
	order := &SalesOrder{
		OrderNo:      NextOrderNo(),
		CustomerCode: req.CustomerCode,
		SkuCode:      req.SkuCode,
		Qty:          req.Qty,
		Status:       OrderStatusCreated,
	}
	return s.repo.Save(order)
}

func (s *OrderService) MarkWmsPushed(orderNo string, outboundNo string) (*SalesOrder, error) {
	order := s.repo.Get(orderNo)
	order.OutboundNo = outboundNo
	order.Status = OrderStatusWmsPushed
	return s.repo.Save(order)
}
