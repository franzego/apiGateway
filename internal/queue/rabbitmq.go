package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/franzego/apigateway/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Queuer interface {
	PublishEmail(message interface{}) error
	PublishPush(message interface{}) error
}

type RabbitMqClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  config.RabbitMqConfig
}

func NewRabbitMqService(cfg config.RabbitMqConfig) (*RabbitMqClient, error) {
	conn, err := amqp.Dial(cfg.Url)
	if err != nil {
		// log.Fatalf("error connecting to rabitmq with this url: %v", cfg.RabbitMq.Url)
		return nil, fmt.Errorf("error connecting to rabitmq with this url: %s", cfg.Url)
	}
	channel, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("error connecting to the channel: %v", err)
	}
	client := &RabbitMqClient{
		conn:    conn,
		channel: channel,
		config:  cfg,
	}
	if err := client.SetUpExchangeQueue(); err != nil {
		return nil, err
	}
	return client, nil

}
func (r *RabbitMqClient) CloseConnection() error {
	return r.conn.Close()
}

func (r *RabbitMqClient) SetUpExchangeQueue() error {
	if err := r.channel.ExchangeDeclare(
		r.config.Exchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %v", err)
	}
	queues := []string{
		r.config.EmailQueue,
		r.config.PushQueue,
	}
	for _, queue := range queues {
		_, err := r.channel.QueueDeclare(
			queue,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("error in declaring queue: %s", queue)
		}
		if err := r.channel.QueueBind(queue, queue, r.config.Exchange, false, nil); err != nil {
			return fmt.Errorf("error in binding queue to exchange: %s", queue)
		}
	}
	return nil
}

func (r *RabbitMqClient) Publish(routingKey string, message interface{}) error {
	by, err := json.Marshal(&message)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := r.channel.PublishWithContext(
		ctx,
		r.config.Exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         by,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	); err != nil {
		return fmt.Errorf("an error occured during publisihing: %v", err)
	}
	return nil
}

func (r *RabbitMqClient) PublishEmail(message interface{}) error {
	return r.Publish(r.config.EmailQueue, message)
}
func (r *RabbitMqClient) PublishPush(message interface{}) error {
	return r.Publish(r.config.PushQueue, message)
}
