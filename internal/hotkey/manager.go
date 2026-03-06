package hotkey

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	hotkeylib "golang.design/x/hotkey"
)

// Action 定义快捷键触发的动作。
type Action string

const (
	ActionToggleManualGear Action = "toggle-manual-gear"
	ActionToggleAutoMode   Action = "toggle-auto-control"
)

// Manager 负责注册/更新/注销全局快捷键。
type Manager struct {
	logger   types.Logger
	onAction func(action Action, shortcut string)

	mutex    sync.Mutex
	bindings map[Action]*binding
	closed   bool
}

type binding struct {
	shortcut string
	hk       *hotkeylib.Hotkey
	stopChan chan struct{}
}

// NewManager 创建快捷键管理器。
func NewManager(logger types.Logger, onAction func(action Action, shortcut string)) *Manager {
	return &Manager{
		logger:   logger,
		onAction: onAction,
		bindings: make(map[Action]*binding),
	}
}

// UpdateBindings 根据配置更新快捷键绑定。
func (m *Manager) UpdateBindings(manualGearShortcut, autoControlShortcut string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		return fmt.Errorf("hotkey manager already stopped")
	}

	var errs []string
	if err := m.setBinding(ActionToggleManualGear, manualGearShortcut); err != nil {
		errs = append(errs, err.Error())
	}
	if err := m.setBinding(ActionToggleAutoMode, autoControlShortcut); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// Stop 释放所有快捷键资源。
func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		return
	}
	m.closed = true

	for action := range m.bindings {
		m.unregisterBinding(action)
	}
}

func (m *Manager) setBinding(action Action, shortcut string) error {
	shortcut = normalizeShortcut(shortcut)

	if existing, ok := m.bindings[action]; ok {
		if existing.shortcut == shortcut {
			return nil
		}
		m.unregisterBinding(action)
	}

	if shortcut == "" {
		return nil
	}

	mods, key, err := ParseShortcut(shortcut)
	if err != nil {
		return fmt.Errorf("%s 快捷键无效: %w", action, err)
	}

	hk := hotkeylib.New(mods, key)
	if err := hk.Register(); err != nil {
		return fmt.Errorf("%s 注册失败(%s): %w", action, shortcut, err)
	}

	b := &binding{
		shortcut: shortcut,
		hk:       hk,
		stopChan: make(chan struct{}),
	}
	m.bindings[action] = b

	go m.listen(action, b)
	m.logInfo("快捷键已注册: %s -> %s", action, shortcut)
	return nil
}

func (m *Manager) unregisterBinding(action Action) {
	b, ok := m.bindings[action]
	if !ok || b == nil {
		return
	}
	close(b.stopChan)
	if err := b.hk.Unregister(); err != nil {
		m.logDebug("快捷键注销失败: %s (%v)", action, err)
	}
	delete(m.bindings, action)
}

func (m *Manager) listen(action Action, b *binding) {
	for {
		select {
		case <-b.stopChan:
			return
		case <-b.hk.Keydown():
			if m.onAction != nil {
				m.onAction(action, b.shortcut)
			}
		}
	}
}

// ParseShortcut 解析快捷键字符串，支持 Ctrl/Alt/Shift/Win + A-Z/0-9/F1-F12。
func ParseShortcut(input string) ([]hotkeylib.Modifier, hotkeylib.Key, error) {
	normalized := normalizeShortcut(input)
	if normalized == "" {
		return nil, hotkeylib.Key(0), fmt.Errorf("empty shortcut")
	}

	parts := strings.Split(normalized, "+")
	mods := make([]hotkeylib.Modifier, 0, 3)
	modSet := map[hotkeylib.Modifier]bool{}
	key := hotkeylib.Key(0)
	keySet := false

	for _, part := range parts {
		if part == "" {
			continue
		}

		if mod, ok := parseModifier(part); ok {
			if !modSet[mod] {
				mods = append(mods, mod)
				modSet[mod] = true
			}
			continue
		}

		if keySet {
			return nil, hotkeylib.Key(0), fmt.Errorf("multiple main keys")
		}

		parsedKey, err := parseKey(part)
		if err != nil {
			return nil, hotkeylib.Key(0), err
		}
		key = parsedKey
		keySet = true
	}

	if !keySet {
		return nil, hotkeylib.Key(0), fmt.Errorf("missing main key")
	}
	if len(mods) == 0 {
		return nil, hotkeylib.Key(0), fmt.Errorf("missing modifier")
	}

	return mods, key, nil
}

func parseModifier(part string) (hotkeylib.Modifier, bool) {
	switch strings.ToUpper(strings.TrimSpace(part)) {
	case "CTRL", "CONTROL":
		return hotkeylib.ModCtrl, true
	case "ALT":
		return hotkeylib.ModAlt, true
	case "SHIFT":
		return hotkeylib.ModShift, true
	case "WIN", "WINDOWS", "SUPER":
		return hotkeylib.ModWin, true
	default:
		return hotkeylib.Modifier(0), false
	}
}

func parseKey(part string) (hotkeylib.Key, error) {
	token := strings.ToUpper(strings.TrimSpace(part))

	if len(token) == 1 {
		switch token[0] {
		case 'A':
			return hotkeylib.KeyA, nil
		case 'B':
			return hotkeylib.KeyB, nil
		case 'C':
			return hotkeylib.KeyC, nil
		case 'D':
			return hotkeylib.KeyD, nil
		case 'E':
			return hotkeylib.KeyE, nil
		case 'F':
			return hotkeylib.KeyF, nil
		case 'G':
			return hotkeylib.KeyG, nil
		case 'H':
			return hotkeylib.KeyH, nil
		case 'I':
			return hotkeylib.KeyI, nil
		case 'J':
			return hotkeylib.KeyJ, nil
		case 'K':
			return hotkeylib.KeyK, nil
		case 'L':
			return hotkeylib.KeyL, nil
		case 'M':
			return hotkeylib.KeyM, nil
		case 'N':
			return hotkeylib.KeyN, nil
		case 'O':
			return hotkeylib.KeyO, nil
		case 'P':
			return hotkeylib.KeyP, nil
		case 'Q':
			return hotkeylib.KeyQ, nil
		case 'R':
			return hotkeylib.KeyR, nil
		case 'S':
			return hotkeylib.KeyS, nil
		case 'T':
			return hotkeylib.KeyT, nil
		case 'U':
			return hotkeylib.KeyU, nil
		case 'V':
			return hotkeylib.KeyV, nil
		case 'W':
			return hotkeylib.KeyW, nil
		case 'X':
			return hotkeylib.KeyX, nil
		case 'Y':
			return hotkeylib.KeyY, nil
		case 'Z':
			return hotkeylib.KeyZ, nil
		case '0':
			return hotkeylib.Key0, nil
		case '1':
			return hotkeylib.Key1, nil
		case '2':
			return hotkeylib.Key2, nil
		case '3':
			return hotkeylib.Key3, nil
		case '4':
			return hotkeylib.Key4, nil
		case '5':
			return hotkeylib.Key5, nil
		case '6':
			return hotkeylib.Key6, nil
		case '7':
			return hotkeylib.Key7, nil
		case '8':
			return hotkeylib.Key8, nil
		case '9':
			return hotkeylib.Key9, nil
		}
	}

	if strings.HasPrefix(token, "F") {
		n, err := strconv.Atoi(strings.TrimPrefix(token, "F"))
		if err == nil {
			switch n {
			case 1:
				return hotkeylib.KeyF1, nil
			case 2:
				return hotkeylib.KeyF2, nil
			case 3:
				return hotkeylib.KeyF3, nil
			case 4:
				return hotkeylib.KeyF4, nil
			case 5:
				return hotkeylib.KeyF5, nil
			case 6:
				return hotkeylib.KeyF6, nil
			case 7:
				return hotkeylib.KeyF7, nil
			case 8:
				return hotkeylib.KeyF8, nil
			case 9:
				return hotkeylib.KeyF9, nil
			case 10:
				return hotkeylib.KeyF10, nil
			case 11:
				return hotkeylib.KeyF11, nil
			case 12:
				return hotkeylib.KeyF12, nil
			}
		}
	}

	return hotkeylib.Key(0), fmt.Errorf("unsupported key: %s", part)
}

func normalizeShortcut(input string) string {
	parts := strings.Split(strings.TrimSpace(input), "+")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, "+")
}

func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}
