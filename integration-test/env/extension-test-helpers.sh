#!/bin/bash
set -uex
logFilePath="/var/log/azure/Extension/force-kill-extension.txt"

force_kill_apphealth() {
    app_health_pid=$(ps -ef | grep "applicationhealth-extension" | grep -v grep | grep -v tee | awk '{print $2}')

    if [ -z "$app_health_pid" ]; then
        echo "Applicationhealth extension is not running" > $logFilePath
        return 0
    fi
    echo "Killing the applicationhealth extension forcefully" >> $logFilePath
    kill -9 $app_health_pid

    sleep 10

    output=$(check_running_processes)
    if [ "$output" == "Applicationhealth and VMWatch are not running" ]; then
        echo "$output" >> /var/log/azure/Extension/force-kill-extension.txt
        echo "Successfully killed the apphealth extension" >> $logFilePath
        echo "Successfully killed the VMWatch extension" >> $logFilePath
    else
        echo "$output" >> /var/log/azure/Extension/force-kill-extension.txt
        echo "Failed to kill the apphealth extension" >> $logFilePath
    fi
    
}

check_running_processes() {
    local output=$(ps -ef | grep -e "applicationhealth-extension" -e "vmwatch_linux_amd64" | grep -v grep | grep -v tee)
    if [ -z "$output" ]; then
        echo "Applicationhealth and VMWatch are not running"
    else
        if [ -n "$(echo $output | grep "applicationhealth-extension")" ]; then
            echo "Applicationhealth is running"
        fi
        if [ -n "$(echo $output | grep "vmwatch_linux_amd64")" ]; then
            echo "VMWatch is running"
        fi
        echo "$output"
    fi
}
