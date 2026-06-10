package smartcontrol

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
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

	// 低于目标带的工况会要求降转速（负偏移），cooling 倾向禁止负偏移 → 不变。
	if offsets, changed := LearnSteadyOffset(1, 60, 0, 0, false, curve, prevOffsets, types.SmartControlConfig{
		TargetTemp:     70,
		LearningBias:   types.LearningBiasCooling,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}); changed || offsets[0] != 0 || offsets[1] != 0 {
		t.Fatalf("cooling bias learned offsets = %v, changed=%v; want unchanged zeros", offsets, changed)
	}

	// 高于目标温度的工况会要求加转速（正偏移），quiet 倾向禁止正偏移 → 不变。
	if offsets, changed := LearnSteadyOffset(0, 80, 0, 0, false, curve, prevOffsets, types.SmartControlConfig{
		TargetTemp:     70,
		LearningBias:   types.LearningBiasQuiet,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}); changed || offsets[0] != 0 || offsets[1] != 0 {
		t.Fatalf("quiet bias learned offsets = %v, changed=%v; want unchanged zeros", offsets, changed)
	}
}

func TestStableObserverUsesConfiguredWindowAndDelay(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 60, RPM: 1800}}
	observer := NewStableObserver(len(curve))
	cfg := types.SmartControlConfig{
		LearnWindow:    4,
		LearnDelay:     2,
		MinRPMChange:   50,
		TargetTemp:     68,
		MaxLearnOffset: 300,
	}

	for i := range 5 {
		if steady := observer.Observe(60, 1800, curve, cfg); steady.Ready {
			t.Fatalf("sample %d unexpectedly reached steady state", i)
		}
	}
	if steady := observer.Observe(60, 1800, curve, cfg); !steady.Ready {
		t.Fatalf("expected steady state after configured delay+window")
	}
}

func TestLearnSteadyOffsetHoldsInComfortBand(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}
	// 舒适带为 [70-5, 70] = [65,70]，带内不应再调整（消除“无脑降温”）。
	if offsets, changed := LearnSteadyOffset(1, 68, 0, 0, false, curve, []int{0, 0}, cfg); changed {
		t.Fatalf("in-band steady temp should not change offsets, got %v changed=%v", offsets, changed)
	}
}

func TestLearnSteadyOffsetOnlyAdjustsLocalBucket(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
		{Temperature: 90, RPM: 3000},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 300,
	}
	offsets, changed := LearnSteadyOffset(1, 82, 0, 0, false, curve, []int{0, 0, 0}, cfg)
	if !changed {
		t.Fatalf("expected local bucket learning to change offsets")
	}
	if offsets[1] <= 0 {
		t.Fatalf("expected middle bucket offset to increase, got %v", offsets)
	}
	if offsets[0] != 0 || offsets[2] != 0 {
		t.Fatalf("expected neighboring buckets to remain unchanged, got %v", offsets)
	}
	if offsets[1] >= 80 {
		t.Fatalf("expected smoothing to keep a single-step change below the hard step cap, got %v", offsets)
	}
}

func TestLearnSteadyOffsetCoolsWhenAboveTarget(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}
	offsets, changed := LearnSteadyOffset(0, 80, 0, 0, false, curve, []int{0, 0}, cfg)
	if !changed || offsets[0] <= 0 {
		t.Fatalf("above-target steady temp should raise RPM offset, got %v changed=%v", offsets, changed)
	}
}

func TestLearnSteadyOffsetSavesNoiseWhenBelowTarget(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 2000},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      10,
		MaxLearnOffset: 600,
	}
	offsets, changed := LearnSteadyOffset(1, 55, 0, 0, false, curve, []int{0, 0}, cfg)
	if !changed || offsets[1] >= 0 {
		t.Fatalf("well-below-target steady temp should lower RPM offset, got %v changed=%v", offsets, changed)
	}
}

// 冷却低效时，同样的温差应允许更大幅度的降速（更省噪音）。
func TestLearnSteadyOffsetEfficiencyScalesReduction(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1000},
		{Temperature: 70, RPM: 3000},
	}
	cfg := types.SmartControlConfig{
		TargetTemp:     70,
		Hysteresis:     2,
		LearnRate:      6,
		MaxLearnOffset: 1000,
	}
	// 高效冷却（0.02°C/RPM）：降速幅度小。
	effHigh, _ := LearnSteadyOffset(1, 55, 0, 0.02, true, curve, []int{0, 0}, cfg)
	// 低效冷却（0.002°C/RPM）：降速幅度大。
	effLow, _ := LearnSteadyOffset(1, 55, 0, 0.002, true, curve, []int{0, 0}, cfg)
	if !(effLow[1] < effHigh[1]) {
		t.Fatalf("lower cooling efficiency should reduce RPM more aggressively: low=%d high=%d", effLow[1], effHigh[1])
	}
}

// 噪音档案：局部斜率陡（降速省噪多）时降速应比斜率平坦时更激进。
func TestNoiseDownGainScalesWithLocalSlope(t *testing.T) {
	// 1000-2500 RPM 几乎不增噪，2500-4000 RPM 噪音陡升。
	profile := []types.NoiseProfilePoint{
		{RPM: 1000, DB: 0},
		{RPM: 1500, DB: 0.3},
		{RPM: 2000, DB: 0.6},
		{RPM: 2500, DB: 1.0},
		{RPM: 3000, DB: 5.0},
		{RPM: 3500, DB: 10.0},
		{RPM: 4000, DB: 15.0},
	}
	cfg := types.SmartControlConfig{NoiseWeight: 4, NoiseProfile: profile}

	flatGain := noiseDownGain(1500, cfg)
	steepGain := noiseDownGain(3500, cfg)
	if !(flatGain < 1 && steepGain > 1 && steepGain > flatGain) {
		t.Fatalf("expected flat<1<steep, got flat=%v steep=%v", flatGain, steepGain)
	}

	// NoiseWeight=0 时档案不参与学习。
	cfg.NoiseWeight = 0
	if gain := noiseDownGain(3500, cfg); gain != 1 {
		t.Fatalf("NoiseWeight=0 should disable noise gain, got %v", gain)
	}

	// 无档案时增益为 1。
	if gain := noiseDownGain(3500, types.SmartControlConfig{NoiseWeight: 4}); gain != 1 {
		t.Fatalf("missing profile should yield neutral gain, got %v", gain)
	}
}

func TestSanitizeNoiseProfileSortsAndNormalizes(t *testing.T) {
	profile := []types.NoiseProfilePoint{
		{RPM: 3000, DB: -30},
		{RPM: 1000, DB: -42},
		{RPM: 2000, DB: -38},
		{RPM: 2000, DB: -37.5},
		{RPM: -50, DB: 1},
	}
	sanitized, changed := sanitizeNoiseProfile(profile)
	if !changed {
		t.Fatalf("expected sanitize to report change")
	}
	if len(sanitized) != 3 {
		t.Fatalf("expected 3 points after cleanup, got %v", sanitized)
	}
	if sanitized[0].RPM != 1000 || sanitized[2].RPM != 3000 {
		t.Fatalf("expected ascending RPM order, got %v", sanitized)
	}
	if sanitized[0].DB != 0 {
		t.Fatalf("expected min DB shifted to 0, got %v", sanitized)
	}
	// 已归一化的档案再次清洗应保持不变。
	if again, changedAgain := sanitizeNoiseProfile(sanitized); changedAgain {
		t.Fatalf("sanitize should be idempotent, got %v", again)
	}
}
