@echo off
setlocal
cd /d "%~dp0.."

echo == Windows without WSL/Git Bash ==
echo Stopping and removing containers and volumes...
docker compose down -v
if errorlevel 1 exit /b 1

echo Building and starting the clean demo environment...
docker compose up --build -d
if errorlevel 1 exit /b 1

timeout /t 20 /nobreak

call scripts\seed-test-data.bat
if errorlevel 1 exit /b 1

call scripts\check-stand.bat
exit /b %errorlevel%
