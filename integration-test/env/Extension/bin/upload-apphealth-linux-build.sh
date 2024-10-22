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
latestbuild=$(az pipelines runs list  --org $org --project "$projectGuid"  --pipeline-ids $pipelineId | jq '[ .[] | select(.result == "succeeded")]' | jq '[.[].id] | sort | last')
# download the final output artifacts
rm -rf /tmp/linux_artifact
az pipelines runs artifact download --org $org --project "$projectGuid" --run-id $latestbuild --artifact-name drop_2_windows --path /tmp/linux_artifact
# get the zip file from the build artifact 
unzip /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/applicationhealth-extension*.zip -d /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped

# Unzip the local zip file to a temporary directory
rm -rf /tmp/local_zip
mkdir -p /tmp/local_zip
unzip /workspaces/applicationhealth-extension-linux/bundle/applicationhealth-extension.zip -d /tmp/local_zip

# Copy the specific files from the local zip to the target directory
cp /tmp/local_zip/bin/applicationhealth-extension /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/
cp /tmp/local_zip/bin/applicationhealth-extension-arm64 /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/
cp /tmp/local_zip/bin/applicationhealth-shim /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/

# Set the permissions for the copied files
chmod 775 /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/applicationhealth-extension
chmod 775 /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/applicationhealth-extension-arm64
chmod 775 /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/applicationhealth-shim
chmod -R 775 /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped/bin/VMWatch

# # zip it up

cd /tmp/linux_artifact/caps/ApplicationHealthLinuxTest/v2/ServiceGroupRoot/unzipped && zip -r /tmp/linux.zip . && cd -

# # upload it to the storage account
echo "Uploading linux.zip to container: $container"
az storage blob upload --account-name apphealthtestpackages --subscription $subscription --container-name $container --name linux.zip --file /tmp/linux.zip --overwrite --auth-mode login
