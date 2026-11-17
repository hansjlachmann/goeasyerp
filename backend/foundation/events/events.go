// Package events: points in the business logic where extension apps add their own code without
// changing the base. The base application (or an app) defines an event with the type of what it
// passes,
//
//	var OnAfterPost = events.New[*AfterPost]("Sales-Post.OnAfterPost")
//
// publishes it at that point (`err := OnAfterPost.Publish(&AfterPost{…})`), and an app subscribes
// in its Register (`salesposting.OnAfterPost.Subscribe(func(e *salesposting.AfterPost) error {…})`).
// The subscribers of the enabled apps run in subscription order — the apps in dependency order;
// the first error stops and is the publisher's error (a posting is rolled back with it).
// An event is added to the base where an app needs one.
package events

import (
	"fmt"
	"sort"
	"sync"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
)

// Event is a point an app can subscribe to; T is what the publisher passes (a pointer to a
// struct of the event's values, which a subscriber may change where the event says so)
type Event[T any] struct {
	name string
	mu   sync.RWMutex
	subs []subscriber[T]
}

type subscriber[T any] struct {
	app string
	fn  func(T) error
}

var (
	namesMu sync.Mutex
	names   = map[string]bool{}
)

// New defines an event; a name is one event (defining it twice panics)
func New[T any](name string) *Event[T] {
	namesMu.Lock()
	defer namesMu.Unlock()
	if names[name] {
		panic(fmt.Sprintf("event %s is defined twice", name))
	}
	names[name] = true
	return &Event[T]{name: name}
}

// Name of the event
func (e *Event[T]) Name() string { return e.name }

// Subscribe adds a subscriber, as the app whose Register is running (extensions.Current)
func (e *Event[T]) Subscribe(fn func(T) error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subs = append(e.subs, subscriber[T]{app: extensions.Current(), fn: fn})
}

// Publish runs the subscribers of the enabled apps in subscription order; the first error
// stops and is returned
func (e *Event[T]) Publish(arg T) error {
	e.mu.RLock()
	subs := append([]subscriber[T](nil), e.subs...)
	e.mu.RUnlock()
	for _, s := range subs {
		if !extensions.Enabled(s.app) {
			continue
		}
		err := s.fn(arg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Names lists the defined events
func Names() []string {
	namesMu.Lock()
	defer namesMu.Unlock()
	list := make([]string, 0, len(names))
	for n := range names {
		list = append(list, n)
	}
	sort.Strings(list)
	return list
}
