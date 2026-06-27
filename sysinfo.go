package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// SysInfo is the tool function registered in the tool registry.
func SysInfo(args map[string]string) (string, error) {
	return sysInfo()
}

// sysInfo returns a formatted snapshot of system and environment information.
func sysInfo() (string, error) {
	now := time.Now()

	date := now.Format("2006-01-02")
	tm := now.Format("15:04:05")
	tz, offset := now.Zone()
	utcOff := fmt.Sprintf("%+03d:%02d", offset/3600, (offset%3600)/60)

	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	user := os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH
	goVer := runtime.Version()

	uptime := "unknown"
	if startTime.IsZero() {
		uptime = "unknown"
	} else {
		elapsed := now.Sub(startTime)
		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		if hours > 0 {
			uptime = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			uptime = fmt.Sprintf("%dm", minutes)
		}
	}

	return fmt.Sprintf(
		"System Information\n"+
			"─────────────────────────────\n"+
			"Date:       %s\n"+
			"Time:       %s\n"+
			"Timezone:    %s (UTC%s)\n"+
			"OS:         %s\n"+
			"Arch:       %s\n"+
			"Hostname:   %s\n"+
			"CWD:        %s\n"+
			"User:       %s\n"+
			"Go Version:  %s\n"+
			"Uptime:     %s\n"+
			"─────────────────────────────",
		date, tm, tz, utcOff, osName, arch, hostname, cwd, user, goVer, uptime,
	), nil
}

// startTime is set by main.go when the process starts.
var startTime = time.Now()
