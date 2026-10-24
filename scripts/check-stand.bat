@echo off
setlocal EnableExtensions
cd /d "%~dp0.."

echo == compose ps ==
docker compose ps

echo == frontend ==
curl.exe -sS -f -o NUL -w "frontend: %%{http_code}\n" http://localhost/
echo == swagger ==
curl.exe -sS -f -o NUL -w "swagger:  %%{http_code}\n" http://localhost/api/v1/docs
echo == openapi ==
curl.exe -sS -f -o NUL -w "openapi:  %%{http_code}\n" http://localhost/api/v1/openapi.json

echo == login ==
set "LOGIN=%ADMIN_LOGIN%"
if not defined LOGIN set "LOGIN=admin"
set "PASSWORD=%ADMIN_PASSWORD%"
if not defined PASSWORD set "PASSWORD=admin"
set "LOGIN_CODE="
for /f "usebackq delims=" %%R in (`curl.exe -sS -X POST http://localhost/api/v1/auth/login -H "Content-Type: application/json" -d "{\"login\":\"%LOGIN%\",\"password\":\"%PASSWORD%\"}" -o NUL -w "%%{http_code}"`) do set "LOGIN_CODE=%%R"
echo login:    %LOGIN_CODE%

echo == bot status ==
docker compose ps bot
echo == bot logs (last 10) ==
docker compose logs --tail=10 bot
echo == telegram chat validation ==
set "CHAT_ID="
for /f "usebackq delims=" %%C in (`docker compose exec -T bot sh -c "printf %%s \"$TELEGRAM_CHAT_ID\""`) do set "CHAT_ID=%%C"
if "%CHAT_ID%"=="" echo WARNING: TELEGRAM_CHAT_ID is a placeholder or empty - reminders will fail
if "%CHAT_ID%"=="0" echo WARNING: TELEGRAM_CHAT_ID is a placeholder or empty - reminders will fail
if "%CHAT_ID%"=="-1001234567890" echo WARNING: TELEGRAM_CHAT_ID is a placeholder or empty - reminders will fail
if not "%CHAT_ID%"=="" if not "%CHAT_ID%"=="0" if not "%CHAT_ID%"=="-1001234567890" echo chat_id set: yes
exit /b 0
