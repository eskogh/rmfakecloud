// Package automation provides optional, asynchronous integrations. It does not
// participate in the tablet's storage or messaging protocols.
package automation

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID string `json:"id"`
}
type Document struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}
type Page struct {
	ID     string `json:"id"`
	Number int    `json:"number,omitempty"`
}
type Content struct {
	Text   string `json:"text,omitempty"`
	PNGURL string `json:"png_url,omitempty"`
	PDFURL string `json:"pdf_url,omitempty"`
}
type Commit struct {
	Before string
	After  string
}

type Data struct {
	Commit   *Commit   `json:"-"`
	User     User      `json:"user"`
	Document *Document `json:"document,omitempty"`
	Page     *Page     `json:"page,omitempty"`
	Content  *Content  `json:"content,omitempty"`
	Test     bool      `json:"test,omitempty"`
	Source   string    `json:"source,omitempty"`
}
type Event struct {
	TargetID  string    `json:"-"`
	Version   string    `json:"version"`
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      Data      `json:"data"`
}

func NewEvent(name, uid string) Event {
	return Event{Version: "1", ID: uuid.NewString(), Event: name, Timestamp: time.Now().UTC(), Data: Data{User: User{ID: uid}}}
}

// Attachment opens a fresh stream for every attempt; callers own and close it.
// Attachments stay outside the JSON event and can be used by explicit actions.
type Attachment struct {
	Name        string
	ContentType string
	Size        int64
	Open        func(context.Context) (io.ReadCloser, error)
}
type Action interface {
	Send(context.Context, Event, *Attachment) error
}
type EventHandler interface {
	HandleEvent(context.Context, Event) error
}
type Integration interface {
	EventHandler
	ID() string
	Name() string
	Type() string
	Enabled() bool
	SubscribedEvents() []string
}

func Matches(pattern, event string) bool {
	return pattern == "*" || pattern == event || (strings.HasSuffix(pattern, ".*") && strings.HasPrefix(event, strings.TrimSuffix(pattern, "*")))
}
