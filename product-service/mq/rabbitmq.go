package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type OrderMessage struct {
	ProductID int64 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

type Manager struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func InitRabbitMQ(amqpURL string) (*Manager, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("無法建立 RabbitMQ 連接: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (m *Manager) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.ch != nil {
		_ = m.ch.Close()
		log.Println("已關閉 AMQP 通道")
	}

	m.wg.Wait()

	if m.conn != nil {
		_ = m.conn.Close()
		log.Println("已斷開 RabbitMQ 連接")
	}
}

func (m *Manager) StartOrderListener(queueName string, workerCount int, handler func(msg OrderMessage)) error {
	ch, err := m.conn.Channel()
	if err != nil {
		return fmt.Errorf("無法建立通道: %w", err)
	}
	m.ch = ch

	err = ch.Qos(workerCount*2, 0, false)
	if err != nil {
		return fmt.Errorf("無法設置 QoS: %w", err)
	}

	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("無法建立佇列: %w", err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("注冊接收端失敗: %w", err)
	}

	log.Printf("開始監聽: %s，啟動了 %d 個 Worker", q.Name, workerCount)

	for i := 0; i < workerCount; i++ {
		m.wg.Add(1)
		go func(workerID int) {
			defer m.wg.Done()

			for {
				select {
				case <-m.ctx.Done():
					// 收到優雅關閉信號，退出迴圈
					log.Printf("[Worker %d] 收到停止訊號，準備退出...", workerID)
					return
				case d, ok := <-msgs:
					if !ok {
						// 說明底層的 RabbitMQ Channel 已經被關閉了
						log.Printf("[Worker %d] 訊息通道已關閉", workerID)
						return
					}

					var orderMsg OrderMessage
					if err := json.Unmarshal(d.Body, &orderMsg); err != nil {
						log.Printf("[Worker %d] 反序列化失敗: %v", workerID, err)
						_ = d.Nack(false, false)
						continue
					}

					handler(orderMsg)
					_ = d.Ack(false)
				}
			}
		}(i)
	}

	return nil
}
