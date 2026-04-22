@echo off
setLocal enableDelayedExpansion

set WIRE_CMD=wire gen ./cmd

where wire >nul 2>nul
if %errorLevel% neq 0 (
    echo Wire is not installed, installing now...
    go install github.com/google/wire/cmd/wire@latest
)
echo `%WIRE_CMD%` at: %date% %time% && %WIRE_CMD%

endLocal
