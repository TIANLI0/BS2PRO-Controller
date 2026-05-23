package temperature

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

const (
	DefaultHistoryCapacity              = 360
	DefaultHistorySampleInterval        = 5 * time.Second
	DefaultHistoryRelativePath          = "telemetry/history.bin"
	historyBinaryMagic                  = "THST"
	historyBinaryVersion         uint16 = 1
	historyEnabledFlag           uint8  = 1
)

type HistoryRecorder struct {
	mutex          sync.RWMutex
	logger         types.Logger
	filePath       string
	enabled        bool
	capacity       int
	sampleInterval time.Duration
	points         []types.TemperatureHistoryPoint
	next           int
	filled         bool
	lastSampleAt   int64
}

func NewHistoryRecorder(filePath string, capacity int, sampleInterval time.Duration, logger types.Logger) *HistoryRecorder {
	if capacity <= 0 {
		capacity = DefaultHistoryCapacity
	}
	if sampleInterval <= 0 {
		sampleInterval = DefaultHistorySampleInterval
	}

	recorder := &HistoryRecorder{
		logger:         logger,
		filePath:       filePath,
		capacity:       capacity,
		sampleInterval: sampleInterval,
		enabled:        false,
		points:         make([]types.TemperatureHistoryPoint, capacity),
	}
	recorder.load()
	return recorder
}

func (r *HistoryRecorder) SetEnabled(enabled bool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.enabled = enabled
	if !enabled {
		r.clearLocked()
	}
	return r.saveLocked()
}

func (r *HistoryRecorder) IsEnabled() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.enabled
}

func (r *HistoryRecorder) Add(temp types.TemperatureData, fanData *types.FanData) (types.TemperatureHistoryPoint, bool) {
	if temp.CPUTemp <= 0 && temp.GPUTemp <= 0 {
		return types.TemperatureHistoryPoint{}, false
	}

	timestamp := normalizeTimestampMillis(temp.UpdateTime)
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}

	fanRPM := 0
	if fanData != nil {
		fanRPM = int(fanData.CurrentRPM)
	}

	point := types.TemperatureHistoryPoint{
		Timestamp: timestamp,
		CPUTemp:   temp.CPUTemp,
		GPUTemp:   temp.GPUTemp,
		FanRPM:    fanRPM,
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.enabled {
		return types.TemperatureHistoryPoint{}, false
	}
	if r.lastSampleAt > 0 && timestamp-r.lastSampleAt < r.sampleInterval.Milliseconds() {
		return types.TemperatureHistoryPoint{}, false
	}

	r.points[r.next] = point
	r.lastSampleAt = timestamp
	r.next = (r.next + 1) % r.capacity
	if r.next == 0 {
		r.filled = true
	}
	if err := r.saveLocked(); err != nil {
		r.logError("保存温度历史失败: %v", err)
	}
	return point, true
}

func (r *HistoryRecorder) Snapshot() types.TemperatureHistoryPayload {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return types.TemperatureHistoryPayload{
		Enabled:               r.enabled,
		SampleIntervalSeconds: int(r.sampleInterval / time.Second),
		Points:                r.snapshotPointsLocked(),
	}
}

func normalizeTimestampMillis(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}
	if timestamp < 1_000_000_000_000 {
		return timestamp * 1000
	}
	return timestamp
}

func (r *HistoryRecorder) load() {
	if r.filePath == "" {
		return
	}

	if err := r.loadBinaryFile(r.filePath); err == nil {
		return
	} else if !os.IsNotExist(err) {
		r.logError("解析温度历史文件失败: %v", err)
	}
}

func (r *HistoryRecorder) loadBinaryFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return r.loadBinaryData(data)
}

func (r *HistoryRecorder) loadBinaryData(data []byte) error {
	reader := bytes.NewReader(data)
	magic := make([]byte, len(historyBinaryMagic))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return err
	}
	if string(magic) != historyBinaryMagic {
		return io.ErrUnexpectedEOF
	}

	var version uint16
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version != historyBinaryVersion {
		return fmt.Errorf("unsupported history version: %d", version)
	}

	var flags uint8
	var reserved uint8
	var sampleIntervalSeconds uint32
	var count uint32
	var updatedAt int64
	if err := binary.Read(reader, binary.LittleEndian, &flags); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &reserved); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &sampleIntervalSeconds); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &updatedAt); err != nil {
		return err
	}

	points := make([]types.TemperatureHistoryPoint, 0, count)
	for i := uint32(0); i < count; i++ {
		var timestamp int64
		var cpuTemp int32
		var gpuTemp int32
		var fanRPM int32
		if err := binary.Read(reader, binary.LittleEndian, &timestamp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &cpuTemp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &gpuTemp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &fanRPM); err != nil {
			return err
		}
		points = append(points, types.TemperatureHistoryPoint{
			Timestamp: normalizeTimestampMillis(timestamp),
			CPUTemp:   int(cpuTemp),
			GPUTemp:   int(gpuTemp),
			FanRPM:    int(fanRPM),
		})
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.enabled = flags&historyEnabledFlag != 0
	if sampleIntervalSeconds > 0 {
		r.sampleInterval = time.Duration(sampleIntervalSeconds) * time.Second
	}
	r.applyLoadedPointsLocked(points)
	return nil
}

func (r *HistoryRecorder) applyLoadedPointsLocked(points []types.TemperatureHistoryPoint) {
	if len(points) > r.capacity {
		points = points[len(points)-r.capacity:]
	}
	for i := range r.points {
		r.points[i] = types.TemperatureHistoryPoint{}
	}
	copy(r.points, points)
	r.next = len(points)
	if r.next >= r.capacity {
		r.next = 0
		r.filled = true
	} else {
		r.filled = len(points) == r.capacity
	}
	if len(points) > 0 {
		r.lastSampleAt = points[len(points)-1].Timestamp
	} else {
		r.lastSampleAt = 0
	}
}

func (r *HistoryRecorder) snapshotPointsLocked() []types.TemperatureHistoryPoint {
	points := make([]types.TemperatureHistoryPoint, 0, r.capacity)
	if r.filled {
		points = append(points, r.points[r.next:]...)
		points = append(points, r.points[:r.next]...)
	} else {
		points = append(points, r.points[:r.next]...)
	}
	return points
}

func (r *HistoryRecorder) saveLocked() error {
	if r.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return err
	}
	snapshot := r.snapshotPointsLocked()
	var flags uint8
	if r.enabled {
		flags |= historyEnabledFlag
	}
	buffer := bytes.NewBuffer(make([]byte, 0, 24+len(snapshot)*20))
	if _, err := buffer.WriteString(historyBinaryMagic); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, historyBinaryVersion); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, flags); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, uint8(0)); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, uint32(r.sampleInterval/time.Second)); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, uint32(len(snapshot))); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, time.Now().UnixMilli()); err != nil {
		return err
	}
	for _, point := range snapshot {
		if err := binary.Write(buffer, binary.LittleEndian, normalizeTimestampMillis(point.Timestamp)); err != nil {
			return err
		}
		if err := binary.Write(buffer, binary.LittleEndian, int32(point.CPUTemp)); err != nil {
			return err
		}
		if err := binary.Write(buffer, binary.LittleEndian, int32(point.GPUTemp)); err != nil {
			return err
		}
		if err := binary.Write(buffer, binary.LittleEndian, int32(point.FanRPM)); err != nil {
			return err
		}
	}
	data := buffer.Bytes()
	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	_ = os.Remove(r.filePath)
	if err := os.Rename(tmpPath, r.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return os.WriteFile(r.filePath, data, 0644)
	}
	return nil
}

func (r *HistoryRecorder) clearLocked() {
	for i := range r.points {
		r.points[i] = types.TemperatureHistoryPoint{}
	}
	r.next = 0
	r.filled = false
	r.lastSampleAt = 0
}

func (r *HistoryRecorder) logError(format string, args ...any) {
	if r.logger != nil {
		r.logger.Error(format, args...)
	}
}
