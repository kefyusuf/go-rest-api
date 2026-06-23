@echo off
setlocal
C:\Users\yukonit\go\bin\swag init -g main.go -d .,.\internal\handler,.\internal\model -o .\docs
endlocal
