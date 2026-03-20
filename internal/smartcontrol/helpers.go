package smartcontrol

import "github.com/TIANLI0/BS2PRO-Controller/internal/types"

func getCurveEdgeRPMBounds(curve []types.FanCurvePoint) (int, int) {
	return GetCurveRPMBounds(curve)
}

// GetCurveRPMBounds returns the min/max RPM bounds of the user curve.
func GetCurveRPMBounds(curve []types.FanCurvePoint) (int, int) {
	if len(curve) == 0 {
		return 0, 4000
	}
	minRPM := curve[0].RPM
	maxRPM := curve[0].RPM
	for i := 1; i < len(curve); i++ {
		rpm := curve[i].RPM
		if rpm < minRPM {
			minRPM = rpm
		}
		if rpm > maxRPM {
			maxRPM = rpm
		}
	}
	return minRPM, maxRPM
}

func clampOffsetForPoint(offset, baseRPM, leftMinRPM, rightMaxRPM, maxLearnOffset int) int {
	minOffset := leftMinRPM - baseRPM
	maxOffset := rightMaxRPM - baseRPM
	minOffset = max(minOffset, -maxLearnOffset)
	maxOffset = min(maxOffset, maxLearnOffset)
	if minOffset > maxOffset {
		return 0
	}
	return clampInt(offset, minOffset, maxOffset)
}

func constrainOffsetsToCurveBounds(offsets []int, curve []types.FanCurvePoint, maxLearnOffset int) ([]int, bool) {
	if len(offsets) == 0 || len(curve) == 0 {
		return offsets, false
	}
	leftMinRPM, rightMaxRPM := getCurveEdgeRPMBounds(curve)
	updated := false
	normalized := make([]int, len(offsets))
	copy(normalized, offsets)
	for i := range normalized {
		if i >= len(curve) {
			normalized[i] = 0
			updated = true
			continue
		}
		clamped := clampOffsetForPoint(normalized[i], curve[i].RPM, leftMinRPM, rightMaxRPM, maxLearnOffset)
		if clamped != normalized[i] {
			normalized[i] = clamped
			updated = true
		}
	}
	return normalized, updated
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

func medianOfThree(a, b, c int) int {
	if a > b {
		a, b = b, a
	}
	if b > c {
		b, c = c, b
	}
	if a > b {
		a, b = b, a
	}
	return b
}

// FilterTransientSpike suppresses single-sample transient temperature spikes in the control loop.
func FilterTransientSpike(currentTemp int, recentTemps []int, targetTemp, hysteresis int) (int, bool) {
	if len(recentTemps) < 3 {
		return currentTemp, false
	}

	// In the high temperature zone, be conservative to avoid suppressing real overheating.
	if currentTemp >= targetTemp+10 {
		return currentTemp, false
	}

	last3 := recentTemps[len(recentTemps)-3:]
	baseline := medianOfThree(last3[0], last3[1], last3[2])
	spikeBand := max(2, hysteresis+2)
	if currentTemp-baseline >= spikeBand {
		return baseline, true
	}

	return currentTemp, false
}

// isSustainedAboveThreshold checks if recent temperature readings have been sustained above the specified threshold for at least minCount times.
func isSustainedAboveThreshold(temps []int, threshold, minCount int) bool {
	if minCount <= 0 || len(temps) < minCount {
		return false
	}
	start := len(temps) - minCount
	for i := start; i < len(temps); i++ {
		if temps[i] < threshold {
			return false
		}
	}
	return true
}

func enforceNonDecreasingRPM(curve []types.FanCurvePoint) {
	for i := 1; i < len(curve); i++ {
		if curve[i].RPM < curve[i-1].RPM {
			curve[i].RPM = curve[i-1].RPM
		}
	}
}
