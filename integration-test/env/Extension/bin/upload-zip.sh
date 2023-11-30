#!/bin/bash

# this is a helper script to upload the latest binaries for app health extension and vmwatch to a zip file
# temp solution until we get something better in place, run this script on a dev machine

set -uex

$org=$1
$projectGuid=$2
$pipelineId=$3
$subscription=$4

# get the latest build of the linux pipeline from devops
latestbuild=$(az pipelines runs list  --org $org --project "$projectGuid"  --pipeline-ids $pipelineId | jq "[.[].id] | sort | last")
# download the final output artifacts
rm -rf /tmp/linux_artifact
az pipelines runs artifact download --org $org --project "$projectGuid" --run-id $latestbuild --artifact-name drop_2_windows --path /tmp/linux_artifact
# get the zip file from the build artifact 
unzip /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/applicationhealth-extension*.zip -d /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped

# now copy the latest binaries for vmwatch and app health extension to the folder structure
rm /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/VMWatch/*
cp ./VMWatch/* /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/VMWatch/
cp ../../../../bin/* /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/

# zip it up

cd /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped && zip -r /tmp/vmwatch.zip . && cd -

# upload it to the storage account
az storage blob upload --account-name vmwatchtest --subscription $subscription --container-name packages --name linux.zip --file /tmp/vmwatch.zip --overwrite
