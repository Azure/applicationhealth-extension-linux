FROM mcr.microsoft.com/devcontainers/go:1-1.21-bullseye

RUN apt-get -qqy update && \
	apt-get -qqy install jq openssl ca-certificates && \
        apt-get -qqy clean && \
        rm -rf /var/lib/apt/lists/*

# Create the directories and files that need to be present
RUN mkdir -p /var/lib/waagent && \
        mkdir -p /var/lib/waagent/Extension/config && \
        mkdir -p /var/lib/waagent/Extension/status && \
        mkdir -p /var/log/azure/Extension/VE.RS.ION

# copy default extension settings into the appropriate location
COPY extension-settings.json /var/lib/waagent/Extension/config/0.settings
# install go tools we need for build
RUN go install github.com/ahmetb/govvv@latest

