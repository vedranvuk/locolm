//go:build linux

package sysinfo

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func diskRootPath() string { return "/" }

func writeCPUInfo(b *strings.Builder) {
	if data, err := os.ReadFile("/sys/devices/system/cpu/online"); err == nil {
		if n := countOnlineCPUs(strings.TrimSpace(string(data))); n > 0 {
			fmt.Fprintf(b, "- **Online Processors**: %d\n", n)
		}
	}
	var info unix.Utsname
	if err := unix.Uname(&info); err == nil {
		fmt.Fprintf(b, "- **Sysname**: %s\n", unix.ByteSliceToString(info.Sysname[:]))
		fmt.Fprintf(b, "- **Release**: %s\n", unix.ByteSliceToString(info.Release[:]))
		fmt.Fprintf(b, "- **Version**: %s\n", unix.ByteSliceToString(info.Version[:]))
		fmt.Fprintf(b, "- **Machine**: %s\n", unix.ByteSliceToString(info.Machine[:]))
	}
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			fmt.Fprintf(b, "- **Load Average (1/5/15)**: %s / %s / %s\n", fields[0], fields[1], fields[2])
		}
	}
}

// countOnlineCPUs parses Linux CPU online masks like "0-3" or "0,2-3,7".
func countOnlineCPUs(s string) int {
	total := 0
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.IndexByte(part, '-'); i >= 0 {
			lo, err1 := strconv.Atoi(strings.TrimSpace(part[:i]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(part[i+1:]))
			if err1 == nil && err2 == nil && hi >= lo {
				total += hi - lo + 1
			}
		} else if n, err := strconv.Atoi(part); err == nil {
			_ = n
			total++
		}
	}
	return total
}

func writeMemoryInfo(b *strings.Builder) {
	var si unix.Sysinfo_t
	if err := unix.Sysinfo(&si); err != nil {
		return
	}
	unit := uint64(si.Unit)
	total := si.Totalram * unit
	free := si.Freeram * unit
	avail := si.Freeram * unit // Linux Sysinfo_t has no Availram; approximate with free.
	used := total - avail
	fmt.Fprintf(b, "- **Total**: %s\n", humanBytes(total))
	fmt.Fprintf(b, "- **Free**: %s\n", humanBytes(free))
	fmt.Fprintf(b, "- **Available**: %s\n", humanBytes(avail))
	fmt.Fprintf(b, "- **Used**: %s (%.1f%%)\n", humanBytes(used), percent(used, total))
	fmt.Fprintf(b, "- **Shared**: %s\n", humanBytes(si.Sharedram*unit))
	fmt.Fprintf(b, "- **Buffer/Cache**: %s\n", humanBytes(si.Bufferram*unit))
	fmt.Fprintf(b, "- **Swap Total**: %s\n", humanBytes(si.Totalswap*unit))
	fmt.Fprintf(b, "- **Swap Free**: %s\n", humanBytes(si.Freeswap*unit))
	fmt.Fprintf(b, "- **Procs**: %d\n", si.Procs)
}

func writeDiskInfo(b *strings.Builder, path string) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	fmt.Fprintf(b, "- **%s**: %s total, %s free, %s avail (%.1f%% used)\n",
		path, humanBytes(total), humanBytes(free), humanBytes(avail), percent(total-avail, total))
}
