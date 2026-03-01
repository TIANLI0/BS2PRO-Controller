package smartcontrol

import (
	"github.com/TIANLI0/BS2PRO-Controller/internal/temperature"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// CalculateTargetRPM 计算智能目标转速
func CalculateTargetRPM(avgTemp, lastAvgTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	effectiveCurve := make([]types.FanCurvePoint, len(curve))
	activeOffsets := selectOffsetsForTrend(avgTemp-lastAvgTemp, cfg)
	leftMinRPM, rightMaxRPM := getCurveEdgeRPMBounds(curve)
	for i, point := range curve {
		offset := 0
		if i < len(activeOffsets) {
			offset = activeOffsets[i]
		} else if i < len(cfg.LearnedOffsets) {
			offset = cfg.LearnedOffsets[i]
		}
		offset = clampOffsetForPoint(offset, point.RPM, leftMinRPM, rightMaxRPM, cfg.MaxLearnOffset)
		effectiveCurve[i] = types.FanCurvePoint{
			Temperature: point.Temperature,
			RPM:         clampInt(point.RPM+offset, leftMinRPM, rightMaxRPM),
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
	targetRPM += sampleRateBias(tempDelta, cfg)

	if tempDelta > 0 {
		preheatBand := cfg.Hysteresis + 4 + cfg.TrendGain/2
		distanceToTarget := cfg.TargetTemp - avgTemp
		if distanceToTarget >= 0 && distanceToTarget <= preheatBand {
			preemptiveBoost := (preheatBand - distanceToTarget) * (4 + cfg.Aggressiveness + cfg.TrendGain)
			targetRPM += preemptiveBoost
		}
		targetRPM += tempDelta * (8 + cfg.Aggressiveness*2 + cfg.TrendGain*3)
	}
	if tempDelta < 0 {
		targetRPM += tempDelta * (1 + cfg.TrendGain/3)
	}

	if avgTemp >= cfg.TargetTemp+15 {
		targetRPM += 320 + cfg.OverheatWeight*15
	}

	return clampInt(targetRPM, 0, 4000)
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

func selectRateBiasesForTrend(tempDelta int, cfg types.SmartControlConfig) []int {
	if tempDelta > 0 && len(cfg.LearnedRateHeat) > 0 {
		return cfg.LearnedRateHeat
	}
	if tempDelta < 0 && len(cfg.LearnedRateCool) > 0 {
		return cfg.LearnedRateCool
	}
	if len(cfg.LearnedRateHeat) > 0 && len(cfg.LearnedRateCool) > 0 {
		blended := make([]int, rateBucketCount())
		for i := range blended {
			blended[i] = (cfg.LearnedRateHeat[i] + cfg.LearnedRateCool[i]) / 2
		}
		return blended
	}
	return nil
}

func sampleRateBias(tempDelta int, cfg types.SmartControlConfig) int {
	rateBiases := selectRateBiasesForTrend(tempDelta, cfg)
	if len(rateBiases) != rateBucketCount() {
		return 0
	}
	return rateBiases[rateBucketIndex(tempDelta)]
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
