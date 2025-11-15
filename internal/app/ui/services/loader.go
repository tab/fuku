package services

import "github.com/charmbracelet/bubbles/spinner"

// LoaderItem represents a single loader operation
type LoaderItem struct {
	Service string
	Message string
}

// Loader holds loader state for loading indicators
type Loader struct {
	Model  spinner.Model
	Active bool
	queue  []LoaderItem
}

// Start adds a new operation to the loader queue (or updates existing)
func (l *Loader) Start(service, msg string) {
	for i := range l.queue {
		if l.queue[i].Service == service {
			l.queue[i].Message = msg
			return
		}
	}

	l.queue = append(l.queue, LoaderItem{Service: service, Message: msg})
	l.Active = true
}

// Stop removes a service operation from the queue
func (l *Loader) Stop(service string) {
	for i := 0; i < len(l.queue); i++ {
		if l.queue[i].Service == service {
			l.queue = append(l.queue[:i], l.queue[i+1:]...)
			break
		}
	}

	if len(l.queue) == 0 {
		l.Active = false
	}
}

// StopAll clears the entire loader queue
func (l *Loader) StopAll() {
	l.queue = nil
	l.Active = false
}

// Message returns the current loader message (front of queue)
func (l *Loader) Message() string {
	if len(l.queue) == 0 {
		return ""
	}

	return l.queue[0].Message
}

// Has checks if a service is already in the loader queue
func (l *Loader) Has(service string) bool {
	for _, item := range l.queue {
		if item.Service == service {
			return true
		}
	}

	return false
}
