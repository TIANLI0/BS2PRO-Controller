package coreapp

import "github.com/TIANLI0/THRM/internal/types"

func cloneIntSlice(input []int) []int {
	if len(input) == 0 {
		return nil
	}
	out := make([]int, len(input))
	copy(out, input)
	return out
}

func syncSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.SmartControl.LearnedOffsetsByProfile == nil {
		cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{}
	}
	activeID := cfg.ActiveFanCurveProfileID
	if activeID == "" {
		return false
	}

	changed := false
	if _, ok := cfg.SmartControl.LearnedOffsetsByProfile[activeID]; !ok {
		if cfg.SmartControl.LearnedOffsets != nil {
			cfg.SmartControl.LearnedOffsetsByProfile[activeID] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
			changed = true
		} else {
			cfg.SmartControl.LearnedOffsetsByProfile[activeID] = make([]int, len(cfg.FanCurve))
			changed = true
		}
	}

	loaded := cfg.SmartControl.LearnedOffsetsByProfile[activeID]
	if loaded == nil {
		loaded = make([]int, len(cfg.FanCurve))
		cfg.SmartControl.LearnedOffsetsByProfile[activeID] = loaded
		changed = true
	}
	if cfg.SmartControl.LearnedOffsets == nil || len(cfg.SmartControl.LearnedOffsets) != len(loaded) {
		cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
		changed = true
	} else {
		for i := range loaded {
			if cfg.SmartControl.LearnedOffsets[i] != loaded[i] {
				cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
				changed = true
				break
			}
		}
	}
	return changed
}

func storeSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	activeID := cfg.ActiveFanCurveProfileID
	if activeID == "" {
		return false
	}
	if cfg.SmartControl.LearnedOffsetsByProfile == nil {
		cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{}
	}
	cfg.SmartControl.LearnedOffsetsByProfile[activeID] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
	storeSmartControlPrefsForActiveProfile(cfg)
	return true
}

// storeSmartControlPrefsForActiveProfile 把当前学习偏好（目标温度/学习倾向）
// 记到当前曲线方案名下。噪音校准是全局的，不在此列。
func storeSmartControlPrefsForActiveProfile(cfg *types.AppConfig) {
	activeID := cfg.ActiveFanCurveProfileID
	if activeID == "" {
		return
	}
	if cfg.SmartControl.TargetTempByProfile == nil {
		cfg.SmartControl.TargetTempByProfile = map[string]int{}
	}
	if cfg.SmartControl.LearningBiasByProfile == nil {
		cfg.SmartControl.LearningBiasByProfile = map[string]string{}
	}
	cfg.SmartControl.TargetTempByProfile[activeID] = cfg.SmartControl.TargetTemp
	cfg.SmartControl.LearningBiasByProfile[activeID] = types.NormalizeLearningBias(cfg.SmartControl.LearningBias)
}

// loadSmartControlPrefsForActiveProfile 切换曲线方案后，恢复该方案记忆的
// 目标温度与学习倾向；没有记忆时保持当前值（随后由 store 落表）。
func loadSmartControlPrefsForActiveProfile(cfg *types.AppConfig) {
	activeID := cfg.ActiveFanCurveProfileID
	if activeID == "" {
		return
	}
	if temp, ok := cfg.SmartControl.TargetTempByProfile[activeID]; ok && temp >= 45 && temp <= 90 {
		cfg.SmartControl.TargetTemp = temp
	}
	if bias, ok := cfg.SmartControl.LearningBiasByProfile[activeID]; ok {
		cfg.SmartControl.LearningBias = types.NormalizeLearningBias(bias)
	}
}
