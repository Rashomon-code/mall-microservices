package main

import (
	"context"
	"log"
	"mall/order-service/mq"
	"mall/order-service/server"
	productpb "mall/proto"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	mysqlHost := getEnv("MYSQL_HOST", "127.0.0.1")
	dsn := "root:rootpassword@tcp(" + mysqlHost + ":3306)/mall_order?charset=utf8mb4&parseTime=True&loc=Local"

	var err error
	var db *gorm.DB

	err = mq.InitRabbitMQ("amqp://admin:password123@mall_rabbitmq:5672/")
	if err != nil {
		log.Fatalf("RabbitMQ 初始化失敗: %v", err)
	}
	log.Println("RabbitMQ 連綫成功")

	for i := 0; i < 5; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("MySQL 尚未就緒，2 秒後重試... 錯誤原因: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("連接失敗,錯誤原因: %v", err)
	}
	log.Println("[Order Service] MySQL 連線成功！")

	productHost := getEnv("PRODUCT_SERVICE_HOST", "127.0.0.1")
	conn, err := grpc.Dial(productHost+":50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("無法連線至 Product gRPC 服務: %v", err)
	}
	defer conn.Close()

	productClient := productpb.NewProductServiceClient(conn)

	orderServer := server.NewOrderServer(db, productClient)
	log.Println("[Order Service] 服務初始化完成，準備開始模擬下單測試...")

	ctx := context.Background()

	log.Println("\n--- 下單：購買 3 件 ---")
	orderId, err := orderServer.CreateOrder(ctx, 101, 3)
	if err != nil {
		log.Fatalf("下單失敗: %v", err)
	}
	log.Printf("下單成功,訂單ID: %d", orderId)

	time.Sleep(1 * time.Second)

	log.Println("\n--- 取消訂單 ---")
	result, err := orderServer.CancelOrder(ctx, orderId, 101, 3)
	if err != nil {
		log.Printf("訂單取消失敗，原因: %v", err)
	} else {
		log.Printf("結果: %s", result)
		err = mq.PublishStockRollback(101, 3)
	}

	time.Sleep(1 * time.Second)

	log.Println("\n--- 再次下單 ---")
	newOrderId, err := orderServer.CreateOrder(ctx, 101, 4)
	if err != nil {
		log.Printf("結果: 下單失敗，原因: %v (代表庫存回補失敗)", err)
	} else {
		log.Printf("結果: 下單成功！新訂單 ID = %d (代表 Redis 庫存已成功回補！)", newOrderId)
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
