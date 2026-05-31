package smartcontrol

import (
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

// CalculateTargetRPM 以基础曲线加学习偏移计算目标转速。
func CalculateTargetRPM(currentTemp int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) int {
	if len(curve) == 0 {
		return 0
	}

	offsets := cfg.LearnedOffsets
	if !cfg.Learning {
		offsets = nil
	} else if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}
	effectiveCurve := buildEffectiveCurve(curve, offsets, effectiveOffsetCap(cfg))
	rpm := temperature.CalculateTargetRPM(currentTemp, effectiveCurve)
	if rpm <= 0 {
		return 0
	}

	leftMin, rightMax := GetCurveRPMBounds(effectiveCurve)
	return clampInt(rpm, leftMin, rightMax)
}

// buildEffectiveCurve 把基础曲线与学习偏移合成有效曲线。
func buildEffectiveCurve(curve []types.FanCurvePoint, offsets []int, cap int) []types.FanCurvePoint {
	out := make([]types.FanCurvePoint, len(curve))
	leftMin, rightMax := GetCurveRPMBounds(curve)
	for i, p := range curve {
		off := 0
		if i < len(offsets) {
			off = offsets[i]
		}
		off = clampOffsetForPoint(off, p.RPM, leftMin, rightMax, cap)
		out[i] = types.FanCurvePoint{
			Temperature: p.Temperature,
			RPM:         clampInt(p.RPM+off, leftMin, rightMax),
		}
	}
	enforceNonDecreasingRPM(out)
	return out
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
