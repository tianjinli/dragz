@echo off
setLocal enableDelayedExpansion

echo ============================================
echo Please copy this script and modify as needed
echo ============================================

set ENV_FILE=.env.dev
set BUILD_TAG=-tags file
set BIN_PATH=./bin/main.exe
set PACKAGE=./cmd/dragz

for /F "usebackq tokens=1* delims==" %%i in ("%ENV_FILE%") do (
    set "line=%%i=%%j"
    if not "%%i" == "" if not "!line:~0,1!" == "#" (
        set "%%i=%%j"
        echo Set %%i=%%j
    )
)

where air >nul 2>nul
if %errorLevel% neq 0 (
    echo Air is not installed, installing now...
    go install github.com/air-verse/air@latest
)
air -tmp_dir ./bin -build.bin %BIN_PATH% -build.cmd "go build %BUILD_TAG% -o %BIN_PATH% %PACKAGE%"

endLocal
