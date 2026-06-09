package coreapp

import (
	"slices"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func normalizeFanFeatureConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	normalizedSpeedAvoidance := types.NormalizeSpeedAvoidanceConfig(cfg.SpeedAvoidance)
	if normalizedSpeedAvoidance != cfg.SpeedAvoidance {
		cfg.SpeedAvoidance = normalizedSpeedAvoidance
		changed = true
	}

	normalizedSchedule := types.NormalizeTimeCurveScheduleConfig(cfg.TimeCurveSchedule, cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if !slices.EqualFunc(normalizedSchedule.Rules, cfg.TimeCurveSchedule.Rules, func(left, right types.TimeCurveScheduleRule) bool {
		if left.ID != right.ID || left.Name != right.Name || left.Enabled != right.Enabled || left.StartTime != right.StartTime || left.EndTime != right.EndTime || left.CurveProfileID != right.CurveProfileID {
			return false
		}
		return slices.Equal(left.Weekdays, right.Weekdays)
	}) || normalizedSchedule.Enabled != cfg.TimeCurveSchedule.Enabled {
		cfg.TimeCurveSchedule = normalizedSchedule
		changed = true
	}

	return changed
}

func applySpeedAvoidance(targetRPM, minAllowedRPM, maxAllowedRPM, prevTargetRPM, controlTemp, prevControlTemp int, cfg types.SpeedAvoidanceConfig) (int, bool) {
	if !cfg.Enabled || targetRPM <= 0 {
		return targetRPM, false
	}
	if cfg.EmergencyBypassTemp > 0 && controlTemp >= cfg.EmergencyBypassTemp {
		return targetRPM, false
	}

	avoidMin := min(cfg.MinRPM, cfg.MaxRPM)
	avoidMax := max(cfg.MinRPM, cfg.MaxRPM)
	if targetRPM < avoidMin || targetRPM > avoidMax {
		return targetRPM, false
	}

	downCandidate := max(minAllowedRPM, avoidMin-cfg.MarginRPM)
	upCandidate := min(maxAllowedRPM, avoidMax+cfg.MarginRPM)
	downValid := downCandidate < avoidMin
	upValid := upCandidate > avoidMax

	if !downValid && !upValid {
		return targetRPM, false
	}
	if downValid && !upValid {
		return downCandidate, true
	}
	if upValid && !downValid {
		return upCandidate, true
	}

	heatingUp := prevControlTemp > 0 && controlTemp > prevControlTemp
	rampUp := prevTargetRPM > 0 && targetRPM >= prevTargetRPM
	if heatingUp || rampUp {
		return upCandidate, true
	}
	if prevTargetRPM > 0 {
		if prevTargetRPM <= avoidMin {
			return downCandidate, true
		}
		if prevTargetRPM >= avoidMax {
			return upCandidate, true
		}
	}

	if targetRPM-downCandidate <= upCandidate-targetRPM {
		return downCandidate, true
	}
	return upCandidate, true
}

func (a *CoreApp) applyTimeCurveSchedule(now time.Time) {
	cfg := a.configManager.Get()
	if !cfg.TimeCurveSchedule.Enabled || len(cfg.TimeCurveSchedule.Rules) == 0 {
		return
	}

	rule := findMatchingTimeCurveScheduleRule(cfg.TimeCurveSchedule, now)
	if rule == nil {
		return
	}
	if strings.TrimSpace(rule.CurveProfileID) == "" || rule.CurveProfileID == cfg.ActiveFanCurveProfileID {
		return
	}

	if _, err := a.SetActiveFanCurveProfile(rule.CurveProfileID); err != nil {
		a.logError("分时曲线切换失败[%s -> %s]: %v", rule.Name, rule.CurveProfileID, err)
		return
	}
	a.logInfo("分时曲线已生效: %s -> %s", rule.Name, rule.CurveProfileID)
}

func findMatchingTimeCurveScheduleRule(schedule types.TimeCurveScheduleConfig, now time.Time) *types.TimeCurveScheduleRule {
	currentMinutes := now.Hour()*60 + now.Minute()
	weekday := int(now.Weekday())
	previousWeekday := (weekday + 6) % 7

	for index := range schedule.Rules {
		rule := &schedule.Rules[index]
		if !rule.Enabled || strings.TrimSpace(rule.CurveProfileID) == "" {
			continue
		}
		startMinutes, ok := parseScheduleClock(rule.StartTime)
		if !ok {
			continue
		}
		endMinutes, ok := parseScheduleClock(rule.EndTime)
		if !ok {
			continue
		}

		if scheduleRuleMatches(rule.Weekdays, weekday, previousWeekday, currentMinutes, startMinutes, endMinutes) {
			return rule
		}
	}

	return nil
}

func parseScheduleClock(value string) (int, bool) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func scheduleRuleMatches(days []int, weekday, previousWeekday, currentMinutes, startMinutes, endMinutes int) bool {
	if startMinutes == endMinutes {
		return slices.Contains(days, weekday)
	}
	if startMinutes < endMinutes {
		return slices.Contains(days, weekday) && currentMinutes >= startMinutes && currentMinutes < endMinutes
	}
	if currentMinutes >= startMinutes {
		return slices.Contains(days, weekday)
	}
	return slices.Contains(days, previousWeekday) && currentMinutes < endMinutes
}
