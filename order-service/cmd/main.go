package main

import (
	"context"
	"log"
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

	log.Println("\n--- 第一次下單測試：購買 3 件 ---")
	result1 := orderServer.CreateOrder(ctx, 101, 3)
	log.Printf("結果: %s", result1)

	time.Sleep(1 * time.Second)

	log.Println("\n--- 第二次下單測試：購買 3 件 ---")
	result2 := orderServer.CreateOrder(ctx, 101, 3)
	log.Printf("結果: %s", result2)

	log.Println("\n--- 取消訂單測試： 取消三件")
	result3 := orderServer.CancelOrder()
	log.Printf("結果: %s", result3)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
