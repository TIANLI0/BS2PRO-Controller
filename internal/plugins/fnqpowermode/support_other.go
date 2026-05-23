//go:build !windows

package fnqpowermode

// DetectSupport 检测当前主机是否支持 Lenovo Legion Fn+Q 插件。
func DetectSupport() (bool, HostInfo, error) {
	return false, HostInfo{}, nil
}
