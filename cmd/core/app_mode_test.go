package main

import "testing"

func TestDidDeviceSwitchToManualMode(t *testing.T) {
	tests := []struct {
		name         string
		previousMode string
		currentMode  string
		want         bool
	}{
		{name: "detects auto to manual transition", previousMode: "自动模式(实时转速)", currentMode: "挡位工作模式", want: true},
		{name: "ignores startup manual snapshot", previousMode: "", currentMode: "挡位工作模式", want: false},
		{name: "ignores repeated manual updates", previousMode: "挡位工作模式", currentMode: "挡位工作模式", want: false},
		{name: "ignores non-manual current mode", previousMode: "自动模式(实时转速)", currentMode: "自动模式(实时转速)", want: false},
		{name: "detects unknown to manual transition", previousMode: "未知模式(0x0D)", currentMode: "挡位工作模式", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := didDeviceSwitchToManualMode(test.previousMode, test.currentMode); got != test.want {
				t.Fatalf("didDeviceSwitchToManualMode(%q, %q) = %v, want %v", test.previousMode, test.currentMode, got, test.want)
			}
		})
	}
}
