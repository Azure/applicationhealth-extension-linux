BINDIR=bin
LINUX_BIN=applicationhealth-extension
LINUX_BIN_ARM64=applicationhealth-extension-arm64
WINDOWS_BIN=applicationhealth-extension-windows.exe
WINDOWS_BIN_ARM64=applicationhealth-extension-windows-arm64.exe
BUNDLEDIR=bundle
BUNDLE=applicationhealth-extension.zip
TESTBINDIR=testbin
WEBSERVERBIN=webserver
WINDOWS_TEST_BUNDLEDIR=bundle-windows-test
WINDOWS_TEST_BUNDLE=applicationhealth-extension-windows.zip

bundle: clean binary
	@mkdir -p $(BUNDLEDIR)
	zip ./$(BUNDLEDIR)/$(BUNDLE) ./$(BINDIR)/$(LINUX_BIN)
	zip ./$(BUNDLEDIR)/$(BUNDLE) ./$(BINDIR)/$(LINUX_BIN_ARM64)
	zip ./$(BUNDLEDIR)/$(BUNDLE) ./$(BINDIR)/applicationhealth-shim
	zip -j ./$(BUNDLEDIR)/$(BUNDLE) ./misc/HandlerManifest.json
	zip -j ./$(BUNDLEDIR)/$(BUNDLE) ./misc/manifest.xml

binary-linux: clean
	if [ -z "$$GOPATH" ]; then \
	  echo "GOPATH is not set"; \
	  exit 1; \
	fi
	# Set CGO_ENABLED=0 for static binaries, note that another approach might be needed if dependencies change
	# (see https://github.com/golang/go/issues/26492 for using an external linker if CGO is required)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X github.com/Azure/applicationhealth-extension-linux/internal/version.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/linux/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(LINUX_BIN) ./main
	cp ./misc/linux/applicationhealth-shim ./$(BINDIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X github.com/Azure/applicationhealth-extension-linux/internal/version.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/linux/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(TESTBINDIR)/$(WEBSERVERBIN) ./integration-test/webserver
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X github.com/Azure/applicationhealth-extension-linux/internal/version.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/linux/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(LINUX_BIN_ARM64) ./main 

binary-windows: clean
	if [ -z "$$GOPATH" ]; then \
	  echo "GOPATH is not set"; \
	  exit 1; \
	fi
	# Set CGO_ENABLED=0 for static binaries, note that another approach might be needed if dependencies change
	# (see https://github.com/golang/go/issues/26492 for using an external linker if CGO is required)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X github.com/Azure/applicationhealth-extension-linux/internal/version.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/windows/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(WINDOWS_BIN) ./main
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X github.com/Azure/applicationhealth-extension-linux/internal/version.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/windows/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(TESTBINDIR)/$(WEBSERVERBIN) ./integration-test/webserver
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -v -mod=readonly \
	  -ldflags "-X github.com/Azure/applicationhealth-extension-linux/internal/version.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/windows/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(WINDOWS_BIN_ARM64) ./main 

clean:
	rm -rf "$(BINDIR)" "$(BUNDLEDIR)" "$(TESTBINDIR)" "$(WINDOWS_TEST_BUNDLEDIR)"

# set up the files in the dev container for debugging locally with a default settings file
# ONLY run this if in a dev container as it can mess with local machine
testenv-linux:
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
	cp misc/linux/HandlerManifest.json /var/lib/waagent/Extension/
	cp misc/linux/manifest.xml /var/lib/waagent/Extension/
	cp misc/linux/applicationhealth-shim /var/lib/waagent/Extension/bin/
	cp bin/applicationhealth-extension /var/lib/waagent/Extension/bin
	mkdir -p /var/lib/waagent/Extension/status
	mkdir -p /var/log/azure/Extension/events
	mkdir -p /var/lib/waagent/Extension/config/
	cp ./.devcontainer/extension-settings.json /var/lib/waagent/Extension/config/0.settings

devcontainer: binary testenv-linux

testenv-windows: binary-windows 
	@mkdir -p $(WINDOWS_TEST_BUNDLEDIR)
	# Create the custom directory within the bundle directory
	mkdir $(WINDOWS_TEST_BUNDLEDIR)/localdev
	mkdir -p ./$(BINDIR)/VMWatch
	cp -r ./integration-test/env/Extension/bin/VMWatch/* ./$(BINDIR)/VMWatch
	rm ./$(BINDIR)/VMWatch/*linux*
	zip -r ./$(WINDOWS_TEST_BUNDLEDIR)/$(WINDOWS_TEST_BUNDLE) ./$(BINDIR)
	# Copy windows directory to the localdev directory
	cp -r ./integration-test/env/windows $(WINDOWS_TEST_BUNDLEDIR)/localdev
	zip -r -j ./$(WINDOWS_TEST_BUNDLEDIR)/$(WINDOWS_TEST_BUNDLE) $(WINDOWS_TEST_BUNDLEDIR)/localdev
	zip -j ./$(WINDOWS_TEST_BUNDLEDIR)/$(WINDOWS_TEST_BUNDLE) ./misc/windows/manifest.xml

.PHONY: clean binary
