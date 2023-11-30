#!/bin/bash

# this is a helper script to upload the latest binaries for app health extension and vmwatch to a zip file
# temp solution until we get something better in place, run this script on a dev machine

set -uex

# get the latest build of the linux pipeline from devops
latestbuild=$(az pipelines runs list  --org https://msazure.visualstudio.com --project "b32aa71e-8ed2-41b2-9d77-5bc261222004"  --pipeline-ids 298312 | jq "[.[].id] | sort | last")
# download the final output artifacts
rm -rf /tmp/linux_artifact
az pipelines runs artifact download --org https://msazure.visualstudio.com --project "b32aa71e-8ed2-41b2-9d77-5bc261222004" --run-id $latestbuild --artifact-name drop_2_windows --path /tmp/linux_artifact
# get the zip file from the build artifact 
unzip /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/applicationhealth-extension*.zip -d /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped

# now copy the latest binaries for vmwatch and app health extension to the folder structure
rm /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/VMWatch/*
cp ./VMWatch/* /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/VMWatch/
cp ../../../../bin/* /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/

# zip it up

cd /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped && zip -r /tmp/vmwatch.zip . && cd -

# upload it to the storage account
az storage blob upload --account-name vmwatchtest --subscription 3f3e281a-dc49-4930-b5cf-7ac71cd31603 --container-name packages --name linux.zip --file /tmp/vmwatch.zip --overwrite
