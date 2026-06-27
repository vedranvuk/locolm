# Build Workflow Rules

## CRITICAL: Kill running instances BEFORE rebuilding
- Windows does NOT allow overwriting a running executable
- ALWAYS stop ALL running processes before rebuilding:
  1. `Stop-Process -Name "locolm" -Force -ErrorAction SilentlyContinue`
  2. `Stop-Process -Name "llama-server" -Force -ErrorAction SilentlyContinue`
  3. `Stop-Process -Name "llama" -Force -ErrorAction SilentlyContinue`
  4. Then rebuild: `go build -o bin/locolm.exe ./cmd/locolm/`
  5. Then run: `cd bin; .\locolm.exe`
- llama-server holds 12GB+ of model in memory — if you don't kill it, rebuild fails
- This is Windows, not Linux. You CANNOT overwrite a running binary.
- WITHOUT EXCEPTION, always kill first, then build, then run.
