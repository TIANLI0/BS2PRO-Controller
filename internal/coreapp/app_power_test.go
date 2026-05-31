package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
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

func TestShouldSendTargetRPM(t *testing.T) {
	tests := []struct {
		name          string
		targetRPM     int
		prevTargetRPM int
		minRPMChange  int
		fanData       *types.FanData
		want          bool
	}{
		{name: "rejects zero target", targetRPM: 0, prevTargetRPM: -1, minRPMChange: 50, want: false},
		{name: "sends initial positive target", targetRPM: 1800, prevTargetRPM: -1, minRPMChange: 50, want: true},
		{name: "sends significant target change", targetRPM: 1900, prevTargetRPM: 1800, minRPMChange: 50, want: true},
		{name: "skips small target change", targetRPM: 1820, prevTargetRPM: 1800, minRPMChange: 50, want: false},
		{name: "resends when device reports zero target", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 0}, want: true},
		{name: "resends when device target drifts", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 1700}, want: true},
		{name: "keeps small device target drift", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 1775}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSendTargetRPM(test.targetRPM, test.prevTargetRPM, test.minRPMChange, test.fanData); got != test.want {
				t.Fatalf("shouldSendTargetRPM() = %v, want %v", got, test.want)
			}
		})
	}
}
