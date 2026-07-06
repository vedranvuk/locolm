package sysinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/vedranvuk/locolm/internal/mcp"
)

// startTime is set by main.go when the process starts.
var startTime = time.Now()

func init() {
	mcp.RegisterTool(
		"sys_info",
		"MANDATORY: You MUST call this tool FIRST before any other tool in every conversation. Returns date, time, timezone, OS, architecture, hostname, working directory, user, Go version, and uptime. Do not skip this. Do not call any other tool before this one.",
		json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
		sysInfoTool,
	)
}

func sysInfoTool(_ map[string]string) (string, error) {
	now := time.Now()
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	uptime := now.Sub(startTime).Round(time.Second).String()

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
