package runtime

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// LogFilePath is the location of the combined log file
const LogFilePath = "/tmp/fuku.log"

// LogFilter provides service filtering for log output
type LogFilter interface {
	IsEnabled(service string) bool
}

// LogWriter writes service logs to a file with runtime filtering
type LogWriter interface {
	Start(ctx context.Context)
	Close() error
}

type logWriter struct {
	eventBus   EventBus
	filter     LogFilter
	file       *os.File
	mu         sync.RWMutex
	done       chan struct{}
	writeQueue chan string
	writerDone chan struct{}
	started    bool
	cancel     context.CancelFunc
}

// NewLogWriter creates a new log writer
func NewLogWriter(eventBus EventBus, filter LogFilter) LogWriter {
	return &logWriter{
		eventBus:   eventBus,
		filter:     filter,
		done:       make(chan struct{}),
		writeQueue: make(chan string, 10000),
		writerDone: make(chan struct{}),
	}
}

// Start begins listening for log events and writing to file
func (lw *logWriter) Start(_ context.Context) {
	if err := lw.openFile(); err != nil {
		return
	}

	lw.started = true

	ctx, cancel := context.WithCancel(context.Background())
	lw.cancel = cancel

	eventChan := lw.eventBus.Subscribe(ctx)

	go lw.processEvents(eventChan)
	go lw.writeLoop()
}

// Close stops the log writer and removes the log file
func (lw *logWriter) Close() error {
	if !lw.started {
		return nil
	}

	lw.cancel()
	close(lw.done)
	close(lw.writeQueue)
	<-lw.writerDone

	lw.mu.Lock()
	defer lw.mu.Unlock()

	if lw.file == nil {
		return nil
	}

	if err := lw.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	lw.file = nil

	if err := os.Remove(LogFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove log file: %w", err)
	}

	return nil
}

func (lw *logWriter) openFile() error {
	_ = os.Remove(LogFilePath)

	file, err := os.Create(LogFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	lw.mu.Lock()
	lw.file = file
	lw.mu.Unlock()

	return nil
}

func (lw *logWriter) processEvents(eventChan <-chan Event) {
	for {
		select {
		case <-lw.done:
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			lw.handleEvent(event)
		}
	}
}

func (lw *logWriter) handleEvent(event Event) {
	if event.Type != EventLogLine {
		return
	}

	data, ok := event.Data.(LogLineData)
	if !ok {
		return
	}

	if !lw.filter.IsEnabled(data.Service) {
		return
	}

	select {
	case lw.writeQueue <- data.Message:
	default:
	}
}

func (lw *logWriter) writeLoop() {
	defer close(lw.writerDone)

	for message := range lw.writeQueue {
		lw.writeLine(message)
	}
}

func (lw *logWriter) writeLine(message string) {
	lw.mu.RLock()
	defer lw.mu.RUnlock()

	if lw.file == nil {
		return
	}

	fmt.Fprintln(lw.file, message)
	_ = lw.file.Sync()
}
