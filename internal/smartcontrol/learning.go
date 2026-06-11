package smartcontrol

import (
	"github.com/TIANLI0/THRM/internal/types"
)

// StableObserver + 稳态学习用到的常量。
const (
	// rpmPerDegree     = 50
	hardOffsetCap    = 600
	stableTempBand   = 2
	stableMinSamples = 6
	stableRPMBand    = 120

	// 冷却效率估计与转速寻优相关常量。
	effHistoryLen    = 6      // 每个温度桶保留的稳态 (转速,温度) 样本数
	minRPMSpanForEff = 80     // 估计冷却效率所需的最小转速跨度 (RPM)
	effFloorPerRPM   = 0.0008 // 冷却效率下限 (°C/RPM)，防止步长发散
	effCeilPerRPM    = 0.05   // 冷却效率上限 (°C/RPM)
	defaultEffPerRPM = 0.008  // 无历史时的保守冷却效率 (≈0.8°C/100RPM)
	maxLearnStep     = 80     // 单次学习的最大转速调整 (RPM)
	learnStepDeadRPM = 20     // 小于此调整量则忽略，避免抖动
	minSafetyStep    = 20     // 温度超目标时的最小降温步长 (RPM)
	defaultTargetTmp = 70     // TargetTemp 未配置时的回退目标温度 (°C)

	offsetSmoothPasses         = 2
	offsetSmoothPullLimit      = 30
	offsetSmoothSelfWeight     = 0.7
	offsetSmoothNeighborWeight = 0.15
	offsetSmoothRadius         = 2 // 平滑只作用于学习桶 ± 该半径，避免抹平远处已学偏移
	eqConsistencyBand          = 3 // °C；新旧平衡点热学矛盾超过该值视为负载已变化

	// 噪音档案（麦克风实测转速-噪音曲线）相关常量。
	noiseProfileMinPoints  = 3   // 少于该点数不足以估计局部斜率
	noiseProfileMinSpanRPM = 500 // 档案覆盖的最小转速跨度
	noiseProfileMinRiseDB  = 1.0 // 全程噪音上升低于该值视为测量无效
	noiseGainRawMin        = 0.3 // 局部/平均斜率比的下限
	noiseGainRawMax        = 2.0 // 局部/平均斜率比的上限
	noiseGainMin           = 0.4 // 最终降速增益下限
	noiseGainMax           = 1.8 // 最终降速增益上限
	noiseWeightBaseline    = 4.0 // NoiseWeight 默认值，作为增益强度基准
)

// eqPoint 记录一次稳态 (转速, 温度) 平衡点。
type eqPoint struct {
	rpm  int
	temp int
}

// SteadyResult 是一次稳态观测的结果。
type SteadyResult struct {
	BucketIdx int     // 命中的曲线点索引；-1 表示无效
	MeanTemp  int     // 稳态平均温度 (°C)
	MeanRPM   int     // 稳态期间的平均下发转速 (RPM)
	LocalEff  float64 // 局部冷却效率 (°C/RPM)，正值
	HaveEff   bool    // 是否成功估计出冷却效率
	Ready     bool    // 是否达到稳态、可触发一次学习
}

// StableObserver 为每个曲线点累积稳态采样，并维护 (转速,温度) 平衡点历史。
type StableObserver struct {
	curveLen   int
	samples    [][]int     // 每个温度桶的温度采样
	rpmSamples [][]int     // 与 samples 平行的转速采样
	history    [][]eqPoint // 每个温度桶最近的稳态平衡点
	settle     []int       // 每个温度桶进入稳定采样前的延迟计数
	lastTemps  []int       // 最近一次观测温度
	lastRPMs   []int       // 最近一次观测到的实际转速
	seen       []bool      // 最近观测是否有效
}

// NewStableObserver 创建针对当前曲线长度的观察者。
func NewStableObserver(curveLen int) *StableObserver {
	if curveLen <= 0 {
		curveLen = 1
	}
	o := &StableObserver{curveLen: curveLen}
	o.allocBuffers(curveLen)
	return o
}

func (o *StableObserver) allocBuffers(curveLen int) {
	o.samples = make([][]int, curveLen)
	o.rpmSamples = make([][]int, curveLen)
	o.history = make([][]eqPoint, curveLen)
	o.settle = make([]int, curveLen)
	o.lastTemps = make([]int, curveLen)
	o.lastRPMs = make([]int, curveLen)
	o.seen = make([]bool, curveLen)
	for i := range o.samples {
		o.samples[i] = make([]int, 0, 24)
		o.rpmSamples[i] = make([]int, 0, 24)
		o.history[i] = make([]eqPoint, 0, effHistoryLen)
	}
}

// Resize 在曲线长度变化时调整内部缓冲。曲线变化会使历史失效，因此一并清空。
func (o *StableObserver) Resize(curveLen int) {
	if curveLen <= 0 {
		curveLen = 1
	}
	if o.curveLen == curveLen {
		o.Reset()
		return
	}
	o.curveLen = curveLen
	o.allocBuffers(curveLen)
}

// Reset 清空进行中的采样缓冲，但保留已学到的效率历史。
func (o *StableObserver) Reset() {
	for i := range o.samples {
		o.samples[i] = o.samples[i][:0]
		o.rpmSamples[i] = o.rpmSamples[i][:0]
		o.settle[i] = 0
		o.lastTemps[i] = 0
		o.lastRPMs[i] = 0
		o.seen[i] = false
	}
}

func stableSampleWindow(cfg types.SmartControlConfig) int {
	window := cfg.LearnWindow
	if window <= 0 {
		window = stableMinSamples
	}
	return clampInt(window, 3, 24)
}

func stableSampleDelay(cfg types.SmartControlConfig) int {
	delay := max(cfg.LearnDelay, 0)
	return clampInt(delay, 0, 8)
}

func stableRPMRange(cfg types.SmartControlConfig) int {
	return max(stableRPMBand, cfg.MinRPMChange)
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

// Observe 把一次 (温度, 实际转速) 采样放入对应温度桶。
// 达到稳态时返回平均温度、平均转速及局部冷却效率估计。
func (o *StableObserver) Observe(temp, effectiveRPM int, curve []types.FanCurvePoint, cfg types.SmartControlConfig) SteadyResult {
	idx := pickBucketIndex(temp, curve)
	if idx < 0 || idx >= len(o.samples) {
		return SteadyResult{BucketIdx: -1}
	}
	window := stableSampleWindow(cfg)
	delay := stableSampleDelay(cfg)
	rpmBand := stableRPMRange(cfg)

	if o.seen[idx] {
		tempJump := absInt(temp-o.lastTemps[idx]) > stableTempBand+1
		rpmJump := effectiveRPM > 0 && o.lastRPMs[idx] > 0 && absInt(effectiveRPM-o.lastRPMs[idx]) > rpmBand
		if tempJump || rpmJump {
			o.samples[idx] = o.samples[idx][:0]
			o.rpmSamples[idx] = o.rpmSamples[idx][:0]
			o.settle[idx] = 0
		}
	} else {
		o.seen[idx] = true
		o.settle[idx] = 0
	}
	o.lastTemps[idx] = temp
	o.lastRPMs[idx] = effectiveRPM

	if o.settle[idx] < delay {
		o.settle[idx]++
		return SteadyResult{BucketIdx: idx}
	}

	o.samples[idx] = append(o.samples[idx], temp)
	o.rpmSamples[idx] = append(o.rpmSamples[idx], effectiveRPM)
	if len(o.samples[idx]) > window {
		o.samples[idx] = o.samples[idx][len(o.samples[idx])-window:]
		o.rpmSamples[idx] = o.rpmSamples[idx][len(o.rpmSamples[idx])-window:]
	}

	if len(o.samples[idx]) < window {
		return SteadyResult{BucketIdx: idx}
	}
	minT, maxT, sumT, sumR := o.samples[idx][0], o.samples[idx][0], 0, 0
	minR, maxR := o.rpmSamples[idx][0], o.rpmSamples[idx][0]
	for i, t := range o.samples[idx] {
		if t < minT {
			minT = t
		}
		if t > maxT {
			maxT = t
		}
		rpm := o.rpmSamples[idx][i]
		if rpm < minR {
			minR = rpm
		}
		if rpm > maxR {
			maxR = rpm
		}
		sumT += t
		sumR += rpm
	}
	if maxT-minT > stableTempBand || maxR-minR > rpmBand {
		return SteadyResult{BucketIdx: idx}
	}

	meanT := sumT / len(o.samples[idx])
	meanR := sumR / len(o.rpmSamples[idx])
	o.samples[idx] = o.samples[idx][:0]
	o.rpmSamples[idx] = o.rpmSamples[idx][:0]
	o.settle[idx] = 0

	o.recordEquilibrium(idx, meanR, meanT)
	eff, haveEff := o.localEfficiency(idx)

	return SteadyResult{
		BucketIdx: idx,
		MeanTemp:  meanT,
		MeanRPM:   meanR,
		LocalEff:  eff,
		HaveEff:   haveEff,
		Ready:     true,
	}
}

// recordEquilibrium 把一次稳态平衡点写入桶历史（环形保留最近 effHistoryLen 条）。
// 同一转速附近的旧样本会被新样本覆盖，使历史反映最新的热行为；
// 与新样本热学矛盾的旧样本（说明环境/负载已变化）会被剔除，避免污染效率估计。
func (o *StableObserver) recordEquilibrium(idx, rpm, temp int) {
	if idx < 0 || idx >= len(o.history) {
		return
	}
	hist := o.history[idx]
	replaced := false
	kept := hist[:0]
	for _, p := range hist {
		if !replaced && absInt(p.rpm-rpm) < minRPMSpanForEff {
			kept = append(kept, eqPoint{rpm: rpm, temp: temp})
			replaced = true
			continue
		}
		if !staleEquilibrium(p, rpm, temp) {
			kept = append(kept, p)
		}
	}
	if !replaced {
		kept = append(kept, eqPoint{rpm: rpm, temp: temp})
	}
	if len(kept) > effHistoryLen {
		kept = kept[len(kept)-effHistoryLen:]
	}
	o.history[idx] = kept
}

// staleEquilibrium 判断旧平衡点 p 是否与新平衡点 (rpm, temp) 热学矛盾，
// 即来自不同的环境/负载工况：
//   - 方向矛盾：低转速点反而更冷（或高转速点反而更热），超出 eqConsistencyBand；
//   - 幅度矛盾：两点隐含的冷却效率超过物理上限 effCeilPerRPM。
func staleEquilibrium(p eqPoint, rpm, temp int) bool {
	if p.rpm < rpm {
		if p.temp+eqConsistencyBand < temp {
			return true
		}
		maxDrop := effCeilPerRPM*float64(rpm-p.rpm) + eqConsistencyBand
		return float64(p.temp-temp) > maxDrop
	}
	if p.rpm > rpm {
		if p.temp > temp+eqConsistencyBand {
			return true
		}
		maxDrop := effCeilPerRPM*float64(p.rpm-rpm) + eqConsistencyBand
		return float64(temp-p.temp) > maxDrop
	}
	return false
}

// localEfficiency 对桶历史中的全部平衡点做最小二乘回归，估计局部冷却效率
// (°C/RPM, 正值)。相比只取两个端点，回归对单点测量噪声更稳健。
// 更高转速对应更低温度时效率为正；若数据不足或冷却无效则保守处理。
func (o *StableObserver) localEfficiency(idx int) (float64, bool) {
	if idx < 0 || idx >= len(o.history) {
		return 0, false
	}
	hist := o.history[idx]
	if len(hist) < 2 {
		return 0, false
	}
	minRPM, maxRPM := hist[0].rpm, hist[0].rpm
	sumR, sumT := 0, 0
	for _, p := range hist {
		if p.rpm < minRPM {
			minRPM = p.rpm
		}
		if p.rpm > maxRPM {
			maxRPM = p.rpm
		}
		sumR += p.rpm
		sumT += p.temp
	}
	if maxRPM-minRPM < minRPMSpanForEff {
		return 0, false
	}
	meanR := float64(sumR) / float64(len(hist))
	meanT := float64(sumT) / float64(len(hist))
	var cov, varR float64
	for _, p := range hist {
		dr := float64(p.rpm) - meanR
		cov += dr * (float64(p.temp) - meanT)
		varR += dr * dr
	}
	// 冷却有效时温度随转速下降，回归斜率为负，取反得到正效率。
	eff := -cov / varR
	if eff < effFloorPerRPM {
		// 冷却几乎无效（甚至负相关）：视为最低效率，让寻优倾向于降转速省噪音。
		eff = effFloorPerRPM
	}
	if eff > effCeilPerRPM {
		eff = effCeilPerRPM
	}
	return eff, true
}

// alphaFromLearnRate 把 1..10 的 LearnRate 映射成反馈系数。
func alphaFromLearnRate(learnRate int) float64 {
	if learnRate < 1 {
		learnRate = 1
	}
	if learnRate > 10 {
		learnRate = 10
	}
	return 0.025 + float64(learnRate-1)*0.0125
}

// effectiveOffsetCap 取 cfg.MaxLearnOffset 和 hardOffsetCap 的较小值。
func effectiveOffsetCap(cfg types.SmartControlConfig) int {
	cap := cfg.MaxLearnOffset
	if cap <= 0 || cap > hardOffsetCap {
		cap = hardOffsetCap
	}
	return cap
}

// targetTempCeiling 返回学习寻优使用的目标温度上限。
func targetTempCeiling(cfg types.SmartControlConfig) int {
	if cfg.TargetTemp > 0 {
		return cfg.TargetTemp
	}
	return defaultTargetTmp
}

// comfortBandWidth 返回目标温度下方的舒适带宽度 (°C)。
// 舒适带内不动作，避免无意义的转速抖动；带宽随滞回温差略微放宽。
func comfortBandWidth(cfg types.SmartControlConfig) int {
	band := max(cfg.Hysteresis+3, 3)
	return band
}

// localNoiseSlope 估计 rpm 附近的噪音斜率 (dB/RPM)。
// 取包含 rpm 的档案段并向两侧各扩一个点做中心差分，抑制单点测量噪声。
func localNoiseSlope(rpm int, profile []types.NoiseProfilePoint) (float64, bool) {
	n := len(profile)
	if n < 2 {
		return 0, false
	}
	seg := n - 2
	for i := 0; i < n-1; i++ {
		if rpm < profile[i+1].RPM {
			seg = i
			break
		}
	}
	lo := max(seg-1, 0)
	hi := min(seg+2, n-1)
	span := profile[hi].RPM - profile[lo].RPM
	if span <= 0 {
		return 0, false
	}
	slope := (profile[hi].DB - profile[lo].DB) / float64(span)
	if slope < 0 {
		slope = 0
	}
	return slope, true
}

// noiseDownGain 依据实测噪音档案计算降速步长增益。
//
// 思路：降速的价值 = 省下的噪音。局部噪音斜率（dB/RPM）相对全程平均斜率
// 越陡，说明在当前转速附近降一点转速就能省较多噪音，学习降速应更积极（增益>1）；
// 斜率平坦说明降速几乎听不出差别，不如保留散热余量（增益<1）。
// NoiseWeight 控制档案对学习的影响强度，0 表示完全不参考档案。
func noiseDownGain(rpm int, cfg types.SmartControlConfig) float64 {
	profile := cfg.NoiseProfile
	if len(profile) < noiseProfileMinPoints || cfg.NoiseWeight <= 0 || rpm <= 0 {
		return 1
	}
	span := profile[len(profile)-1].RPM - profile[0].RPM
	totalRise := profile[len(profile)-1].DB - profile[0].DB
	if span < noiseProfileMinSpanRPM || totalRise < noiseProfileMinRiseDB {
		return 1
	}
	avgSlope := totalRise / float64(span)
	local, ok := localNoiseSlope(rpm, profile)
	if !ok || avgSlope <= 0 {
		return 1
	}

	raw := local / avgSlope
	if raw < noiseGainRawMin {
		raw = noiseGainRawMin
	}
	if raw > noiseGainRawMax {
		raw = noiseGainRawMax
	}

	influence := float64(cfg.NoiseWeight) / noiseWeightBaseline
	if influence > 1.5 {
		influence = 1.5
	}
	gain := 1 + (raw-1)*influence
	if gain < noiseGainMin {
		gain = noiseGainMin
	}
	if gain > noiseGainMax {
		gain = noiseGainMax
	}
	return gain
}

// solveLearnStep 依据稳态温度、目标温度带与冷却效率，求出本次应施加的转速调整 (RPM)。
//
// 策略：
//   - 温度高于目标温度  → 加转速降温，步长 = α·(超出°C)/效率，确保把温度压回目标附近。
//   - 温度处于舒适带内  → 保持不动（这是消除“无脑降温”的关键：温度够低就不再加速）。
//   - 温度低于舒适带    → 主动降转速省噪音，可降幅 = α·(可上升°C)/效率；
//     冷却越低效（效率小），同样的降速带来的升温越小，于是越敢大幅降速。
//     若存在实测噪音档案，降幅再按当前转速附近的降噪收益加权（见 noiseDownGain）。
//
// 冷却效率 eff (°C/RPM) 把“温度误差”换算成“转速需求”，使步长物理合理、收敛快且不易过冲。
func solveLearnStep(steadyTemp, steadyRPM int, eff float64, haveEff bool, cfg types.SmartControlConfig) int {
	ceiling := targetTempCeiling(cfg)
	lowTarget := ceiling - comfortBandWidth(cfg)
	alpha := alphaFromLearnRate(cfg.LearnRate)

	if !haveEff || eff < effFloorPerRPM {
		eff = defaultEffPerRPM
	}
	if eff > effCeilPerRPM {
		eff = effCeilPerRPM
	}

	var step float64
	switch {
	case steadyTemp > ceiling:
		step = alpha * float64(steadyTemp-ceiling) / eff
		if step < minSafetyStep {
			step = minSafetyStep
		}
	case steadyTemp < lowTarget:
		step = -alpha * float64(lowTarget-steadyTemp) / eff * noiseDownGain(steadyRPM, cfg)
	default:
		return 0
	}

	if step > maxLearnStep {
		step = maxLearnStep
	}
	if step < -maxLearnStep {
		step = -maxLearnStep
	}

	delta := roundFloat(step)
	if steadyTemp <= ceiling && absInt(delta) < learnStepDeadRPM {
		return 0
	}
	return delta
}

// LearnSteadyOffset 根据一次稳态观测（温度 + 转速 + 冷却效率）更新学习偏移。
func LearnSteadyOffset(
	bucketIdx int,
	steadyMeanTemp int,
	steadyMeanRPM int,
	localEff float64,
	haveEff bool,
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

	if steadyMeanRPM <= 0 {
		steadyMeanRPM = curve[bucketIdx].RPM + offsets[bucketIdx]
	}
	mainDelta := solveLearnStep(steadyMeanTemp, steadyMeanRPM, localEff, haveEff, cfg)
	if mainDelta == 0 {
		return offsets, false
	}

	cap := effectiveOffsetCap(cfg)
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
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}

	// 只在学习桶附近做一轮局部平滑：既把变化柔和地扩散给邻点，
	// 又不会在每次学习时反复稀释远处已学好的偏移。
	smoothOffsets(curve, offsets, bucketIdx, cap, leftMin, rightMax)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}
	enforceMonotonicWithOffsets(curve, offsets, cap, leftMin, rightMax)

	changed := false
	for i := range offsets {
		if i >= len(prevOffsets) || offsets[i] != prevOffsets[i] {
			changed = true
			break
		}
	}
	return offsets, changed
}

// roundFloat 四舍五入到最近整数
func roundFloat(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// smoothOffsets 在 center ± offsetSmoothRadius 的窗口内做加权平滑。
// 限定窗口是为了让一次学习只影响局部，不抹平远处已学到的偏移。
func smoothOffsets(curve []types.FanCurvePoint, offsets []int, center, cap, leftMin, rightMax int) {
	limit := min(len(offsets), len(curve))
	if limit < 3 {
		return
	}
	lo := max(center-offsetSmoothRadius, 1)
	hi := min(center+offsetSmoothRadius, limit-2)
	if lo > hi {
		return
	}
	work := make([]int, len(offsets))
	copy(work, offsets)
	for range offsetSmoothPasses {
		copy(work, offsets)
		for i := lo; i <= hi; i++ {
			target := roundFloat(
				offsetSmoothSelfWeight*float64(offsets[i]) +
					offsetSmoothNeighborWeight*float64(offsets[i-1]) +
					offsetSmoothNeighborWeight*float64(offsets[i+1]),
			)
			pull := target - offsets[i]
			if pull > offsetSmoothPullLimit {
				target = offsets[i] + offsetSmoothPullLimit
			} else if pull < -offsetSmoothPullLimit {
				target = offsets[i] - offsetSmoothPullLimit
			}
			work[i] = clampOffsetForPoint(target, curve[i].RPM, leftMin, rightMax, cap)
		}
		copy(offsets, work)
	}
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
