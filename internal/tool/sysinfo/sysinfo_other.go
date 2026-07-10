//go:build !linux && !windows

package sysinfo

import (
	"fmt"
	"runtime"
	"strings"
)

func diskRootPath() string { return "/" }

func writeCPUInfo(b *strings.Builder) {
	fmt.Fprintf(b, "- **Platform**: %s\n", runtime.GOOS)
}

func writeMemoryInfo(b *strings.Builder) {
	fmt.Fprintf(b, "- **Note**: detailed memory info not available on %s\n", runtime.GOOS)
}

func writeDiskInfo(b *strings.Builder, path string) {
	fmt.Fprintf(b, "- **%s**: disk details not available on %s\n", path, runtime.GOOS)
}
