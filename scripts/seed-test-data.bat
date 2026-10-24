@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0.."

set "BASE_URL=http://localhost"
set "LOGIN=%ADMIN_LOGIN%"
if not defined LOGIN set "LOGIN=admin"
set "PASSWORD=%ADMIN_PASSWORD%"
if not defined PASSWORD set "PASSWORD=admin"

for /f "usebackq delims=" %%D in (`powershell -NoProfile -Command "(Get-Date).AddDays(2).ToString('yyyy-MM-dd')"`) do set "DUE=%%D"
if not defined DUE (
  echo ERROR: cannot calculate due date
  exit /b 1
)

echo == login ==
for /f "usebackq delims=" %%T in (`powershell -NoProfile -Command "$payload = @{login=$env:LOGIN; password=$env:PASSWORD} ^| ConvertTo-Json -Compress; $r = curl.exe -sS -X POST '%BASE_URL%/api/v1/auth/login' -H 'Content-Type: application/json' -d $payload ^| ConvertFrom-Json; $r.access_token"`) do set "TOKEN=%%T"
if not defined TOKEN (
  echo LOGIN FAILED
  exit /b 1
)
echo login: OK

set "EXISTING_FILE=%TEMP%\invoice-keeper-existing.json"
curl.exe -sS -f "%BASE_URL%/api/v1/recurring-payments" -H "Authorization: Bearer %TOKEN%" -o "%EXISTING_FILE%"
if errorlevel 1 exit /b 1

for /f "usebackq delims=" %%I in (`powershell -NoProfile -Command "$d=Get-Content -Raw -LiteralPath $env:EXISTING_FILE ^| ConvertFrom-Json; @($d) ^| Where-Object {$_.title -like '[DEMO]*'} ^| ForEach-Object {$_.id}"`) do (
  echo deleting old DEMO id=%%I
  curl.exe -sS -f -X DELETE "%BASE_URL%/api/v1/recurring-payments/%%I" -H "Authorization: Bearer %TOKEN%" -o NUL
)

echo due date: %DUE%
call :create "[DEMO] Rental office" 50000 RUB
call :create "[DEMO] VPS server" 20 USD
call :create "[DEMO] Figma subscription" 15 EUR

set "FINAL_FILE=%TEMP%\invoice-keeper-final.json"
curl.exe -sS -f "%BASE_URL%/api/v1/recurring-payments?is_active=true" -H "Authorization: Bearer %TOKEN%" -o "%FINAL_FILE%"
for /f "usebackq delims=" %%C in (`powershell -NoProfile -Command "$d=Get-Content -Raw -LiteralPath $env:FINAL_FILE ^| ConvertFrom-Json; @($d) ^| Where-Object {$_.title -like '[DEMO]*'} ^| Measure-Object ^| Select-Object -ExpandProperty Count"`) do set "COUNT=%%C"
echo active DEMO count: %COUNT%
if not "%COUNT%"=="3" (
  echo ERROR: expected 3 active DEMO payments
  exit /b 1
)

echo == summary ==
curl.exe -sS -f "%BASE_URL%/api/v1/payments/summary" -H "Authorization: Bearer %TOKEN%"
echo.
echo seed: OK
exit /b 0

:create
set "TITLE=%~1"
set "AMOUNT=%~2"
set "CURRENCY=%~3"
for /f "usebackq delims=" %%R in (`powershell -NoProfile -Command "$payload = @{title=$env:TITLE; amount=[decimal]$env:AMOUNT; currency=$env:CURRENCY; next_payment_date=$env:DUE; periodicity='monthly'} ^| ConvertTo-Json -Compress; curl.exe -sS -f -X POST '%BASE_URL%/api/v1/recurring-payments' -H 'Authorization: Bearer %TOKEN%' -H 'Content-Type: application/json' -d $payload"`) do set "RESPONSE=%%R"
if errorlevel 1 exit /b 1
echo %CURRENCY%: OK
exit /b 0
