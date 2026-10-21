package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

type Logger struct {
	mu     sync.Mutex
	writer io.Writer
}

func NewLogger(writer io.Writer) *Logger {
	return &Logger{writer: writer}
}

func (logger *Logger) Emit(level, message string, fields map[string]interface{}) error {
	record := make(map[string]interface{}, len(fields)+3)
	for key, value := range fields {
		record[key] = value
	}
	record["timestamp"] = time.Now().UTC()
	record["level"] = level
	record["message"] = message
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode structured log record: %w", err)
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if _, err := logger.writer.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write structured log record: %w", err)
	}
	return nil
}
