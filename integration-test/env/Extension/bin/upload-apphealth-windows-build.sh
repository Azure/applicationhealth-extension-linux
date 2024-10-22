#!/bin/bash

# this is a helper script to upload the latest binaries for app health extension and vmwatch to a zip file
# temp solution until we get something better in place, run this script on a dev machine

set -uex

org=$1
projectGuid=$2
pipelineId=$3
subscription=$4
container="${5:-packages}"

# get the latest build of the linux pipeline from devops
echo "Getting the latest build from DevOps..."
latestbuild=$(az pipelines runs list  --org $org --project "$projectGuid"  --pipeline-ids $pipelineId | jq '[ .[] | select(.result == "succeeded")]' | jq '[.[].id] | sort | last')
echo "Latest build ID: $latestbuild"

# download the final output artifacts
echo "Downloading the final output artifacts..."
rm -rf /tmp/windows_artifact
az pipelines runs artifact download --org $org --project "$projectGuid" --run-id $latestbuild --artifact-name drop_build_retail_amd64 --path /tmp/windows_artifact

echo "Unzipping the build artifact..."
# get the zip file from the build artifact 
unzip /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/ApplicationHealthWindows_*.zip -d /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/unzipped || true
echo "Unzipped the windows artifact"

# Unzip the local zip file to a temporary directory
rm -rf /tmp/local_zip
mkdir -p /tmp/local_zip

echo "Unzipping the local zip file..."
unzip /workspaces/applicationhealth-extension-linux/bundle/applicationhealth-extension.zip -d /tmp/local_zip
 
echo "Unzipped the local zip"

# # Copy the specific files from the local zip to the target directory
echo "Copying specific files from the local zip to the target directory..."
cp /tmp/local_zip/bin/AppHealthExtension.exe /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/unzipped/bin/
cp /tmp/local_zip/bin/AppHealthExtension-arm64.exe /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/unzipped/bin/

# Verify the contents of the target directory
echo "Contents of /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/unzipped/bin:"
ls -l /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/unzipped/bin/

# # # zip it up
echo "Zipping up the modified files..."
cd /tmp/windows_artifact/retail-amd64/exports/Ev2/caps/ApplicationHealthWindowsTest/ServiceGroupRoot/unzipped && zip -r /tmp/windows.zip . && cd -

# # # upload it to the storage account
echo "Uploading linux.zip to container: $container"
az storage blob upload --account-name apphealthtestpackages --subscription $subscription --container-name $container --name windows.zip --file /tmp/windows.zip --overwrite --auth-mode login
