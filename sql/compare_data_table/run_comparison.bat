@echo off
REM Compare Data Table Tool - Launcher Script
REM This batch file makes it easier to run the data comparison tool

echo PostgreSQL Master Data Comparison Tool
echo =====================================
echo.

REM Check if Go is installed
where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo Error: Go is not installed or not in your PATH.
    echo Please install Go from https://golang.org/dl/
    exit /b 1
)

REM Run the tool with all arguments passed to this script
cd /d "%~dp0"
go run cmd/main.go %*

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo There was an error running the comparison tool.
    exit /b %ERRORLEVEL%
)

echo.
echo Comparison completed successfully.
