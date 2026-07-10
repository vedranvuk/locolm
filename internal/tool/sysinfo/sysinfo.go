package sysinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
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

type SysInfoTool struct {
	config    *Config
	startTime time.Time
}

func New(config *Config) (*SysInfoTool, error) {
	if config == nil {
		config = DefaultConfig()
	}
	return &SysInfoTool{
		config:    config,
		startTime: time.Now(),
	}, nil
}

func (self *SysInfoTool) Register(r mcp.Registry) {
	r.RegisterTool(
		"sys_info",
		"MANDATORY: You MUST call this tool FIRST before any other tool in every conversation. Returns date, time, timezone, OS, architecture, hostname, working directory, user, Go version, and uptime. Do not skip this. Do not call any other tool before this one.",
		json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
		self.Run,
	)
}

func (self *SysInfoTool) Run(_ map[string]string) (string, error) {
	now := time.Now()
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	uptime := now.Sub(self.startTime).Round(time.Second).String()

	info := fmt.Sprintf(
		"### System Runtime\n"+
			"- **Date/Time**: %s\n"+
			"- **Uptime**: %s\n"+
			"- **OS/Arch**: %s/%s\n"+
			"- **Hostname**: %s\n"+
			"- **User**: %s\n"+
			"- **CWD**: %s\n",
		now.Format("2006-01-02 15:04:05"),
		uptime,
		runtime.GOOS, runtime.GOARCH,
		hostname,
		os.Getenv("USER"),
		cwd,
	)

	return info, nil
}
