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

.PHONY: clean binary
