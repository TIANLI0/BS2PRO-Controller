package smartcontrol

import "github.com/TIANLI0/BS2PRO-Controller/internal/types"

// NormalizeConfig 归一化智能控温配置
func NormalizeConfig(cfg types.SmartControlConfig, curve []types.FanCurvePoint) (types.SmartControlConfig, bool) {
	defaults := types.GetDefaultSmartControlConfig(curve)
	changed := false

	if !cfg.Learning {
		cfg.Learning = true
		changed = true
	}

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
	if sanitized, updated := constrainOffsetsToCurveBounds(cfg.LearnedOffsetsHeat, curve, cfg.MaxLearnOffset); updated {
		cfg.LearnedOffsetsHeat = sanitized
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
	if sanitized, updated := constrainOffsetsToCurveBounds(cfg.LearnedOffsetsCool, curve, cfg.MaxLearnOffset); updated {
		cfg.LearnedOffsetsCool = sanitized
		changed = true
	}

	if normalized, updated := normalizeRateBiases(cfg.LearnedRateHeat, cfg.MaxLearnOffset); updated {
		cfg.LearnedRateHeat = normalized
		changed = true
	}
	if normalized, updated := normalizeRateBiases(cfg.LearnedRateCool, cfg.MaxLearnOffset); updated {
		cfg.LearnedRateCool = normalized
		changed = true
	}

	if cfg.RampDownLimit > cfg.RampUpLimit+300 {
		cfg.RampDownLimit = cfg.RampUpLimit + 300
		changed = true
	}

	blended := BlendOffsets(cfg.LearnedOffsetsHeat, cfg.LearnedOffsetsCool)
	if sanitized, updated := constrainOffsetsToCurveBounds(blended, curve, cfg.MaxLearnOffset); updated {
		blended = sanitized
		changed = true
	}
	if !intSlicesEqual(blended, cfg.LearnedOffsets) {
		cfg.LearnedOffsets = blended
		changed = true
	}

	return cfg, changed
}

// BlendOffsets 将升温/降温偏移融合为兼容视图
func BlendOffsets(heatOffsets, coolOffsets []int) []int {
	if len(heatOffsets) == 0 && len(coolOffsets) == 0 {
		return nil
	}

	size := max(len(coolOffsets), len(heatOffsets))
	blended := make([]int, size)
	for i := range size {
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
