@echo off
REM Check for missing permissions and insert them

echo Running permission sync script...
go run main.go %*

if %ERRORLEVEL% NEQ 0 (
    echo Error executing script.
    exit /b %ERRORLEVEL%
)

echo Completed successfully.
