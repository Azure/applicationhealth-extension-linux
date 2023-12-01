#!/bin/bash
set -uex
if [ -z "$1" ]
  then
    echo "USAGE update-vmwatch.sh <version>"
    echo eg specific version      : update-vmwatch.cmd "1.0.2"
    echo eg latest version of 1.0 : update-vmwatch.cmd "1.0.*"
    echo eg latest version        : update-vmwatch.cmd "*"
    exit 1
fi

az artifacts universal download --organization "https://msazure.visualstudio.com/" --project "b32aa71e-8ed2-41b2-9d77-5bc261222004" --scope project --feed "VMWatch" --name "vmwatch" --version "$1" --path ./VMWatch 

# remove the windows and darwin binaries
rm ./VMWatch/*windows*
rm ./VMWatch/*darwin*

chmod +x ./VMWatch/vmwatch_linux*
