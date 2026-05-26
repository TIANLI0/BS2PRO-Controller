package smartcontrol

import (
	"testing"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

func TestCalculateTargetRPMIgnoresOffsetsWhenLearningDisabled(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		Learning:       false,
		MaxLearnOffset: 600,
		LearnedOffsets: []int{500, 500},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 1500 {
		t.Fatalf("CalculateTargetRPM() = %d, want base curve RPM 1500", got)
	}
}

func TestCalculateTargetRPMAppliesOffsetsWhenLearningEnabled(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		MaxLearnOffset: 600,
		LearnedOffsets: []int{500, 500},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 1750 {
		t.Fatalf("CalculateTargetRPM() = %d, want learned curve RPM 1750", got)
	}
}

func TestCalculateTargetRPMRespectsCoolingBias(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		LearningBias:   types.LearningBiasCooling,
		MaxLearnOffset: 600,
		LearnedOffsets: []int{-500, -500},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 1500 {
		t.Fatalf("CalculateTargetRPM() = %d, want base curve RPM 1500", got)
	}
}

func TestCalculateTargetRPMRespectsQuietBias(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		Learning:       true,
		LearningBias:   types.LearningBiasQuiet,
		MaxLearnOffset: 600,
		LearnedOffsets: []int{500, 500},
	}

	got := CalculateTargetRPM(60, curve, cfg)
	if got != 1500 {
		t.Fatalf("CalculateTargetRPM() = %d, want base curve RPM 1500", got)
	}
}

func TestLearnSteadyOffsetRespectsLearningBias(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	prevOffsets := []int{0, 0}

	if offsets, changed := LearnSteadyOffset(1, 60, curve, prevOffsets, types.SmartControlConfig{
		LearningBias:   types.LearningBiasCooling,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}); changed || offsets[0] != 0 || offsets[1] != 0 {
		t.Fatalf("cooling bias learned offsets = %v, changed=%v; want unchanged zeros", offsets, changed)
	}

	if offsets, changed := LearnSteadyOffset(0, 60, curve, prevOffsets, types.SmartControlConfig{
		LearningBias:   types.LearningBiasQuiet,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}); changed || offsets[0] != 0 || offsets[1] != 0 {
		t.Fatalf("quiet bias learned offsets = %v, changed=%v; want unchanged zeros", offsets, changed)
	}
}
