package server

import (
	"context"
	"errors"
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

func (s *OrderServer) CreateOrder(ctx context.Context, productID int64, quantity int32) (int64, error) {
	log.Printf("[Order Service] 收到下單請求 -> 商品ID: %d, 數量: %d", productID, quantity)

	rpcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := s.ProductClient.DeductStock(rpcCtx, &productpb.DeductStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})

	if err != nil {
		log.Printf("[Order Service] 呼叫 Product Service 失敗: %v", err)
		return 0, errors.New("系統繁忙，請稍後再試")
	}

	if !resp.Success {
		log.Printf("[Order Service] 扣減庫存失敗: %s", resp.Message)
		return 0, errors.New(resp.Message)
	}

	order := OrderModel{
		ProductID: productID,
		Quantity:  quantity,
		Status:    "PAID",
	}

	if err := s.DB.Create(&order).Error; err != nil {
		log.Printf("[Order Service] 訂單寫入資料庫失敗: %v", err)
		return 0, errors.New("訂單建立失敗")
	}

	log.Printf("[Order Service] 訂單建立成功！訂單ID: %d", order.ID)
	return order.ID, nil
}

func (s *OrderServer) CancelOrder(ctx context.Context, orderId int64, productId int64, quantity int32) string {
	log.Printf("[Order Service] 收到取消請求 -> 商品ID: %d, 數量: %d", productId, quantity)

	rpcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := s.ProductClient.AddStock(rpcCtx, &productpb.AddStockRequest{
		ProductId: productId,
		Quantity:  quantity,
	})
	if err != nil {
		log.Printf("[Order Service] 呼叫 Product Service 失敗: %v", err)
		return "請求失敗，請稍後再試"
	}

	if !resp.Success {
		log.Printf("[Order Service] 添加庫存失敗: %s", resp.Message)
	}

	result := s.DB.Model(&OrderModel{}).Where("ID = ?", orderId).Update("Status", "CANCELED")
	if result.Error != nil {
		log.Printf("資料庫更新失敗: %v", result.Error)
		return "資料庫未更新！"
	}

	if result.RowsAffected == 0 {
		log.Println("沒有任何資料被更新（可能找不到該 ID，或是新值與舊值一樣）")
	}

	return "取消成功！"
}
