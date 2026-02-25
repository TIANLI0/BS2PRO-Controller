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

	if cfg.RampDownLimit > cfg.RampUpLimit+300 {
		cfg.RampDownLimit = cfg.RampUpLimit + 300
		changed = true
	}

	return cfg, changed
}

// CalculateTargetRPM 计算智能目标转速
func CalculateTargetRPM(avgTemp, lastAvgTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	effectiveCurve := make([]types.FanCurvePoint, len(curve))
	for i, point := range curve {
		offset := 0
		if i < len(cfg.LearnedOffsets) {
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
		targetRPM += tempDelta * (8 + cfg.Aggressiveness*3)
	}

	if avgTemp >= cfg.TargetTemp+15 {
		targetRPM += 350
	}

	return clampInt(targetRPM, 1000, 4000)
}

// LearnCurveOffsets 学习并更新曲线偏移
func LearnCurveOffsets(avgTemp, lastAvgTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) ([]int, bool) {
	if len(curve) == 0 {
		return cfg.LearnedOffsets, false
	}

	offsets := make([]int, len(curve))
	copy(offsets, cfg.LearnedOffsets)

	idx := nearestCurveIndex(avgTemp, curve)
	errorTemp := avgTemp - cfg.TargetTemp
	tempDelta := avgTemp - lastAvgTemp

	learnGain := 2 + cfg.LearnRate*2
	delta := errorTemp * learnGain

	if tempDelta > 0 {
		delta += tempDelta * (1 + cfg.LearnRate)
	}

	if absInt(delta) < 6 {
		return offsets, false
	}

	delta = clampInt(delta, -40, 80)

	changed := false
	newMain := clampInt(offsets[idx]+delta, -cfg.MaxLearnOffset, cfg.MaxLearnOffset)
	if newMain != offsets[idx] {
		offsets[idx] = newMain
		changed = true
	}

	neighborDelta := delta / 2
	if neighborDelta != 0 {
		if idx > 0 {
			newLeft := clampInt(offsets[idx-1]+neighborDelta, -cfg.MaxLearnOffset, cfg.MaxLearnOffset)
			if newLeft != offsets[idx-1] {
				offsets[idx-1] = newLeft
				changed = true
			}
		}
		if idx < len(offsets)-1 {
			newRight := clampInt(offsets[idx+1]+neighborDelta, -cfg.MaxLearnOffset, cfg.MaxLearnOffset)
			if newRight != offsets[idx+1] {
				offsets[idx+1] = newRight
				changed = true
			}
		}
	}

	return offsets, changed
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
