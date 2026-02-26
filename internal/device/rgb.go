package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

const (
	lightSpeedFast   byte = 0x05
	lightSpeedMedium byte = 0x0A
	lightSpeedSlow   byte = 0x0F
)

// SetLightStrip 设置灯带模式
func (m *Manager) SetLightStrip(cfg types.LightStripConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return fmt.Errorf("设备未连接")
	}

	brightness := clampLightBrightness(cfg.Brightness)
	speed := parseLightSpeed(cfg.Speed)

	switch cfg.Mode {
	case "off":
		return m.setRGBOffLocked()
	case "smart_temp":
		return m.setLightSmartTempLocked()
	case "static_single":
		color := firstOrDefaultColor(cfg.Colors)
		return m.setLightStaticSingleLocked(color, brightness)
	case "static_multi":
		colors := toThreeColors(cfg.Colors)
		return m.setLightStaticMultiLocked(colors, brightness)
	case "rotation":
		colors := ensureMinColors(cfg.Colors, 1)
		return m.setLightRotationLocked(colors, speed, brightness)
	case "flowing":
		return m.setLightFlowingLocked(speed, brightness)
	case "breathing":
		colors := ensureMinColors(cfg.Colors, 1)
		return m.setLightBreathingLocked(colors, speed, brightness)
	default:
		return fmt.Errorf("未知灯带模式: %s", cfg.Mode)
	}
}

// SetRGBOff 关闭RGB灯光
func (m *Manager) SetRGBOff() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	err := m.setRGBOffLocked()
	return err == nil
}

func (m *Manager) setRGBOffLocked() error {
	return m.sendLightCommandLocked(0x46, 0x03, 0x00)
}

func clampLightBrightness(value int) byte {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return byte(value)
}

func parseLightSpeed(speed string) byte {
	switch speed {
	case "fast":
		return lightSpeedFast
	case "slow":
		return lightSpeedSlow
	default:
		return lightSpeedMedium
	}
}

func firstOrDefaultColor(colors []types.RGBColor) types.RGBColor {
	if len(colors) == 0 {
		return types.RGBColor{R: 255, G: 255, B: 255}
	}
	return colors[0]
}

func toThreeColors(colors []types.RGBColor) [3]types.RGBColor {
	base := [3]types.RGBColor{
		{R: 255, G: 0, B: 0},
		{R: 0, G: 255, B: 0},
		{R: 0, G: 128, B: 255},
	}
	for i := 0; i < len(base) && i < len(colors); i++ {
		base[i] = colors[i]
	}
	return base
}

func ensureMinColors(colors []types.RGBColor, min int) []types.RGBColor {
	if len(colors) >= min {
		return colors
	}
	defaults := []types.RGBColor{{R: 255, G: 0, B: 0}, {R: 0, G: 255, B: 0}, {R: 0, G: 128, B: 255}}
	result := make([]types.RGBColor, 0, min)
	result = append(result, colors...)
	for len(result) < min {
		result = append(result, defaults[(len(result))%len(defaults)])
	}
	return result
}

func lightChecksum(payload []byte) byte {
	var sum uint16
	for _, b := range payload[2:] {
		sum += uint16(b)
	}
	return byte(sum & 0xFF)
}

func (m *Manager) sendLightCommandLocked(fields ...byte) error {
	cmd := append([]byte{0x5A, 0xA5}, fields...)
	cmd = append(cmd, lightChecksum(cmd))

	buf := make([]byte, 65)
	buf[0] = 0x02
	copy(buf[1:], cmd)

	if _, err := m.device.Write(buf); err != nil {
		return err
	}
	return nil
}

func makeLightF0(mode, speed, brightness byte, baseColor types.RGBColor) [10]byte {
	return [10]byte{0x00, 0x02, 0x00, mode, speed, brightness, baseColor.R, baseColor.G, baseColor.B, 0x00}
}

func (m *Manager) applyLightFramesLocked(f0 [10]byte, frames [30][10]byte) error {
	if err := m.sendLightCommandLocked(0x46, 0x03, 0x00); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)

	handshakes := [][]byte{
		{0x46, 0x03, 0x01}, {0x46, 0x03, 0x01}, {0x45, 0x02},
		{0x45, 0x03, 0x01}, {0x41, 0x02}, {0x41, 0x03, 0x01},
	}
	for _, cmd := range handshakes {
		if err := m.sendLightCommandLocked(cmd...); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}

	f0Payload := append([]byte{0x47, 0x0D, 0x00}, f0[:]...)
	if err := m.sendLightCommandLocked(f0Payload...); err != nil {
		return err
	}

	for i := range 30 {
		framePayload := append([]byte{0x47, 0x0D, byte(i + 1)}, frames[i][:]...)
		if err := m.sendLightCommandLocked(framePayload...); err != nil {
			return err
		}
		time.Sleep(1 * time.Millisecond)
	}

	return m.sendLightCommandLocked(0x43, 0x03, 0x01)
}

func (m *Manager) setLightStaticSingleLocked(color types.RGBColor, brightness byte) error {
	f0 := makeLightF0(0x00, lightSpeedMedium, brightness, color)
	var frames [30][10]byte

	factor := float64(brightness) / 100.0
	r := byte(float64(color.R) * factor)
	g := byte(float64(color.G) * factor)
	b := byte(float64(color.B) * factor)

	targetIndices := []int{2, 5, 8, 11, 14}
	for _, idx := range targetIndices {
		frames[idx][6], frames[idx][7], frames[idx][8] = r, g, b
	}

	return m.applyLightFramesLocked(f0, frames)
}

func (m *Manager) setLightStaticMultiLocked(colors [3]types.RGBColor, brightness byte) error {
	f0 := makeLightF0(0x00, lightSpeedMedium, brightness, colors[0])
	var frames [30][10]byte
	factor := float64(brightness) / 100.0
	targetIndices := []int{2, 5, 8, 11, 14}

	for z, idx := range targetIndices {
		col := colors[(z+1)%3]
		frames[idx][6] = byte(float64(col.R) * factor)
		frames[idx][7] = byte(float64(col.G) * factor)
		frames[idx][8] = byte(float64(col.B) * factor)
	}

	return m.applyLightFramesLocked(f0, frames)
}

func (m *Manager) setLightRotationLocked(colors []types.RGBColor, speed, brightness byte) error {
	if len(colors) < 1 {
		return fmt.Errorf("旋转需要至少 1 个颜色")
	}
	if len(colors) > 6 {
		colors = colors[:6]
	}

	f0 := makeLightF0(0x05, speed, brightness, types.RGBColor{R: 0, G: 0, B: 0})
	var frames [30][10]byte

	stream := make([]byte, 304)
	numColors := len(colors)
	factor := float64(brightness) / 100.0

	for chunkIdx := range 6 {
		chunkStart := chunkIdx * 30

		for p := range 10 {
			var r, g, b byte
			if p < 6 {
				colorIdx := (p + chunkIdx) % 6
				if colorIdx < numColors {
					target := colors[colorIdx]
					r = byte(float64(target.R) * factor)
					g = byte(float64(target.G) * factor)
					b = byte(float64(target.B) * factor)
				}
			}

			stream[chunkStart+p*3] = r
			stream[chunkStart+p*3+1] = g
			stream[chunkStart+p*3+2] = b
		}
	}

	for k := range 304 {
		if k < 4 {
			f0[6+k] = stream[k]
		} else {
			idx := k - 4
			frames[idx/10][idx%10] = stream[k]
		}
	}

	return m.applyLightFramesLocked(f0, frames)
}

func (m *Manager) setLightFlowingLocked(speed, brightness byte) error {
	flowingBase := [9][10]byte{
		{0x7f, 0x7f, 0x00, 0xff, 0x00, 0x7f, 0x7f, 0x00, 0xff, 0x00},
		{0x00, 0x7f, 0x00, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x00},
		{0x00, 0x00, 0x00, 0xff, 0x00, 0x7f, 0x7f, 0x00, 0xff, 0x00},
		{0x7f, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00, 0x7f},
		{0x7f, 0x00, 0xff, 0x00, 0x00, 0x7f, 0x00, 0x7f, 0x00, 0x00},
		{0xff, 0x00, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00},
	}

	f0 := makeLightF0(0x05, speed, brightness, types.RGBColor{R: 0, G: 255, B: 0})

	factor := float64(brightness) / 100.0
	var frames [30][10]byte
	for i := range 30 {
		src := flowingBase[i%9]
		for j := range 9 {
			frames[i][j] = byte(float64(src[j]) * factor)
		}
		frames[i][9] = src[9]
	}

	return m.applyLightFramesLocked(f0, frames)
}

func (m *Manager) setLightBreathingLocked(colors []types.RGBColor, speed, brightness byte) error {
	if len(colors) == 0 {
		return fmt.Errorf("颜色列表不能为空")
	}
	if len(colors) > 5 {
		colors = colors[:5]
	}

	mode := byte(len(colors)*2 - 1)
	f0 := makeLightF0(mode, speed, brightness, types.RGBColor{R: 0, G: 0, B: 0})
	var frames [30][10]byte
	factor := float64(brightness) / 100.0

	var pattern [30]byte
	for i, col := range colors {
		offset := i * 6
		if offset+2 >= len(pattern) {
			break
		}
		pattern[offset] = byte(float64(col.R) * factor)
		pattern[offset+1] = byte(float64(col.G) * factor)
		pattern[offset+2] = byte(float64(col.B) * factor)
	}

	for k := range 304 {
		val := pattern[k%30]
		if k < 4 {
			f0[6+k] = val
		} else {
			idx := k - 4
			frames[idx/10][idx%10] = val
		}
	}

	return m.applyLightFramesLocked(f0, frames)
}

func (m *Manager) setLightSmartTempLocked() error {
	handshakes := [][]byte{
		{0x46, 0x03, 0x01}, {0x46, 0x03, 0x01}, {0x45, 0x02}, {0x45, 0x03, 0x01},
	}
	for _, cmd := range handshakes {
		if err := m.sendLightCommandLocked(cmd...); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := m.sendLightCommandLocked(0x44, 0x03, 0x01); err != nil {
		return err
	}
	time.Sleep(5 * time.Millisecond)
	return m.sendLightCommandLocked(0x43, 0x03, 0x01)
}
