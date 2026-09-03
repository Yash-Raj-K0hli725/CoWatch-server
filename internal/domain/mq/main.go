package mq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	amqp "github.com/rabbitmq/amqp091-go"
)

const concurrency = 2

type Consumer struct {
	url       string
	queueName string
	conn      *amqp.Connection
	ch        *amqp.Channel
	wg        sync.WaitGroup
}

func NewConsumer(url, queueName string) *Consumer {
	return &Consumer{
		url:       url,
		queueName: queueName,
	}
}

// Start manages connection setup, message consumption, and automatic reconnections.
func (c *Consumer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := c.connectAndConsume(ctx); err != nil {
			log.Printf("[RabbitMQ] Error: %v. Retrying in 5 seconds...", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (c *Consumer) connectAndConsume(ctx context.Context) error {
	var err error

	c.conn, err = amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer c.conn.Close()

	c.ch, err = c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer c.ch.Close()

	// Declare durable queue
	_, err = c.ch.QueueDeclare(
		c.queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Prefetch 1 message at a time per worker
	if err = c.ch.Qos(concurrency, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Register consumer
	deliveries, err := c.ch.Consume(
		c.queueName,
		"",    // consumer tag (auto-generated if empty)
		false, // autoAck = false: manually acknowledge after processing
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}
	log.Printf("[RabbitMQ] Consumer ready. Worker PID: %d", os.Getpid())
	// Listen for unexpected channel/connection closures
	closeErrChan := make(chan *amqp.Error, 1)
	c.ch.NotifyClose(closeErrChan)

	workCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	var wg sync.WaitGroup
	for i := 1; i <= concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker := NewWorker(workerID)
			for {
				select {
				case <-workCtx.Done():
					return

				case d, ok := <-deliveries:
					if !ok {
						log.Printf("[RabbitMQ] Delivery stream closed (Worker %d exiting)", workerID)
						return
					}
					// Process message with manual Ack/Nack
					if err = worker.ProcessDelivery(workCtx, d); err != nil {
						log.Printf("[RabbitMQ] Handler error: %v (nack-ing message)", err)
						// Requeue message or forward to a Dead-Letter-Exchange (DLX)
						_ = d.Nack(false, true)
					} else {
						_ = d.Ack(false)
					}
				}
			}
		}(i)
	}

	// Coordinator loop: blocks function execution and monitors lifecycle
	var returnErr error
	select {
	case <-ctx.Done():
		log.Print("[RabbitMQ] Termination signal received. Initiating graceful drain...")
		cancelWorkers()

	case amqpErr, ok := <-closeErrChan:
		if ok && amqpErr != nil {
			returnErr = fmt.Errorf("channel closed unexpectedly: %w", amqpErr)
			log.Printf("[RabbitMQ] %v", returnErr)
		}
		cancelWorkers()
	}
	
	wg.Wait()
	log.Info("[RabbitMQ] All workers drained successfully.")

	return returnErr
}
