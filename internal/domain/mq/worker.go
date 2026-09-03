package mq

import (
	"StreamRoom/enums"
	"StreamRoom/internal/views"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	id     int
	logger *log.Logger
}

func NewWorker(id int) *Worker {
	return &Worker{
		id:     id,
		logger: log.New(os.Stdout, fmt.Sprintf("[Worker-%d] ", id), log.LstdFlags|log.Lmsgprefix),
	}
}

// ProcessDelivery handles decoding, validation, execution, and ack/nack logic.
func (w *Worker) ProcessDelivery(ctx context.Context, d amqp.Delivery) (err error) {
	currentStage := enums.TaskReceived
	w.logger.Printf("Stage: %s | MessageID: %s", currentStage, d.MessageId)

	// Recover from panics inside job execution to prevent worker crashes
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during stage %s: %v", currentStage, r)
			currentStage = enums.TaskFailed
			w.logger.Printf("Stage: %s | Error: %v", currentStage, err)
			_ = d.Nack(false, false) // Drop or route to dead-letter queue
		}
	}()

	// 1. Validation & Unmarshaling Stage
	currentStage = enums.TaskValidating
	var task views.TaskRequest
	if err := json.Unmarshal(d.Body, &task); err != nil {
		w.logger.Printf("Stage: %s | Invalid JSON: %v", currentStage, err)
		_ = d.Nack(false, false) // Malformed payloads should not be requeued
		return err
	}
	//TODO add validations
	//if len(task.Numbers) == 0 {
	//	w.logger.Printf("Stage: %s | Validation failed: empty numbers array", currentStage)
	//	_ = d.Nack(false, false)
	//	return errors.New("empty numbers list")
	//}

	// 2. Processing Stage (with contextual timeout)
	currentStage = enums.TaskProcessing
	w.logger.Printf("Stage: %s | RoomID %s", currentStage, task.ID)

	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = w.startCompilation(jobCtx, task.Obzect)
	if err != nil {
		currentStage = enums.TaskFailed
		w.logger.Printf("Stage: %s | Execution error: %v", currentStage, err)
		_ = d.Nack(false, true) // Requeue for transient failures
		return err
	}

	// 3. Finished Stage & Ack
	currentStage = enums.TaskProcessed
	w.logger.Printf("Stage: %s | Task ID: %s ", currentStage, task.ID)
	return d.Ack(false)
}

func (w *Worker) startCompilation(ctx context.Context, obzect string) error {
	return nil
}
