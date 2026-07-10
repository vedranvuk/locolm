//go:build windows

package sysinfo

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func diskRootPath() string { return `C:\` }

func writeCPUInfo(b *strings.Builder) {
	// GetActiveProcessorCount(ALL_PROCESSOR_GROUPS = 0xffff) returns logical processors.
	const allProcessorGroups = 0xffff
	fmt.Fprintf(b, "- **Logical CPUs (Windows)**: %d\n", windows.GetActiveProcessorCount(allProcessorGroups))
}

func writeMemoryInfo(b *strings.Builder) {
	// GlobalMemoryStatusEx is not wrapped by x/sys/windows; call it directly.
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GlobalMemoryStatusEx")

	type memoryStatusEx struct {
		Length               uint32
		MemoryLoad           uint32
		TotalPhys            uint64
		AvailPhys            uint64
		TotalPageFile        uint64
		AvailPageFile        uint64
		TotalVirtual         uint64
		AvailVirtual         uint64
		AvailExtendedVirtual uint64
	}
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r1, _, err := proc.Call(uintptr(unsafe.Pointer(&m)))
	if r1 == 0 {
		fmt.Fprintf(b, "- **Note**: failed to query memory: %v\n", err)
		return
	}
	total := m.TotalPhys
	avail := m.AvailPhys
	used := total - avail
	fmt.Fprintf(b, "- **Total**: %s\n", humanBytes(total))
	fmt.Fprintf(b, "- **Available**: %s\n", humanBytes(avail))
	fmt.Fprintf(b, "- **Used**: %s (%.1f%%)\n", humanBytes(used), percent(used, total))
	fmt.Fprintf(b, "- **Load (%% in use)**: %d\n", m.MemoryLoad)
	fmt.Fprintf(b, "- **Total Page File**: %s\n", humanBytes(m.TotalPageFile))
	fmt.Fprintf(b, "- **Avail Page File**: %s\n", humanBytes(m.AvailPageFile))
}

func writeDiskInfo(b *strings.Builder, path string) {
	var freeBytes, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(path), &freeBytes, &totalBytes, &totalFreeBytes); err != nil {
		return
	}
	avail := freeBytes
	fmt.Fprintf(b, "- **%s**: %s total, %s free, %s avail (%.1f%% used)\n",
		path, humanBytes(totalBytes), humanBytes(totalFreeBytes), humanBytes(avail), percent(totalBytes-avail, totalBytes))
}
