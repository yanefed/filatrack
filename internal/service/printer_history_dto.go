package service

import "time"

// historyCheckpoint хранит границы синхронизации истории для конкретного сервера.
type historyCheckpoint struct {
	URL      string    `json:"url"`
	Since    time.Time `json:"since"`
	FullSync time.Time `json:"full_sync"`
}
