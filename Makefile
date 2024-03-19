BINDIR=bin
BIN=applicationhealth-extension
BIN_ARM64=applicationhealth-extension-arm64
BUNDLEDIR=bundle
BUNDLE=applicationhealth-extension.zip
TESTBINDIR=testbin
WEBSERVERBIN=webserver

bundle: clean binary
	@mkdir -p $(BUNDLEDIR)
	zip ./$(BUNDLEDIR)/$(BUNDLE) ./$(BINDIR)/$(BIN)
	zip ./$(BUNDLEDIR)/$(BUNDLE) ./$(BINDIR)/$(BIN_ARM64)
	zip ./$(BUNDLEDIR)/$(BUNDLE) ./$(BINDIR)/applicationhealth-shim
	zip -j ./$(BUNDLEDIR)/$(BUNDLE) ./misc/HandlerManifest.json
	zip -j ./$(BUNDLEDIR)/$(BUNDLE) ./misc/manifest.xml

binary: clean
	if [ -z "$$GOPATH" ]; then \
	  echo "GOPATH is not set"; \
	  exit 1; \
	fi
	# Set CGO_ENABLED=0 for static binaries, note that another approach might be needed if dependencies change
	# (see https://github.com/golang/go/issues/26492 for using an external linker if CGO is required)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X main.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(BIN) ./main
	cp ./misc/applicationhealth-shim ./$(BINDIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X main.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(TESTBINDIR)/$(WEBSERVERBIN) ./integration-test/webserver
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X main.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(BIN_ARM64) ./main 
clean:
	rm -rf "$(BINDIR)" "$(BUNDLEDIR)" "$(TESTBINDIR)"

# set up the files in the dev container for debugging locally with a default settings file
# ONLY run this if in a dev container as it can mess with local machine
testenv:
ifneq ("$(RUNNING_IN_DEV_CONTAINER)", "1")
	echo "Target can only run in dev container $(RUNNING_IN_DEV_CONTAINER)"
	exit 1
endif
	cp -r ./integration-test/env/* /var/lib/waagent/
	cp -r ./testbin/* /var/lib/waagent/
	ln -sf /var/lib/waagent/fake-waagent /sbin/fake-waagent || ture
	ln -sf /var/lib/waagent/wait-for-enable /sbin/wait-for-enable
	ln -sf /var/lib/waagent/webserver /sbin/webserver
	ln -sf /var/lib/waagent/webserver_shim /sbin/webserver_shim
	cp misc/HandlerManifest.json /var/lib/waagent/Extension/
	cp misc/manifest.xml /var/lib/waagent/Extension/
	cp misc/applicationhealth-shim /var/lib/waagent/Extension/bin/
	cp bin/applicationhealth-extension /var/lib/waagent/Extension/bin
	mkdir -p /var/log/azure/Extension/events
	mkdir -p /var/lib/waagent/Extension/config/
	cp ./.devcontainer/extension-settings.json /var/lib/waagent/Extension/config/0.settings

devcontainer: binary testenv

.PHONY: clean binary
