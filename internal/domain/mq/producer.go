package mq

import (
	"StreamRoom/internal/views"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Producer struct {
	ch    *amqp.Channel
	conn  *amqp.Connection
	queue string
}

func NewProducer(name, url string) (producer *Producer, err error) {
	var conn *amqp.Connection
	conn, err = amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to dial rabbitmq: %w", err)
	}
	var ch *amqp.Channel
	ch, err = conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}
	_, err = ch.QueueDeclare(
		name,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}
	return &Producer{
		ch:    ch,
		conn:  conn,
		queue: name,
	}, nil
}

func (p *Producer) PushTask(c context.Context, task views.TaskRequest) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}
	err = p.ch.PublishWithContext(
		c,
		"",
		p.queue,
		false, false, amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    time.Now(),
			MessageId:    uuid.NewString(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}

func (p *Producer) Close() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}
