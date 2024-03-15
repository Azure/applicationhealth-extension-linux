REM BOOTSTRAP files that are needed

REM set up logs, config etc
IF NOT EXIST C:\temp\applicationhealth-extension MKDIR C:\temp\applicationhealth-extension
IF NOT EXIST C:\temp\applicationhealth-extension\config MKDIR C:\temp\applicationhealth-extension\config
IF NOT EXIST C:\temp\applicationhealth-extension\events MKDIR C:\temp\applicationhealth-extension\events
IF NOT EXIST C:\temp\applicationhealth-extension\logs MKDIR C:\temp\applicationhealth-extension\logs
IF NOT EXIST C:\temp\applicationhealth-extension\status MKDIR C:\temp\applicationhealth-extension\status

REM now create the fake config files
ECHO. > C:\temp\applicationhealth-extension\123.status
COPY /Y .\localdev.settings C:\temp\applicationhealth-extension\config\123.settings
ECHO. >  C:\temp\applicationhealth-extension\123.txt

REM now HandlerEnvironment.windows.json for  HandlerEnvironment.json which is in same directory as the script
REN HandlerEnvironment.windows.json HandlerEnvironment.json