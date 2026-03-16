package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	cfgpkg "github.com/TIANLI0/BS2PRO-Controller/internal/config"
	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/TIANLI0/BS2PRO-Controller/internal/smartcontrol"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

const curveExportPrefix = "B2C1."

type fanCurveExportPayload struct {
	V        int                     `json:"v"`
	Active   string                  `json:"a"`
	Profiles []types.FanCurveProfile `json:"p"`
}

func cloneFanCurve(curve []types.FanCurvePoint) []types.FanCurvePoint {
	if len(curve) == 0 {
		return nil
	}
	out := make([]types.FanCurvePoint, len(curve))
	copy(out, curve)
	return out
}

func cloneFanCurveProfiles(profiles []types.FanCurveProfile) []types.FanCurveProfile {
	if len(profiles) == 0 {
		return nil
	}
	out := make([]types.FanCurveProfile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, types.FanCurveProfile{
			ID:    p.ID,
			Name:  p.Name,
			Curve: cloneFanCurve(p.Curve),
		})
	}
	return out
}

func truncateByRunes(input string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(input) <= maxRunes {
		return input
	}
	r := []rune(input)
	return string(r[:maxRunes])
}

func normalizeCurveProfileName(name string, fallback string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		n = fallback
	}
	return truncateByRunes(n, 6)
}

func generateCurveProfileID() string {
	return fmt.Sprintf("p%x", time.Now().UnixNano())
}

func findCurveProfileIndex(profiles []types.FanCurveProfile, profileID string) int {
	for i := range profiles {
		if profiles[i].ID == profileID {
			return i
		}
	}
	return -1
}

func normalizeCurveProfilesConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	baseCurve := cloneFanCurve(cfg.FanCurve)
	if len(baseCurve) == 0 {
		baseCurve = types.GetDefaultFanCurve()
		changed = true
	}

	if len(cfg.FanCurveProfiles) == 0 {
		cfg.FanCurveProfiles = []types.FanCurveProfile{{
			ID:    "default",
			Name:  "默认",
			Curve: cloneFanCurve(baseCurve),
		}}
		changed = true
	}

	seenIDs := map[string]bool{}
	normalized := make([]types.FanCurveProfile, 0, len(cfg.FanCurveProfiles))
	for i, p := range cfg.FanCurveProfiles {
		profile := p
		if profile.ID == "" || seenIDs[profile.ID] {
			profile.ID = generateCurveProfileID()
			changed = true
		}
		seenIDs[profile.ID] = true

		fallbackName := fmt.Sprintf("方案%d", i+1)
		name := normalizeCurveProfileName(profile.Name, fallbackName)
		if name != profile.Name {
			profile.Name = name
			changed = true
		}

		if err := cfgpkg.ValidateFanCurve(profile.Curve); err != nil {
			profile.Curve = cloneFanCurve(baseCurve)
			changed = true
		}
		normalized = append(normalized, types.FanCurveProfile{
			ID:    profile.ID,
			Name:  profile.Name,
			Curve: cloneFanCurve(profile.Curve),
		})
	}

	cfg.FanCurveProfiles = normalized
	if len(cfg.FanCurveProfiles) == 0 {
		cfg.FanCurveProfiles = []types.FanCurveProfile{{
			ID:    "default",
			Name:  "默认",
			Curve: cloneFanCurve(baseCurve),
		}}
		changed = true
	}

	if findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID) < 0 {
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[0].ID
		changed = true
	}

	activeIdx := findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if activeIdx < 0 {
		activeIdx = 0
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[0].ID
		changed = true
	}
	activeCurve := cloneFanCurve(cfg.FanCurveProfiles[activeIdx].Curve)
	if len(activeCurve) == 0 {
		activeCurve = types.GetDefaultFanCurve()
		cfg.FanCurveProfiles[activeIdx].Curve = cloneFanCurve(activeCurve)
		changed = true
	}

	if len(cfg.FanCurve) != len(activeCurve) {
		cfg.FanCurve = cloneFanCurve(activeCurve)
		changed = true
	} else {
		for i := range cfg.FanCurve {
			if cfg.FanCurve[i] != activeCurve[i] {
				cfg.FanCurve = cloneFanCurve(activeCurve)
				changed = true
				break
			}
		}
	}

	return changed
}

func (a *CoreApp) fanCurveProfilesPayloadFromConfig(cfg types.AppConfig) types.FanCurveProfilesPayload {
	return types.FanCurveProfilesPayload{
		Profiles: cloneFanCurveProfiles(cfg.FanCurveProfiles),
		ActiveID: cfg.ActiveFanCurveProfileID,
	}
}

func (a *CoreApp) applyCurveProfilesConfig(cfg types.AppConfig) error {
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return nil
}

func (a *CoreApp) GetFanCurveProfiles() types.FanCurveProfilesPayload {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if normalizeCurveProfilesConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存温控曲线方案默认配置失败: %v", err)
		}
	}
	return a.fanCurveProfilesPayloadFromConfig(cfg)
}

func (a *CoreApp) SetActiveFanCurveProfile(profileID string) (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	normalizeCurveProfilesConfig(&cfg)

	idx := findCurveProfileIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		return types.FanCurveProfile{}, fmt.Errorf("未找到温控曲线方案")
	}

	cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[idx].ID
	cfg.FanCurve = cloneFanCurve(cfg.FanCurveProfiles[idx].Curve)
	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}
	return types.FanCurveProfile{
		ID:    cfg.FanCurveProfiles[idx].ID,
		Name:  cfg.FanCurveProfiles[idx].Name,
		Curve: cloneFanCurve(cfg.FanCurveProfiles[idx].Curve),
	}, nil
}

func (a *CoreApp) CycleFanCurveProfile() (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	normalizeCurveProfilesConfig(&cfg)

	if len(cfg.FanCurveProfiles) == 0 {
		return types.FanCurveProfile{}, fmt.Errorf("暂无可用温控曲线方案")
	}

	idx := max(findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID), 0)
	nextIdx := (idx + 1) % len(cfg.FanCurveProfiles)
	cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[nextIdx].ID
	cfg.FanCurve = cloneFanCurve(cfg.FanCurveProfiles[nextIdx].Curve)

	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}

	return types.FanCurveProfile{
		ID:    cfg.FanCurveProfiles[nextIdx].ID,
		Name:  cfg.FanCurveProfiles[nextIdx].Name,
		Curve: cloneFanCurve(cfg.FanCurveProfiles[nextIdx].Curve),
	}, nil
}

func (a *CoreApp) SaveFanCurveProfile(params ipc.SaveFanCurveProfileParams) (types.FanCurveProfile, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	normalizeCurveProfilesConfig(&cfg)

	curve := cloneFanCurve(params.Curve)
	if err := cfgpkg.ValidateFanCurve(curve); err != nil {
		return types.FanCurveProfile{}, err
	}

	profileName := normalizeCurveProfileName(params.Name, "新曲线")
	profileID := strings.TrimSpace(params.ID)
	idx := findCurveProfileIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		profileID = generateCurveProfileID()
		cfg.FanCurveProfiles = append(cfg.FanCurveProfiles, types.FanCurveProfile{
			ID:    profileID,
			Name:  profileName,
			Curve: curve,
		})
		idx = len(cfg.FanCurveProfiles) - 1
	} else {
		cfg.FanCurveProfiles[idx].Name = profileName
		cfg.FanCurveProfiles[idx].Curve = curve
	}

	if params.SetActive || cfg.ActiveFanCurveProfileID == cfg.FanCurveProfiles[idx].ID {
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[idx].ID
		cfg.FanCurve = cloneFanCurve(cfg.FanCurveProfiles[idx].Curve)
	}

	if err := a.applyCurveProfilesConfig(cfg); err != nil {
		return types.FanCurveProfile{}, err
	}

	updated := cfg.FanCurveProfiles[idx]
	return types.FanCurveProfile{ID: updated.ID, Name: updated.Name, Curve: cloneFanCurve(updated.Curve)}, nil
}

func (a *CoreApp) DeleteFanCurveProfile(profileID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	normalizeCurveProfilesConfig(&cfg)

	if len(cfg.FanCurveProfiles) <= 1 {
		return fmt.Errorf("至少保留一个温控曲线方案")
	}

	idx := findCurveProfileIndex(cfg.FanCurveProfiles, profileID)
	if idx < 0 {
		return fmt.Errorf("未找到温控曲线方案")
	}

	cfg.FanCurveProfiles = append(cfg.FanCurveProfiles[:idx], cfg.FanCurveProfiles[idx+1:]...)
	if len(cfg.FanCurveProfiles) == 0 {
		return fmt.Errorf("至少保留一个温控曲线方案")
	}

	if cfg.ActiveFanCurveProfileID == profileID {
		nextIdx := idx
		if nextIdx >= len(cfg.FanCurveProfiles) {
			nextIdx = len(cfg.FanCurveProfiles) - 1
		}
		cfg.ActiveFanCurveProfileID = cfg.FanCurveProfiles[nextIdx].ID
		cfg.FanCurve = cloneFanCurve(cfg.FanCurveProfiles[nextIdx].Curve)
	}

	return a.applyCurveProfilesConfig(cfg)
}

func (a *CoreApp) ExportFanCurveProfiles() (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	normalizeCurveProfilesConfig(&cfg)
	if idx := findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = cloneFanCurve(cfg.FanCurve)
	}

	payload := fanCurveExportPayload{
		V:        1,
		Active:   cfg.ActiveFanCurveProfileID,
		Profiles: cloneFanCurveProfiles(cfg.FanCurveProfiles),
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(plain); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}

	return curveExportPrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func (a *CoreApp) ImportFanCurveProfiles(code string) error {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return fmt.Errorf("导入字符串不能为空")
	}
	if !strings.HasPrefix(trimmed, curveExportPrefix) {
		return fmt.Errorf("导入字符串格式错误")
	}

	raw := strings.TrimPrefix(trimmed, curveExportPrefix)
	compressed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return fmt.Errorf("导入字符串解码失败")
	}

	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("导入字符串解压失败")
	}
	defer zr.Close()

	plain, err := io.ReadAll(zr)
	if err != nil {
		return fmt.Errorf("导入数据读取失败")
	}

	var payload fanCurveExportPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return fmt.Errorf("导入数据格式错误")
	}
	if payload.V != 1 {
		return fmt.Errorf("不支持的导入版本")
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.FanCurveProfiles = cloneFanCurveProfiles(payload.Profiles)
	cfg.ActiveFanCurveProfileID = strings.TrimSpace(payload.Active)
	if normalizeCurveProfilesConfig(&cfg) {
		// normalized in place
	}

	return a.applyCurveProfilesConfig(cfg)
}
