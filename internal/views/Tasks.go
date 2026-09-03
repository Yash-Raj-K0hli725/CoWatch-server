package views

import "time"

type TaskRequest struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Obzect    string    `json:"obzect"`
}
