// Package smartcontrol 提供学习型智能控温算法
package smartcontrol

import (
	"github.com/TIANLI0/BS2PRO-Controller/internal/temperature"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// NormalizeConfig 归一化智能控温配置
func NormalizeConfig(cfg types.SmartControlConfig, curve []types.FanCurvePoint) (types.SmartControlConfig, bool) {
	defaults := types.GetDefaultSmartControlConfig(curve)
	changed := false

	if cfg.TargetTemp < 45 || cfg.TargetTemp > 90 {
		cfg.TargetTemp = defaults.TargetTemp
		changed = true
	}
	if cfg.Aggressiveness < 1 || cfg.Aggressiveness > 10 {
		cfg.Aggressiveness = defaults.Aggressiveness
		changed = true
	}
	if cfg.Hysteresis < 0 || cfg.Hysteresis > 8 {
		cfg.Hysteresis = defaults.Hysteresis
		changed = true
	}
	if cfg.MinRPMChange < 20 || cfg.MinRPMChange > 400 {
		cfg.MinRPMChange = defaults.MinRPMChange
		changed = true
	}
	if cfg.RampUpLimit < 50 || cfg.RampUpLimit > 1200 {
		cfg.RampUpLimit = defaults.RampUpLimit
		changed = true
	}
	if cfg.RampDownLimit < 50 || cfg.RampDownLimit > 1200 {
		cfg.RampDownLimit = defaults.RampDownLimit
		changed = true
	}
	if cfg.LearnRate < 1 || cfg.LearnRate > 10 {
		cfg.LearnRate = defaults.LearnRate
		changed = true
	}
	if cfg.LearnWindow < 3 || cfg.LearnWindow > 24 {
		cfg.LearnWindow = defaults.LearnWindow
		changed = true
	}
	if cfg.LearnDelay < 1 || cfg.LearnDelay > 8 {
		cfg.LearnDelay = defaults.LearnDelay
		changed = true
	}
	if cfg.OverheatWeight < 1 || cfg.OverheatWeight > 12 {
		cfg.OverheatWeight = defaults.OverheatWeight
		changed = true
	}
	if cfg.RPMDeltaWeight < 1 || cfg.RPMDeltaWeight > 12 {
		cfg.RPMDeltaWeight = defaults.RPMDeltaWeight
		changed = true
	}
	if cfg.NoiseWeight < 0 || cfg.NoiseWeight > 12 {
		cfg.NoiseWeight = defaults.NoiseWeight
		changed = true
	}
	if cfg.TrendGain < 1 || cfg.TrendGain > 12 {
		cfg.TrendGain = defaults.TrendGain
		changed = true
	}
	if cfg.MaxLearnOffset < 100 || cfg.MaxLearnOffset > 2000 {
		cfg.MaxLearnOffset = defaults.MaxLearnOffset
		changed = true
	}

	if len(cfg.LearnedOffsets) != len(curve) {
		newOffsets := make([]int, len(curve))
		copy(newOffsets, cfg.LearnedOffsets)
		cfg.LearnedOffsets = newOffsets
		changed = true
	}

	if len(cfg.LearnedOffsetsHeat) != len(curve) {
		newHeatOffsets := make([]int, len(curve))
		if len(cfg.LearnedOffsetsHeat) > 0 {
			copy(newHeatOffsets, cfg.LearnedOffsetsHeat)
		} else {
			copy(newHeatOffsets, cfg.LearnedOffsets)
		}
		cfg.LearnedOffsetsHeat = newHeatOffsets
		changed = true
	}

	if len(cfg.LearnedOffsetsCool) != len(curve) {
		newCoolOffsets := make([]int, len(curve))
		if len(cfg.LearnedOffsetsCool) > 0 {
			copy(newCoolOffsets, cfg.LearnedOffsetsCool)
		} else {
			copy(newCoolOffsets, cfg.LearnedOffsets)
		}
		cfg.LearnedOffsetsCool = newCoolOffsets
		changed = true
	}

	if cfg.RampDownLimit > cfg.RampUpLimit+300 {
		cfg.RampDownLimit = cfg.RampUpLimit + 300
		changed = true
	}

	blended := BlendOffsets(cfg.LearnedOffsetsHeat, cfg.LearnedOffsetsCool)
	if !intSlicesEqual(blended, cfg.LearnedOffsets) {
		cfg.LearnedOffsets = blended
		changed = true
	}

	return cfg, changed
}

// CalculateTargetRPM 计算智能目标转速
func CalculateTargetRPM(avgTemp, lastAvgTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	effectiveCurve := make([]types.FanCurvePoint, len(curve))
	activeOffsets := selectOffsetsForTrend(avgTemp-lastAvgTemp, cfg)
	for i, point := range curve {
		offset := 0
		if i < len(activeOffsets) {
			offset = activeOffsets[i]
		} else if i < len(cfg.LearnedOffsets) {
			offset = cfg.LearnedOffsets[i]
		}
		effectiveCurve[i] = types.FanCurvePoint{
			Temperature: point.Temperature,
			RPM:         clampInt(point.RPM+offset, 1000, 4000),
		}
	}
	enforceNonDecreasingRPM(effectiveCurve)

	targetRPM := temperature.CalculateTargetRPM(avgTemp, effectiveCurve)
	if targetRPM <= 0 {
		return 0
	}

	tempError := avgTemp - cfg.TargetTemp
	if absInt(tempError) > cfg.Hysteresis {
		gain := 12 + cfg.Aggressiveness*4
		targetRPM += tempError * gain
	}

	tempDelta := avgTemp - lastAvgTemp
	if tempDelta > 0 {
		targetRPM += tempDelta * (6 + cfg.Aggressiveness*2 + cfg.TrendGain*2)
	}
	if tempDelta < 0 {
		targetRPM += tempDelta * (1 + cfg.TrendGain/2)
	}

	if avgTemp >= cfg.TargetTemp+15 {
		targetRPM += 320 + cfg.OverheatWeight*15
	}

	return clampInt(targetRPM, 1000, 4000)
}

// LearnCurveOffsets 学习并更新曲线偏移
func LearnCurveOffsets(avgTemp, lastAvgTemp, targetRPM, lastTargetRPM int, recentAvgTemps []int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) ([]int, []int, bool) {
	if len(curve) == 0 {
		return cfg.LearnedOffsetsHeat, cfg.LearnedOffsetsCool, false
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

	learningWindow := max(3, cfg.LearnWindow)
	learningDelay := max(1, cfg.LearnDelay)
	minRequired := learningWindow + learningDelay
	if len(recentAvgTemps) < minRequired {
		return heatOffsets, coolOffsets, false
	}

	windowStart := len(recentAvgTemps) - minRequired
	windowEnd := windowStart + learningWindow
	learningWindowTemps := recentAvgTemps[windowStart:windowEnd]
	if !isStableLearningWindow(learningWindowTemps, cfg.Hysteresis+1) {
		overheatMargin := cfg.TargetTemp + cfg.Hysteresis + 3
		if avgTemp < overheatMargin {
			return heatOffsets, coolOffsets, false
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

	tempTerm := errorTemp * (2 + cfg.LearnRate)
	overheatTerm := overheat * (1 + cfg.OverheatWeight)
	trendTerm := tempDelta * (1 + cfg.TrendGain)

	changePenalty := (rpmDelta / max(20, cfg.MinRPMChange/2)) * cfg.RPMDeltaWeight
	noisePenalty := (noise / 150) * cfg.NoiseWeight

	delta := tempTerm + overheatTerm + trendTerm - changePenalty - noisePenalty

	if learnTempDelta > 0 {
		delta += learnTempDelta * (1 + cfg.TrendGain)
	}
	if learnTempDelta < 0 {
		delta += learnTempDelta * max(1, cfg.TrendGain/2)
	}

	if errorTemp < -cfg.Hysteresis-1 && tempDelta <= 0 {
		delta -= 2 + cfg.NoiseWeight/2
	}

	if absInt(delta) < 4 {
		return heatOffsets, coolOffsets, false
	}

	delta = clampInt(delta, -35, 60)

	activeOffsets := &coolOffsets
	passiveOffsets := &heatOffsets
	if tempDelta >= 0 {
		activeOffsets = &heatOffsets
		passiveOffsets = &coolOffsets
	}

	changed := false
	if applyDeltaAtIndex(*activeOffsets, idx, delta, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx-1, delta/2, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx+1, delta/2, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx-2, delta/4, cfg.MaxLearnOffset) {
		changed = true
	}
	if applyDeltaAtIndex(*activeOffsets, idx+2, delta/4, cfg.MaxLearnOffset) {
		changed = true
	}

	if applyDeltaAtIndex(*passiveOffsets, idx, delta/5, cfg.MaxLearnOffset) {
		changed = true
	}

	return heatOffsets, coolOffsets, changed
}

// BlendOffsets 将升温/降温偏移融合为兼容视图
func BlendOffsets(heatOffsets, coolOffsets []int) []int {
	if len(heatOffsets) == 0 && len(coolOffsets) == 0 {
		return nil
	}

	size := max(len(coolOffsets), len(heatOffsets))
	blended := make([]int, size)
	for i := 0; i < size; i++ {
		heat := 0
		if i < len(heatOffsets) {
			heat = heatOffsets[i]
		}
		cool := 0
		if i < len(coolOffsets) {
			cool = coolOffsets[i]
		}
		blended[i] = (heat + cool) / 2
	}

	return blended
}

func selectOffsetsForTrend(tempDelta int, cfg types.SmartControlConfig) []int {
	if tempDelta > 0 && len(cfg.LearnedOffsetsHeat) > 0 {
		return cfg.LearnedOffsetsHeat
	}
	if tempDelta < 0 && len(cfg.LearnedOffsetsCool) > 0 {
		return cfg.LearnedOffsetsCool
	}
	if len(cfg.LearnedOffsetsHeat) > 0 && len(cfg.LearnedOffsetsCool) > 0 {
		return BlendOffsets(cfg.LearnedOffsetsHeat, cfg.LearnedOffsetsCool)
	}
	return cfg.LearnedOffsets
}

func applyDeltaAtIndex(offsets []int, idx, delta, maxLearnOffset int) bool {
	if delta == 0 || idx < 0 || idx >= len(offsets) {
		return false
	}
	newValue := clampInt(offsets[idx]+delta, -maxLearnOffset, maxLearnOffset)
	if newValue == offsets[idx] {
		return false
	}
	offsets[idx] = newValue
	return true
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

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ApplyRampLimit 应用升降速限幅
func ApplyRampLimit(targetRPM, lastRPM, upLimit, downLimit int) int {
	if targetRPM > lastRPM {
		return min(lastRPM+upLimit, targetRPM)
	}
	if targetRPM < lastRPM {
		return max(lastRPM-downLimit, targetRPM)
	}
	return targetRPM
}

func nearestCurveIndex(temp int, curve []types.FanCurvePoint) int {
	if len(curve) == 0 {
		return 0
	}

	idx := 0
	bestDistance := absInt(curve[0].Temperature - temp)
	for i := 1; i < len(curve); i++ {
		distance := absInt(curve[i].Temperature - temp)
		if distance < bestDistance {
			bestDistance = distance
			idx = i
		}
	}

	return idx
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func enforceNonDecreasingRPM(curve []types.FanCurvePoint) {
	for i := 1; i < len(curve); i++ {
		if curve[i].RPM < curve[i-1].RPM {
			curve[i].RPM = curve[i-1].RPM
		}
	}
}
