package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	productpb "mall/proto"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ProductModel struct {
	Id        int64     `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name;size:255"`
	Price     int64     `gorm:"column:price"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (ProductModel) TableName() string {
	return "products"
}

type ProductServer struct {
	productpb.UnimplementedProductServiceServer
	DB  *gorm.DB
	RDB *redis.Client
}

func NewProductServer(db *gorm.DB, rdb *redis.Client) *ProductServer {
	return &ProductServer{
		DB:  db,
		RDB: rdb,
	}
}

func (s *ProductServer) DeductStock(ctx context.Context, req *productpb.DeductStockRequest) (*productpb.DeductStockResponse, error) {
	log.Printf("[Product Service] 收到扣減庫存請求: 商品ID=%d, 數量=%d", req.ProductId, req.Quantity)

	redisKey := fmt.Sprintf("product:stock:%d", req.ProductId)

	newStock, err := s.RDB.DecrBy(ctx, redisKey, int64(req.Quantity)).Result()
	if err != nil {
		return &productpb.DeductStockResponse{Success: false, Message: "系統錯誤，無法讀取庫存"}, nil
	}

	if newStock < 0 {
		s.RDB.IncrBy(ctx, redisKey, int64(req.Quantity))
		log.Printf("[Product Service] 秒殺失敗！商品ID: %d 庫存不足", req.ProductId)
		return &productpb.DeductStockResponse{Success: false, Message: "秒殺失敗，商品已售罄！"}, nil
	}

	log.Printf("[Product Service] 秒殺成功！Redis 扣減成功，剩餘庫存: %d", newStock)

	return &productpb.DeductStockResponse{Success: true, Message: "庫存扣減成功！恭喜搶到商品！"}, nil
}

func (s *ProductServer) GetProduct(ctx context.Context, req *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
	log.Printf("[Product Service] 查詢商品資訊 -> 商品ID: %d", req.ProductId)

	var prod ProductModel
	err := s.DB.WithContext(ctx).First(&prod, req.ProductId).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("找不到該商品")
		}
		return nil, err
	}

	redisKey := fmt.Sprintf("product:stock:%d", req.ProductId)
	stockStr, err := s.RDB.Get(ctx, redisKey).Result()

	var stock int32 = 0
	if err == nil {
		fmt.Sscanf(stockStr, "%d", &stock)
	} else if errors.Is(err, redis.Nil) {
		stock = 0
	}

	return &productpb.GetProductResponse{
		Id:    prod.Id,
		Name:  prod.Name,
		Price: prod.Price,
		Stock: stock,
	}, nil
}
