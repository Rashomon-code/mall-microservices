package server

import (
	"context"
	"log"
	productpb "mall/proto"
	"time"

	"gorm.io/gorm"
)

type OrderModel struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	ProductID int64     `gorm:"column:product_id"`
	Quantity  int32     `gorm:"column:quantity"`
	Status    string    `gorm:"size:50;column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (OrderModel) TableName() string {
	return "orders"
}

type OrderServer struct {
	DB            *gorm.DB
	ProductClient productpb.ProductServiceClient
}

func NewOrderServer(db *gorm.DB, pc productpb.ProductServiceClient) *OrderServer {
	return &OrderServer{
		DB:            db,
		ProductClient: pc,
	}
}

func (s *OrderServer) CreateOrder(ctx context.Context, productID int64, quantity int32) string {
	log.Printf("[Order Service] 收到下單請求 -> 商品ID: %d, 數量: %d", productID, quantity)

	rpcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := s.ProductClient.DeductStock(rpcCtx, &productpb.DeductStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})

	if err != nil {
		log.Printf("[Order Service] 呼叫 Product Service 失敗: %v", err)
		return "系統繁忙，請稍後再試"
	}

	if !resp.Success {
		log.Printf("[Order Service] 扣減庫存失敗: %s", resp.Message)
		return resp.Message
	}

	order := OrderModel{
		ProductID: productID,
		Quantity:  quantity,
		Status:    "PAID",
	}

	if err := s.DB.Create(&order).Error; err != nil {
		log.Printf("[Order Service] 訂單寫入資料庫失敗: %v", err)
		return "訂單建立失敗"
	}

	log.Printf("[Order Service] 訂單建立成功！訂單ID: %d", order.ID)
	return "下單成功！"
}
