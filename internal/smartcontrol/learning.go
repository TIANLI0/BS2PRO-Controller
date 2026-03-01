package smartcontrol

import "github.com/TIANLI0/BS2PRO-Controller/internal/types"

// LearnCurveOffsets 学习并更新曲线偏移
func LearnCurveOffsets(avgTemp, lastAvgTemp, targetRPM, lastTargetRPM int, recentAvgTemps []int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) ([]int, []int, []int, []int, bool) {
	if len(curve) == 0 {
		rateHeat, _ := normalizeRateBiases(cfg.LearnedRateHeat, cfg.MaxLearnOffset)
		rateCool, _ := normalizeRateBiases(cfg.LearnedRateCool, cfg.MaxLearnOffset)
		return cfg.LearnedOffsetsHeat, cfg.LearnedOffsetsCool, rateHeat, rateCool, false
	}

	heatOffsets := make([]int, len(curve))
	coolOffsets := make([]int, len(curve))
	copy(heatOffsets, cfg.LearnedOffsetsHeat)
	copy(coolOffsets, cfg.LearnedOffsetsCool)

	if len(heatOffsets) != len(curve) {
		heatOffsets = make([]int, len(curve))
		copy(heatOffsets, cfg.LearnedOffsets)
	}
	if len(coolOffsets) != len(curve) {
		coolOffsets = make([]int, len(curve))
		copy(coolOffsets, cfg.LearnedOffsets)
	}

	rateHeat, _ := normalizeRateBiases(cfg.LearnedRateHeat, cfg.MaxLearnOffset)
	rateCool, _ := normalizeRateBiases(cfg.LearnedRateCool, cfg.MaxLearnOffset)

	learningWindow := max(3, cfg.LearnWindow)
	learningDelay := max(1, cfg.LearnDelay)
	minRequired := learningWindow + learningDelay
	if len(recentAvgTemps) < minRequired {
		return heatOffsets, coolOffsets, rateHeat, rateCool, false
	}

	windowStart := len(recentAvgTemps) - minRequired
	windowEnd := windowStart + learningWindow
	learningWindowTemps := recentAvgTemps[windowStart:windowEnd]
	if !isStableLearningWindow(learningWindowTemps, cfg.Hysteresis+1) {
		overheatMargin := cfg.TargetTemp + cfg.Hysteresis + 3
		if avgTemp < overheatMargin {
			return heatOffsets, coolOffsets, rateHeat, rateCool, false
		}
	}

	learnTemp := recentAvgTemps[len(recentAvgTemps)-learningDelay]
	learnPrevTemp := recentAvgTemps[len(recentAvgTemps)-learningDelay-1]
	learnTempDelta := learnTemp - learnPrevTemp

	idx := nearestCurveIndex(learnTemp, curve)
	errorTemp := avgTemp - cfg.TargetTemp
	tempDelta := avgTemp - lastAvgTemp
	overheat := max(0, avgTemp-(cfg.TargetTemp+cfg.Hysteresis))
	rpmDelta := absInt(targetRPM - lastTargetRPM)
	noise := max(0, targetRPM-2800)

	tempTerm := errorTemp * (4 + cfg.LearnRate)
	overheatTerm := overheat * (2 + cfg.OverheatWeight)
	trendTerm := tempDelta * (2 + cfg.TrendGain)

	changePenalty := (rpmDelta / max(30, cfg.MinRPMChange)) * (2 + cfg.RPMDeltaWeight)
	noisePenalty := (noise / 180) * cfg.NoiseWeight

	raw := tempTerm + overheatTerm + trendTerm - changePenalty - noisePenalty

	if tempDelta > 0 {
		preheatBand := cfg.Hysteresis + 4
		distanceToTarget := cfg.TargetTemp - avgTemp
		if distanceToTarget >= 0 && distanceToTarget <= preheatBand {
			raw += (preheatBand - distanceToTarget) * (1 + cfg.TrendGain/2)
		}
	}

	if learnTempDelta > 0 {
		raw += learnTempDelta * (2 + cfg.TrendGain)
	}
	if learnTempDelta < 0 {
		raw += learnTempDelta * max(1, cfg.TrendGain/2)
	}

	if errorTemp < -cfg.Hysteresis-1 && tempDelta <= 0 {
		raw -= 3 + cfg.NoiseWeight
	}

	lowRpmDeltaBand := max(20, cfg.MinRPMChange/2)
	if tempDelta > 0 && rpmDelta <= lowRpmDeltaBand && errorTemp <= cfg.Hysteresis+2 {
		raw -= 4 + cfg.NoiseWeight/2
	}

	if absInt(raw) < 4 {
		return heatOffsets, coolOffsets, rateHeat, rateCool, false
	}

	// 将评分压缩为小步进，避免学习曲线过于陡峭。
	denominator := max(10, 24-cfg.LearnRate*2)
	delta := raw / denominator
	if delta == 0 {
		delta = signInt(raw)
	}
	delta = clampInt(delta, -4, 6)

	activeOffsets := &coolOffsets
	passiveOffsets := &heatOffsets
	activeRate := &rateCool
	passiveRate := &rateHeat
	if tempDelta >= 0 {
		activeOffsets = &heatOffsets
		passiveOffsets = &coolOffsets
		activeRate = &rateHeat
		passiveRate = &rateCool
	}
	rateIdx := rateBucketIndex(tempDelta)

	changed := false
	if applyDeltaAtIndex(*activeOffsets, idx, delta, curve, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx-1, scaledDelta(delta, 2, 3), curve, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx+1, scaledDelta(delta, 2, 3), curve, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx-2, scaledDelta(delta, 1, 3), curve, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx+2, scaledDelta(delta, 1, 3), curve, cfg.MaxLearnOffset) {
		changed = true
	}

	if applyDeltaAtIndex(*passiveOffsets, idx, scaledDelta(delta, 1, 8), curve, cfg.MaxLearnOffset) {
		changed = true
	}

	rateDenominator := max(14, 28-cfg.LearnRate*2)
	rateDelta := raw / rateDenominator
	if rateDelta == 0 {
		rateDelta = signInt(raw)
	}
	rateDelta = clampInt(rateDelta, -3, 4)

	if applyRateBiasDeltaAtIndex(*activeRate, rateIdx, rateDelta, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyRateBiasDeltaAtIndex(*activeRate, rateIdx-1, scaledDelta(rateDelta, 2, 3), cfg.MaxLearnOffset) {
		changed = true
	}
	if applyRateBiasDeltaAtIndex(*activeRate, rateIdx+1, scaledDelta(rateDelta, 2, 3), cfg.MaxLearnOffset) {
		changed = true
	}
	if applyRateBiasDeltaAtIndex(*passiveRate, rateIdx, scaledDelta(rateDelta, 1, 8), cfg.MaxLearnOffset) {
		changed = true
	}

	if smoothAndClampOffsets(*activeOffsets, curve, cfg.MaxLearnOffset) {
		changed = true
	}
	if smoothAndClampOffsets(*passiveOffsets, curve, cfg.MaxLearnOffset) {
		changed = true
	}
	if smoothRateBiases(*activeRate, cfg.MaxLearnOffset) {
		changed = true
	}
	if smoothRateBiases(*passiveRate, cfg.MaxLearnOffset) {
		changed = true
	}

	return heatOffsets, coolOffsets, rateHeat, rateCool, changed
}

func rateBucketCount() int {
	return rateBucketMax - rateBucketMin + 1
}

func rateBucketIndex(tempDelta int) int {
	clamped := clampInt(tempDelta, rateBucketMin, rateBucketMax)
	return clamped - rateBucketMin
}

func clampRateBias(value, maxLearnOffset int) int {
	cap := min(max(80, maxLearnOffset/2), 600)
	return clampInt(value, -cap, cap)
}

func normalizeRateBiases(rateBiases []int, maxLearnOffset int) ([]int, bool) {
	needLen := rateBucketCount()
	normalized := make([]int, needLen)
	changed := false

	if len(rateBiases) != needLen {
		changed = true
	}
	copy(normalized, rateBiases)

	for i := range normalized {
		clamped := clampRateBias(normalized[i], maxLearnOffset)
		if clamped != normalized[i] {
			normalized[i] = clamped
			changed = true
		}
	}

	return normalized, changed
}

func applyDeltaAtIndex(offsets []int, idx, delta int, curve []types.FanCurvePoint, maxLearnOffset int) bool {
	if delta == 0 || idx < 0 || idx >= len(offsets) {
		return false
	}
	if idx >= len(curve) {
		return false
	}
	leftMinRPM, rightMaxRPM := getCurveEdgeRPMBounds(curve)
	newValue := clampOffsetForPoint(offsets[idx]+delta, curve[idx].RPM, leftMinRPM, rightMaxRPM, maxLearnOffset)
	if newValue == offsets[idx] {
		return false
	}
	offsets[idx] = newValue
	return true
}

func applyRateBiasDeltaAtIndex(rateBiases []int, idx, delta, maxLearnOffset int) bool {
	if delta == 0 || idx < 0 || idx >= len(rateBiases) {
		return false
	}
	newValue := clampRateBias(rateBiases[idx]+delta, maxLearnOffset)
	if newValue == rateBiases[idx] {
		return false
	}
	rateBiases[idx] = newValue
	return true
}

func scaledDelta(delta, numerator, denominator int) int {
	if delta == 0 || denominator <= 0 {
		return 0
	}
	absDelta := absInt(delta)
	scaled := (absDelta*numerator + denominator - 1) / denominator
	if scaled == 0 {
		scaled = 1
	}
	if delta < 0 {
		return -scaled
	}
	return scaled
}

func signInt(value int) int {
	if value > 0 {
		return 1
	}
	if value < 0 {
		return -1
	}
	return 0
}

func smoothAndClampOffsets(offsets []int, curve []types.FanCurvePoint, maxLearnOffset int) bool {
	if len(offsets) == 0 || len(curve) == 0 {
		return false
	}
	changed := false
	leftMinRPM, rightMaxRPM := getCurveEdgeRPMBounds(curve)

	smoothed := make([]int, len(offsets))
	for i := range offsets {
		weighted := offsets[i] * 5
		weight := 5
		if i > 0 {
			weighted += offsets[i-1]
			weight++
		}
		if i+1 < len(offsets) {
			weighted += offsets[i+1]
			weight++
		}
		smoothed[i] = weighted / weight
	}

	maxJump := min(max(20, maxLearnOffset/10), 90)

	for i := range offsets {
		candidate := smoothed[i]
		if i > 0 {
			candidate = clampInt(candidate, offsets[i-1]-maxJump, offsets[i-1]+maxJump)
		}
		candidate = clampOffsetForPoint(candidate, curve[i].RPM, leftMinRPM, rightMaxRPM, maxLearnOffset)
		if candidate != offsets[i] {
			offsets[i] = candidate
			changed = true
		}
	}

	return changed
}

func smoothRateBiases(rateBiases []int, maxLearnOffset int) bool {
	if len(rateBiases) == 0 {
		return false
	}

	changed := false
	smoothed := make([]int, len(rateBiases))
	for i := range rateBiases {
		weighted := rateBiases[i] * 4
		weight := 4
		if i > 0 {
			weighted += rateBiases[i-1] * 2
			weight += 2
		}
		if i+1 < len(rateBiases) {
			weighted += rateBiases[i+1] * 2
			weight += 2
		}
		smoothed[i] = weighted / weight
	}

	maxJump := min(max(12, maxLearnOffset/20), 45)
	for i := range rateBiases {
		candidate := clampRateBias(smoothed[i], maxLearnOffset)
		if i > 0 {
			candidate = clampInt(candidate, rateBiases[i-1]-maxJump, rateBiases[i-1]+maxJump)
		}
		if candidate != rateBiases[i] {
			rateBiases[i] = candidate
			changed = true
		}
	}

	return changed
}

func isStableLearningWindow(temps []int, allowedRange int) bool {
	if len(temps) == 0 {
		return false
	}
	maxTemp := temps[0]
	minTemp := temps[0]
	for i := 1; i < len(temps); i++ {
		if temps[i] > maxTemp {
			maxTemp = temps[i]
		}
		if temps[i] < minTemp {
			minTemp = temps[i]
		}
	}

	return maxTemp-minTemp <= max(2, allowedRange)
}
