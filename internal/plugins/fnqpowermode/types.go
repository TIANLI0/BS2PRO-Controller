package fnqpowermode

import "github.com/TIANLI0/THRM/internal/types"

const (
	PluginID   = "lenovo-legion-fnq-power-mode"
	PluginName = "Lenovo Legion Fn+Q Power Mode"
)

// PowerModeState is emitted when the Lenovo firmware reports a power-mode change.
type PowerModeState struct {
	Raw       int    `json:"raw"`
	Mapped    int    `json:"mapped"`
	Mode      string `json:"mode"`
	Source    string `json:"source"`
	Timestamp int64  `json:"timestamp"`
}

// HostInfo 描述当前主机的机型信息。
type HostInfo struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Family       string `json:"family"`
	Product      string `json:"product"`
	Version      string `json:"version"`
	Vendor       string `json:"vendor"`
}

type Options struct {
	Logger       types.Logger
	OnModeChange func(PowerModeState)
}
