// Package forward is the heart of the middleware: it turns a continuous
// stream of poll readings into a sparse stream of *meaningful* events, so
// downstream systems (PostgreSQL, Odoo) receive only significant changes
// instead of every sample.
package forward

import (
	"math"
	"sync"
)

// Sample is one named numeric reading to evaluate for significance.
type Sample struct {
	Key   string  // stable identity across polls, e.g. "silo-1"
	Value float64 // the value to compare against the last forwarded one
}

// Detector decides whether a sample has changed enough since the last one it
// let through. It keeps the last *forwarded* value per key — not the last
// polled value — so slow drift that never individually clears the threshold
// still eventually triggers once the accumulated change is large enough.
type Detector struct {
	threshold float64

	mu   sync.Mutex
	last map[string]float64
}

// NewDetector returns a Detector that forwards a sample when its value differs
// from the last forwarded value for that key by at least threshold (absolute).
func NewDetector(threshold float64) *Detector {
	return &Detector{threshold: threshold, last: make(map[string]float64)}
}

// Significant reports whether s should be forwarded. The first sample for any
// key is always significant (there is no baseline yet). When it returns true,
// the sample becomes the new baseline for future comparisons.
func (d *Detector) Significant(s Sample) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	prev, seen := d.last[s.Key]
	// epsilon absorbs floating-point error so a change of "exactly" the
	// threshold forwards: e.g. 5.3-5.0 is 0.2999…, not 0.3, and without this
	// a value that moved by the configured amount would be silently dropped.
	const epsilon = 1e-9
	if seen && math.Abs(s.Value-prev) < d.threshold-epsilon {
		return false
	}
	d.last[s.Key] = s.Value
	return true
}
