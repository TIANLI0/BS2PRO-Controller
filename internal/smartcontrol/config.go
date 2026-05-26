package smartcontrol

import "github.com/TIANLI0/BS2PRO-Controller/internal/types"

// NormalizeConfig 归一化智能控温配置。
func NormalizeConfig(cfg types.SmartControlConfig, curve []types.FanCurvePoint, _ bool) (types.SmartControlConfig, bool) {
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
	if normalizedBias := types.NormalizeLearningBias(cfg.LearningBias); normalizedBias != cfg.LearningBias {
		cfg.LearningBias = normalizedBias
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
		next := make([]int, len(curve))
		copy(next, cfg.LearnedOffsets)
		cfg.LearnedOffsets = next
		changed = true
	}
	if sanitized, updated := constrainOffsetsToCurveBounds(cfg.LearnedOffsets, curve, cfg.MaxLearnOffset); updated {
		cfg.LearnedOffsets = sanitized
		changed = true
	}
	if sanitized, updated := constrainOffsetsToLearningBias(cfg.LearnedOffsets, cfg.LearningBias); updated {
		cfg.LearnedOffsets = sanitized
		changed = true
	}

	if len(cfg.LearnedOffsetsHeat) != len(curve) {
		next := make([]int, len(curve))
		copy(next, cfg.LearnedOffsetsHeat)
		cfg.LearnedOffsetsHeat = next
		changed = true
	}
	if len(cfg.LearnedOffsetsCool) != len(curve) {
		next := make([]int, len(curve))
		copy(next, cfg.LearnedOffsetsCool)
		cfg.LearnedOffsetsCool = next
		changed = true
	}

	rateLen := rateBucketMax - rateBucketMin + 1
	if len(cfg.LearnedRateHeat) != rateLen {
		next := make([]int, rateLen)
		copy(next, cfg.LearnedRateHeat)
		cfg.LearnedRateHeat = next
		changed = true
	}
	if len(cfg.LearnedRateCool) != rateLen {
		next := make([]int, rateLen)
		copy(next, cfg.LearnedRateCool)
		cfg.LearnedRateCool = next
		changed = true
	}

	if cfg.RampDownLimit > cfg.RampUpLimit+300 {
		cfg.RampDownLimit = cfg.RampUpLimit + 300
		changed = true
	}

	return cfg, changed
}

// BlendOffsets 保留旧接口所需的 Heat/Cool 融合行为。
func BlendOffsets(heatOffsets, coolOffsets []int) []int {
	if len(heatOffsets) == 0 && len(coolOffsets) == 0 {
		return nil
	}
	size := max(len(coolOffsets), len(heatOffsets))
	out := make([]int, size)
	for i := range size {
		h, c := 0, 0
		if i < len(heatOffsets) {
			h = heatOffsets[i]
		}
		if i < len(coolOffsets) {
			c = coolOffsets[i]
		}
		out[i] = (h + c) / 2
	}
	return out
}
