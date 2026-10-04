package detach

import (
	"bytes"
	"encoding/json"
	"time"
)

// Kind names a startup record
type Kind string

// Kind values
const (
	KindProfile  Kind = "profile"
	KindStarting Kind = "starting"
	KindReady    Kind = "ready"
	KindFailed   Kind = "failed"
	KindRunning  Kind = "running"
)

// Record is one JSON line the detached child writes to its parent during the startup
type Record struct {
	Kind     Kind          `json:"kind"`
	Services []string      `json:"services,omitempty"`
	Service  string        `json:"service,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Error    string        `json:"error,omitempty"`
	Address  string        `json:"address,omitempty"`
	PID      int           `json:"pid,omitempty"`
	Count    int           `json:"count,omitempty"`
}

// encode renders a record as one line
func encode(record Record) []byte {
	data, _ := json.Marshal(record)

	return append(data, '\n')
}

// decode reads a record from one line and returns false for a line of plain text, such as an error the child printed
func decode(line []byte) (Record, bool) {
	if !bytes.HasPrefix(line, []byte("{")) {
		return Record{}, false
	}

	var record Record
	if err := json.Unmarshal(line, &record); err != nil || record.Kind == "" {
		return Record{}, false
	}

	return record, true
}
