FROM ubuntu:20.04

RUN apt-get -qqy update && \
	apt-get -qqy install jq openssl ca-certificates && \
        apt-get -y install sysstat bc netcat && \
        apt-get -qqy clean && \
        rm -rf /var/lib/apt/lists/*

# Create the directories and files that need to be present
RUN mkdir -p /var/lib/waagent && \
        mkdir -p /var/lib/waagent/Extension/config && \
        touch /var/lib/waagent/Extension/config/0.settings && \
        mkdir -p /var/lib/waagent/Extension/status && \
        mkdir -p /var/log/azure/Extension/VE.RS.ION && \
        mkdir -p /var/log/azure/Extension/events

# Copy the test environment
WORKDIR /var/lib/waagent
COPY integration-test/env/ .
COPY testbin/ .
RUN ln -s /var/lib/waagent/fake-waagent /sbin/fake-waagent && \
        ln -s /var/lib/waagent/wait-for-enable /sbin/wait-for-enable && \
        ln -s /var/lib/waagent/webserver /sbin/webserver && \
        ln -s /var/lib/waagent/webserver_shim /sbin/webserver_shim

# Copy the handler files
COPY misc/HandlerManifest.json ./Extension/
COPY misc/manifest.xml ./Extension/
COPY misc/applicationhealth-shim ./Extension/bin/
COPY bin/applicationhealth-extension ./Extension/bin/

# Copy Helper functions and scripts
COPY integration-test/test/test_helper.bash /var/lib/waagent
