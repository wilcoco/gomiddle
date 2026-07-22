package forward

import "testing"

func TestFirstSampleAlwaysSignificant(t *testing.T) {
	d := NewDetector(0.3)
	if !d.Significant(Sample{Key: "silo-1", Value: 5.0}) {
		t.Error("first sample for a key must be significant")
	}
}

func TestSmallChangeIsIgnored(t *testing.T) {
	d := NewDetector(0.3)
	d.Significant(Sample{Key: "silo-1", Value: 5.0}) // baseline
	if d.Significant(Sample{Key: "silo-1", Value: 5.2}) {
		t.Error("change of 0.2 (< threshold 0.3) must be ignored")
	}
}

func TestChangeAtOrAboveThresholdForwards(t *testing.T) {
	d := NewDetector(0.3)
	d.Significant(Sample{Key: "silo-1", Value: 5.0})
	if !d.Significant(Sample{Key: "silo-1", Value: 5.3}) {
		t.Error("change of exactly the threshold must forward")
	}
}

func TestBaselineFollowsLastForwarded(t *testing.T) {
	// After a forward, comparisons are against the new value, not the original.
	d := NewDetector(0.3)
	d.Significant(Sample{Key: "silo-1", Value: 5.0}) // baseline 5.0
	d.Significant(Sample{Key: "silo-1", Value: 5.3}) // forwarded, baseline now 5.3
	if d.Significant(Sample{Key: "silo-1", Value: 5.5}) {
		t.Error("5.5 vs new baseline 5.3 is 0.2, should be ignored")
	}
}

func TestSlowDriftEventuallyForwards(t *testing.T) {
	// Each step is below threshold, but the baseline only moves on a forward,
	// so accumulated drift must eventually cross the line.
	d := NewDetector(0.3)
	d.Significant(Sample{Key: "silo-1", Value: 5.00}) // baseline 5.00
	d.Significant(Sample{Key: "silo-1", Value: 5.15}) // ignored, baseline stays 5.00
	if !d.Significant(Sample{Key: "silo-1", Value: 5.30}) {
		t.Error("drift to 5.30 vs baseline 5.00 is 0.30, must forward")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	d := NewDetector(0.3)
	if !d.Significant(Sample{Key: "silo-1", Value: 1.0}) {
		t.Error("silo-1 first sample")
	}
	if !d.Significant(Sample{Key: "silo-2", Value: 1.0}) {
		t.Error("silo-2 must be judged independently of silo-1")
	}
}
