package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

var RabbitConn *amqp.Connection

func InitRabbitMQ(amqpURL string) error {
	var err error
	RabbitConn, err = amqp.Dial(amqpURL)
	if err != nil {
		return fmt.Errorf("無法建立 RabbitMQ 連接: %w", err)
	}
	return nil
}

func PublishStockRollback(productId int64, quantity int) error {
	ch, err := RabbitConn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare("stock_rollback_queue", true, false, false, false, nil)
	if err != nil {
		return err
	}

	msgBody := fmt.Sprintf(`{"product_id":"%d", "quantity":%d}`, productId, quantity)

	return ch.PublishWithContext(context.Background(), "", q.Name, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(msgBody),
	})
}
