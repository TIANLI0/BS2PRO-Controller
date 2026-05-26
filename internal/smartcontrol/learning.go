package smartcontrol

import (
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// StableObserver + 一阶反馈学习用到的常量。
const (
	rpmPerDegree     = 50
	hardOffsetCap    = 600
	stableTempBand   = 2
	stableMinSamples = 6
	neighborShare    = 3
)

// StableObserver 为每个曲线点累积稳态采样。
type StableObserver struct {
	curveLen int
	samples  [][]int
}

// NewStableObserver 创建针对当前曲线长度的观察者。
func NewStableObserver(curveLen int) *StableObserver {
	if curveLen <= 0 {
		curveLen = 1
	}
	samples := make([][]int, curveLen)
	for i := range samples {
		samples[i] = make([]int, 0, stableMinSamples*2)
	}
	return &StableObserver{curveLen: curveLen, samples: samples}
}

// Resize 在曲线长度变化时调整内部缓冲并清空采样。
func (o *StableObserver) Resize(curveLen int) {
	if curveLen <= 0 {
		curveLen = 1
	}
	if o.curveLen == curveLen {
		o.Reset()
		return
	}
	o.curveLen = curveLen
	o.samples = make([][]int, curveLen)
	for i := range o.samples {
		o.samples[i] = make([]int, 0, stableMinSamples*2)
	}
}

// Reset 清空所有累积样本。
func (o *StableObserver) Reset() {
	for i := range o.samples {
		o.samples[i] = o.samples[i][:0]
	}
}

// CurveLen 返回当前观察者的曲线长度。
func (o *StableObserver) CurveLen() int {
	return o.curveLen
}

// pickBucketIndex 按最近邻选择温度所属的曲线点。
func pickBucketIndex(temp int, curve []types.FanCurvePoint) int {
	if len(curve) == 0 {
		return -1
	}
	if temp <= curve[0].Temperature {
		return 0
	}
	if temp >= curve[len(curve)-1].Temperature {
		return len(curve) - 1
	}
	for i := 0; i < len(curve)-1; i++ {
		if temp >= curve[i].Temperature && temp < curve[i+1].Temperature {
			midpoint := (curve[i].Temperature + curve[i+1].Temperature) / 2
			if temp < midpoint {
				return i
			}
			return i + 1
		}
	}
	return len(curve) - 1
}

// Observe 把一次温度采样放入对应温度桶。
func (o *StableObserver) Observe(temp int, curve []types.FanCurvePoint) (int, int, bool) {
	idx := pickBucketIndex(temp, curve)
	if idx < 0 || idx >= len(o.samples) {
		return -1, 0, false
	}

	o.samples[idx] = append(o.samples[idx], temp)
	if len(o.samples[idx]) > stableMinSamples*2 {
		o.samples[idx] = o.samples[idx][len(o.samples[idx])-stableMinSamples*2:]
	}

	if len(o.samples[idx]) < stableMinSamples {
		return idx, 0, false
	}
	minT, maxT, sum := o.samples[idx][0], o.samples[idx][0], 0
	for _, t := range o.samples[idx] {
		if t < minT {
			minT = t
		}
		if t > maxT {
			maxT = t
		}
		sum += t
	}
	if maxT-minT > stableTempBand {
		return idx, 0, false
	}

	mean := sum / len(o.samples[idx])
	o.samples[idx] = o.samples[idx][:0]
	return idx, mean, true
}

// alphaFromLearnRate 把 1..10 的 LearnRate 映射成反馈系数。
func alphaFromLearnRate(learnRate int) float64 {
	if learnRate < 1 {
		learnRate = 1
	}
	if learnRate > 10 {
		learnRate = 10
	}
	return 0.05 + float64(learnRate-1)*0.05
}

// effectiveOffsetCap 取 cfg.MaxLearnOffset 和 hardOffsetCap 的较小值。
func effectiveOffsetCap(cfg types.SmartControlConfig) int {
	cap := cfg.MaxLearnOffset
	if cap <= 0 || cap > hardOffsetCap {
		cap = hardOffsetCap
	}
	return cap
}

// LearnSteadyOffset 根据稳态温度更新学习偏移。
func LearnSteadyOffset(
	bucketIdx int,
	steadyMeanTemp int,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
) ([]int, bool) {
	if bucketIdx < 0 || bucketIdx >= len(curve) {
		return prevOffsets, false
	}

	offsets := make([]int, len(curve))
	for i := range offsets {
		if i < len(prevOffsets) {
			offsets[i] = prevOffsets[i]
		}
	}

	tempError := steadyMeanTemp - curve[bucketIdx].Temperature
	if tempError == 0 {
		return offsets, false
	}

	alpha := alphaFromLearnRate(cfg.LearnRate)
	cap := effectiveOffsetCap(cfg)

	mainDelta := int(alpha * float64(tempError) * float64(rpmPerDegree))
	if mainDelta == 0 {
		if tempError > 0 {
			mainDelta = 1
		} else {
			mainDelta = -1
		}
	}

	neighborDelta := mainDelta / neighborShare

	leftMin, rightMax := GetCurveRPMBounds(curve)

	apply := func(idx, delta int) {
		if idx < 0 || idx >= len(offsets) || delta == 0 {
			return
		}
		offsets[idx] = clampOffsetForPoint(
			offsets[idx]+delta,
			curve[idx].RPM,
			leftMin,
			rightMax,
			cap,
		)
	}
	apply(bucketIdx, mainDelta)
	apply(bucketIdx-1, neighborDelta)
	apply(bucketIdx+1, neighborDelta)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}

	enforceMonotonicWithOffsets(curve, offsets, cap, leftMin, rightMax)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
		enforceMonotonicWithOffsets(curve, offsets, cap, leftMin, rightMax)
	}

	changed := false
	for i := range offsets {
		if i >= len(prevOffsets) || offsets[i] != prevOffsets[i] {
			changed = true
			break
		}
	}
	return offsets, changed
}

// enforceMonotonicWithOffsets 确保 (RPM_i + Δ_i) 沿 i 非递减；
// 如果某点违反，向上调整 Δ_i 直至单调（仍受 cap 与曲线 RPM 上限约束）。
func enforceMonotonicWithOffsets(curve []types.FanCurvePoint, offsets []int, cap, leftMin, rightMax int) {
	for i := 1; i < len(curve) && i < len(offsets); i++ {
		prevEffective := curve[i-1].RPM + offsets[i-1]
		currEffective := curve[i].RPM + offsets[i]
		if currEffective < prevEffective {
			needed := prevEffective - curve[i].RPM
			offsets[i] = clampOffsetForPoint(needed, curve[i].RPM, leftMin, rightMax, cap)
		}
	}
}

// ResetLearnedState 清空学习相关字段（保留可学习开关本身）。
// 旧字段也清空以保证存档一致。
func ResetLearnedState(cfg types.SmartControlConfig, curve []types.FanCurvePoint) types.SmartControlConfig {
	// rateBucketCount 来自 doc.go (rateBucketMax - rateBucketMin + 1)；
	// 这里仅为保持旧字段长度合法，不再被新算法读取。
	rateLen := rateBucketMax - rateBucketMin + 1
	cfg.LearnedOffsets = make([]int, len(curve))
	cfg.LearnedOffsetsHeat = make([]int, len(curve))
	cfg.LearnedOffsetsCool = make([]int, len(curve))
	cfg.LearnedRateHeat = make([]int, rateLen)
	cfg.LearnedRateCool = make([]int, rateLen)
	return cfg
}
