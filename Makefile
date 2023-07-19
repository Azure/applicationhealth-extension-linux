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
	GOOS=linux GOARCH=amd64 govvv build -v \
	  -ldflags "-X main.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(BIN) ./main
	cp ./misc/applicationhealth-shim ./$(BINDIR)
	GOOS=linux GOARCH=amd64 govvv build -v \
	  -ldflags "-X main.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(TESTBINDIR)/$(WEBSERVERBIN) ./integration-test/webserver
	GOOS=linux GOARCH=arm64 govvv build -v \
	  -ldflags "-X main.Version=`grep -E -m 1 -o '<Version>(.*)</Version>' misc/manifest.xml | awk -F">" '{print $$2}' | awk -F"<" '{print $$1}'`" \
	  -o $(BINDIR)/$(BIN_ARM64) ./main 
clean:
	rm -rf "$(BINDIR)" "$(BUNDLEDIR)" "$(TESTBINDIR)"

.PHONY: clean binary
