package smartcontrol

import "github.com/TIANLI0/BS2PRO-Controller/internal/types"

func getCurveEdgeRPMBounds(curve []types.FanCurvePoint) (int, int) {
	if len(curve) == 0 {
		return 0, 4000
	}
	left := curve[0].RPM
	right := curve[len(curve)-1].RPM
	if left > right {
		left, right = right, left
	}
	return left, right
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

func enforceNonDecreasingRPM(curve []types.FanCurvePoint) {
	for i := 1; i < len(curve); i++ {
		if curve[i].RPM < curve[i-1].RPM {
			curve[i].RPM = curve[i-1].RPM
		}
	}
}
