package models

import (
	"encoding/json"
	"time"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	// JobStatusPending indicates the job is waiting to be executed
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning indicates the job is currently being executed
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted indicates the job has completed successfully
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job has failed
	JobStatusFailed JobStatus = "failed"
)

// Job represents a command to be executed
type Job struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	QueueName string    `json:"queue_name"`
	Priority  int       `json:"priority"`
	Status    JobStatus `json:"status"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// BashCommandArgs holds the arguments for a bash command job
type BashCommandArgs struct {
	Command string `json:"command"`
}

// MarshalJSON implements json.Marshaler for BashCommandArgs
func (args BashCommandArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"command": args.Command,
	})
}

// UnmarshalJSON implements json.Unmarshaler for BashCommandArgs
func (args *BashCommandArgs) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if cmd, ok := m["command"].(string); ok {
		args.Command = cmd
	}

	return nil
}
