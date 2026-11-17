package events

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
)

type payload struct{ calls []string }

func TestPublish(t *testing.T) {
	e := New[*payload]("Test.OnSomething")
	e.Subscribe(func(p *payload) error { p.calls = append(p.calls, "base"); return nil })
	extensions.Install("events-app", func() {
		e.Subscribe(func(p *payload) error { p.calls = append(p.calls, "app"); return nil })
	})
	t.Cleanup(func() { extensions.SetDisabled() })

	p := &payload{}
	if err := e.Publish(p); err != nil || len(p.calls) != 2 || p.calls[1] != "app" {
		t.Errorf("calls %v, err %v", p.calls, err)
	}
	extensions.SetDisabled("events-app")
	p = &payload{}
	if err := e.Publish(p); err != nil || len(p.calls) != 1 {
		t.Errorf("disabled app's subscriber ran: %v", p.calls)
	}

	stop := errors.New("refused")
	e.Subscribe(func(*payload) error { return stop })
	e.Subscribe(func(p *payload) error { p.calls = append(p.calls, "after"); return nil })
	p = &payload{}
	if err := e.Publish(p); !errors.Is(err, stop) || len(p.calls) != 1 {
		t.Errorf("an error did not stop the event: %v %v", err, p.calls)
	}

	defer func() {
		if recover() == nil {
			t.Error("an event defined twice was accepted")
		}
	}()
	New[*payload]("Test.OnSomething")
}
