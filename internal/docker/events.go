package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// EventInfo is a normalized Docker daemon event for the TUI.
type EventInfo struct {
	Time    time.Time
	Type    string
	Action  string
	Actor   string
	ID      string
	Message string
	Level   EventLevel
}

type EventLevel int

const (
	EventInfoLevel EventLevel = iota
	EventWarnLevel
	EventCritLevel
)

// WatchEvents streams daemon events until ctx is cancelled.
func (c *Client) WatchEvents(ctx context.Context) (<-chan EventInfo, <-chan error) {
	out := make(chan EventInfo, 64)
	errs := make(chan error, 1)
	if c.IsDemo() {
		go c.demoEventLoop(ctx, out, errs)
		return out, errs
	}

	go func() {
		defer close(out)
		defer close(errs)

		opts := events.ListOptions{
			Filters: filters.NewArgs(
				filters.Arg("type", "container"),
			),
		}
		msgCh, errCh := c.cli.Events(ctx, opts)
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errCh:
				if !ok {
					return
				}
				if err != nil && ctx.Err() == nil {
					errs <- err
				}
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				info := normalizeEvent(msg)
				select {
				case out <- info:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, errs
}

func normalizeEvent(msg events.Message) EventInfo {
	action := string(msg.Action)
	if action == "" {
		action = msg.Status
	}
	name := msg.Actor.Attributes["name"]
	if name == "" {
		name = shortID(msg.Actor.ID)
	}
	id := msg.Actor.ID
	if id == "" {
		id = msg.ID
	}

	attrs := make([]string, 0, 4)
	if img := msg.Actor.Attributes["image"]; img != "" {
		attrs = append(attrs, "image="+img)
	}
	if exit := msg.Actor.Attributes["exitCode"]; exit != "" {
		attrs = append(attrs, "exit="+exit)
	}
	if h := msg.Actor.Attributes["health_status"]; h != "" {
		attrs = append(attrs, "health="+h)
	}

	ts := time.Unix(msg.Time, 0)
	if msg.TimeNano > 0 {
		ts = time.Unix(0, msg.TimeNano)
	}

	info := EventInfo{
		Time:   ts,
		Type:   string(msg.Type),
		Action: action,
		Actor:  name,
		ID:     id,
		Level:  classifyEvent(action, msg.Actor.Attributes),
	}
	if len(attrs) > 0 {
		info.Message = fmt.Sprintf("%s %s (%s)", action, name, strings.Join(attrs, " "))
	} else {
		info.Message = fmt.Sprintf("%s %s", action, name)
	}
	return info
}

func classifyEvent(action string, attrs map[string]string) EventLevel {
	a := strings.ToLower(action)
	switch {
	case a == "oom", strings.HasPrefix(a, "oom"):
		return EventCritLevel
	case a == "die", a == "kill":
		return EventCritLevel
	case strings.Contains(a, "health_status"):
		h := strings.ToLower(attrs["health_status"])
		if h == "unhealthy" || strings.Contains(a, "unhealthy") {
			return EventCritLevel
		}
		if h == "healthy" {
			return EventInfoLevel
		}
		return EventWarnLevel
	case a == "restart", a == "oom-kill":
		return EventWarnLevel
	default:
		return EventInfoLevel
	}
}

func (c *Client) demoEventLoop(ctx context.Context, out chan EventInfo, errs chan error) {
	defer close(out)
	defer close(errs)
	samples := []EventInfo{
		{Action: "start", Actor: "web", Message: "start web", Level: EventInfoLevel},
		{Action: "health_status: healthy", Actor: "api", Message: "health_status: healthy api", Level: EventInfoLevel},
		{Action: "health_status: unhealthy", Actor: "db", Message: "health_status: unhealthy db", Level: EventCritLevel},
		{Action: "die", Actor: "loki", Message: "die loki (exit=0)", Level: EventCritLevel},
		{Action: "oom", Actor: "worker", Message: "oom worker", Level: EventCritLevel},
		{Action: "restart", Actor: "cache", Message: "restart cache", Level: EventWarnLevel},
	}
	i := 0
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			ev := samples[i%len(samples)]
			ev.Time = t
			ev.Type = "container"
			i++
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}
}

// FormatEventLine renders one event for the events panel.
func FormatEventLine(ev EventInfo) string {
	ts := ev.Time.Format("15:04:05")
	mark := " "
	switch ev.Level {
	case EventWarnLevel:
		mark = "!"
	case EventCritLevel:
		mark = "*"
	}
	msg := ev.Message
	if msg == "" {
		msg = fmt.Sprintf("%s %s", ev.Action, ev.Actor)
	}
	return fmt.Sprintf("%s %s %s", ts, mark, msg)
}
