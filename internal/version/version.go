package version

import (
	"strings"
)

// BuildVersion 在编译时通过 ldflags 注入版本号
// 示例: go build -ldflags "-X github.com/TIANLI0/THRM/internal/version.BuildVersion=2.1.0"
var BuildVersion = "dev"

// Get 返回应用版本号
// 优先使用编译时注入的版本号，如果未注入则返回 "dev"
func Get() string {
	if v := strings.TrimSpace(BuildVersion); v != "" {
		return v
	}
	return "dev"
}
