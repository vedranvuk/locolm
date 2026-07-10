package sysinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	// No specific configuration needed for sysinfo tool
}

func DefaultConfig() *Config {
	return &Config{}
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type SysInfo struct {
	config    *Config
	startTime time.Time
}

func New(config *Config) (*SysInfo, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &SysInfo{
		config:    config,
		startTime: time.Now(),
	}, nil
}

func (self *SysInfo) Register(r mcp.Registry) {
	r.RegisterTool(
		"sys_info",
		"Report system and environment details (OS, CPU, memory, disk, Go runtime, environment). Call this first in every conversation to establish context.",
		json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
		self.Run,
	)
}

func (self *SysInfo) Run(_ map[string]string) (string, error) {
	now := time.Now()
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	uptime := now.Sub(self.startTime).Round(time.Second).String()

	var b strings.Builder

	fmt.Fprintf(&b, "### System Runtime\n")
	fmt.Fprintf(&b, "- **Date/Time**: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "- **Uptime**: %s\n", uptime)
	fmt.Fprintf(&b, "- **OS/Arch**: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "- **Hostname**: %s\n", hostname)
	fmt.Fprintf(&b, "- **User**: %s\n", os.Getenv("USER"))
	fmt.Fprintf(&b, "- **CWD**: %s\n", cwd)
	fmt.Fprintf(&b, "- **Temp Dir**: %s\n", os.TempDir())
	fmt.Fprintf(&b, "- **Path Separator**: %q\n", string(os.PathSeparator))
	fmt.Fprintf(&b, "- **List Separator**: %q\n", string(os.PathListSeparator))

	// --- CPU ---
	fmt.Fprintf(&b, "\n### CPU\n")
	fmt.Fprintf(&b, "- **Logical CPUs**: %d\n", runtime.NumCPU())
	writeCPUInfo(&b)

	// --- Memory ---
	fmt.Fprintf(&b, "\n### Memory\n")
	writeMemoryInfo(&b)

	// --- Disk (filesystems mounted under CWD and root) ---
	fmt.Fprintf(&b, "\n### Disk\n")
	for _, path := range uniqueStrings([]string{diskRootPath(), cwd}) {
		writeDiskInfo(&b, path)
	}

	// --- Go Runtime ---
	fmt.Fprintf(&b, "\n### Go Runtime\n")
	fmt.Fprintf(&b, "- **Go Version**: %s\n", runtime.Version())
	fmt.Fprintf(&b, "- **Compiler**: %s\n", runtime.Compiler)
	fmt.Fprintf(&b, "- **Goroutines**: %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "- **GOMAXPROCS**: %d\n", runtime.GOMAXPROCS(0))
	fmt.Fprintf(&b, "- **NumCPU**: %d\n", runtime.NumCPU())
	fmt.Fprintf(&b, "- **GOOS/GOARCH**: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fmt.Fprintf(&b, "- **Heap Alloc**: %s\n", humanBytes(memStats.HeapAlloc))
	fmt.Fprintf(&b, "- **Heap Sys**: %s\n", humanBytes(memStats.HeapSys))
	fmt.Fprintf(&b, "- **Total Alloc**: %s\n", humanBytes(memStats.TotalAlloc))
	fmt.Fprintf(&b, "- **Sys**: %s\n", humanBytes(memStats.Sys))
	fmt.Fprintf(&b, "- **GC Cycles**: %d\n", memStats.NumGC)
	if bi, ok := debug.ReadBuildInfo(); ok {
		fmt.Fprintf(&b, "- **Build Settings**:\n")
		for _, s := range bi.Settings {
			if s.Value == "" {
				continue
			}
			switch s.Key {
			case "vcs", "vcs.revision", "vcs.time", "vcs.modified":
				fmt.Fprintf(&b, "  - %s: %s\n", s.Key, s.Value)
			}
		}
	}

	// --- Environment (non-secret) ---
	fmt.Fprintf(&b, "\n### Environment\n")
	for _, key := range safeEnvKeys {
		if v := os.Getenv(key); v != "" {
			fmt.Fprintf(&b, "- **%s**: %s\n", key, v)
		}
	}

	return b.String(), nil
}

// safeEnvKeys lists environment variables that are safe to expose (no secrets).
var safeEnvKeys = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TERM", "LANG", "LC_ALL",
	"TZ", "PWD", "OLDPWD", "HOSTNAME", "EDITOR", "PAGER", "GOVERSION",
	"GOPATH", "GOROOT", "GOMODCACHE", "GOFLAGS", "CGO_ENABLED", "OS",
	"TMPDIR", "TEMP", "TMP", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP",
	"DESKTOP_SESSION", "DISPLAY", "WAYLAND_DISPLAY", "WINDOW_MANAGER",
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func percent(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
