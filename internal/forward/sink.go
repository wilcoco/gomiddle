package forward

import (
	"context"
	"log/slog"
	"time"
)

// Event is a meaningful change that survived the Detector and is worth
// persisting or reporting. Source/Key identify what changed; Value/Raw carry
// the reading.
type Event struct {
	Source string    // "silo", "injection", ...
	Key    string    // e.g. "silo-1"
	Value  float64   // decoded value (e.g. tons)
	Raw    int       // raw register value, for auditing
	At     time.Time // when the change was observed
}

// Sink receives meaningful events. Implementations must be safe for
// concurrent use and should not block the caller for long — a slow sink
// stalls polling. A real PostgreSQL sink will implement this interface.
type Sink interface {
	Emit(ctx context.Context, e Event) error
}

// LogSink writes events to the logger. It is the default sink for development
// and a useful fallback until the database sink is wired in.
type LogSink struct {
	Log *slog.Logger
}

func (s LogSink) Emit(_ context.Context, e Event) error {
	s.Log.Info("meaningful change",
		"source", e.Source, "key", e.Key, "value", e.Value, "raw", e.Raw)
	return nil
}
