FROM mcr.microsoft.com/devcontainers/go:1.18-bullseye

RUN apt-get -qqy update && \
	apt-get -qqy install jq openssl ca-certificates && \
        apt-get -qqy clean && \
        rm -rf /var/lib/apt/lists/*

# Create the directories and files that need to be present
RUN mkdir -p /var/lib/waagent && \
        mkdir -p /var/lib/waagent/Extension/config && \
        mkdir -p /var/lib/waagent/Extension/status && \
        mkdir -p /var/log/azure/Extension/VE.RS.ION \
        mkdir -p /var/log/azure/Extension/events

# copy default extension settings into the appropriate location
COPY extension-settings.json /var/lib/waagent/Extension/config/0.settings
# install go tools we need for build
RUN go install github.com/ahmetb/govvv@latest

# Install npm
RUN apt-get update && \
    apt-get install -y npm
    
# Updating npm to the latest version
RUN npm cache clean -f && npm install -g n && n stable

# Install dev enviroment dependencies
RUN npm install bats -g

#Install Bats-Assert and Bats-Support
RUN npm install -g https://github.com/bats-core/bats-assert && \
        npm install -g https://github.com/bats-core/bats-support

# Install Parallel 
RUN apt-get install -y parallel

# Install Docker
RUN apt-get install runc -y && \
        apt-get install containerd -y && \
        apt-get install docker.io -y

# Install Docker Engine
RUN curl -fsSL https://test.docker.com -o test-docker.sh && \
        sh test-docker.sh

# Creating ENV variables
ENV CUSTOM_BATS_LIB_PATH /usr/local/lib/node_modules