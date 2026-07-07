package main

import (
	"context"
	"log"
	"mall/product-service/server"
	productpb "mall/proto"
	"net"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	ctx := context.Background()

	mysqlHost := getEnv("MYSQL_HOST", "127.0.0.1")

	// dsn 格式: 使用者:密碼@tcp(主機:連接埠)/資料庫名稱?參數1&參數2
	dsn := "root:rootpassword@tcp(" + mysqlHost + ":3306)/mall_order?charset=utf8mb4&parseTime=True&loc=Local"

	var db *gorm.DB
	var err error

	for i := 0; i < 5; i++ {
		log.Printf("[Product Service] 正在嘗試連線 MySQL... (第 %d/10 次)", i+1)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("MySQL 尚未就緒，2 秒後重試... 錯誤原因: %v", err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("無法連線至 MySQL，已達最大重試次數: %v", err)
	}
	log.Println("[Product Service] MySQL 連線成功！")

	redisHost := getEnv("REDIS_HOST", "127.0.0.1")

	rdb := redis.NewClient(&redis.Options{
		Addr: redisHost + ":6379",
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("無法連線至 Redis: %v", err)
	}
	log.Println("[Product Service] Redis 連線成功！")

	err = rdb.Set(ctx, "product:stock:101", 5, 0).Err()
	if err != nil {
		log.Fatalf("Redis 庫存預熱失敗: %v", err)
	}
	log.Println("[Product Service] 商品 101 庫存預熱成功！當前秒殺庫存: 5 件")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("無法監聽連接埠: %v", err)
	}

	grpcServer := grpc.NewServer()
	productServer := server.NewProductServer(db, rdb)
	productpb.RegisterProductServiceServer(grpcServer, productServer)

	log.Println("[Product Service] gRPC 伺服器正在 port :50051 運行中...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("無法啟動 gRPC 服務: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
