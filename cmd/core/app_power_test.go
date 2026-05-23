package main

import (
	"testing"
	"time"
)

func TestSystemResumeDetectionThreshold(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{name: "uses floor for fast polling", interval: time.Second, want: 20 * time.Second},
		{name: "scales with normal polling", interval: 5 * time.Second, want: 30 * time.Second},
		{name: "caps long polling threshold", interval: 20 * time.Second, want: 45 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := systemResumeDetectionThreshold(test.interval); got != test.want {
				t.Fatalf("systemResumeDetectionThreshold(%v) = %v, want %v", test.interval, got, test.want)
			}
		})
	}
}

func TestShouldRecoverFromSystemResumeGap(t *testing.T) {
	tests := []struct {
		name     string
		gap      time.Duration
		interval time.Duration
		want     bool
	}{
		{name: "ignores normal short gap", gap: 10 * time.Second, interval: time.Second, want: false},
		{name: "detects floor threshold", gap: 20 * time.Second, interval: time.Second, want: true},
		{name: "requires scaled threshold", gap: 40 * time.Second, interval: 10 * time.Second, want: false},
		{name: "detects capped threshold", gap: 45 * time.Second, interval: 10 * time.Second, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldRecoverFromSystemResumeGap(test.gap, test.interval); got != test.want {
				t.Fatalf("shouldRecoverFromSystemResumeGap(%v, %v) = %v, want %v", test.gap, test.interval, got, test.want)
			}
		})
	}
}
