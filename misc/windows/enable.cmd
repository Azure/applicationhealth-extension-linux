if not exist RuntimeSettings\*.settings exit /b -2
call disable.cmd
start cmd /C bin\AppHealthExtension.exe "enable"