package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
)

type loopbackScenario struct {
	name        string
	hosts       string
	listen      string
	dropIPv6    bool
	disableIPv6 bool
	tls13       bool
}

var loopbackScenarios = []loopbackScenario{
	{"ipv4-listener", "::1 localhost\n127.0.0.1 localhost\n", "127.0.0.1", false, false, false},
	{"ipv6-listener", "::1 localhost\n127.0.0.1 localhost\n", "::1", false, false, false},
	{"ipv6-drop-both-addresses", "::1 localhost\n127.0.0.1 localhost\n", "127.0.0.1", true, false, false},
	{"ipv6-drop-only-ipv6-name", "::1 localhost\n", "127.0.0.1", true, false, false},
	{"ipv6-refused-only-ipv6-name", "::1 localhost\n", "127.0.0.1", false, false, false},
	{"ipv4-refused-only-ipv4-name", "127.0.0.1 localhost\n", "::1", false, false, false},
	{"ipv6-disabled-both-addresses", "::1 localhost\n127.0.0.1 localhost\n", "127.0.0.1", false, true, false},
	{"ipv6-disabled-only-ipv6-name", "::1 localhost\n", "127.0.0.1", false, true, false},
	{"tls13-endpoint", "::1 localhost\n127.0.0.1 localhost\n", "127.0.0.1", false, false, true},
	{"alternate-ipv4-loopback", "127.0.0.2 localhost\n", "127.0.0.2", false, false, false},
	{"multiple-ipv4-loopbacks", "127.0.0.0 localhost\n127.0.0.2 localhost\n", "127.0.0.2", false, false, false},
	{"dual-stack-listener", "::1 localhost\n127.0.0.1 localhost\n", "::", false, false, false},
}

// The child changes hosts, sysctls and packet filtering only after proving that
// both its network and mount namespaces differ from the parent test process.
func Test_loopbackProbeLinuxScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("network-namespace integration scenarios are excluded by -short")
	}
	for _, tool := range []string{"unshare", "ip", "tc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("network-namespace scenarios require %s: %v", tool, err)
		}
	}
	if output, err := exec.Command("unshare", "-Urnm", "true").CombinedOutput(); err != nil {
		t.Skipf("unprivileged network/mount namespaces unavailable: %v: %s", err, output)
	}
	netNS, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	mountNS, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range loopbackScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			command := exec.Command("unshare", "-Urnm", "--", os.Args[0],
				"-test.run=^Test_loopbackProbeLinuxChild$", "-test.v", "-test.timeout=45s")
			command.Env = append(os.Environ(),
				"AHE_TEST_SCENARIO="+scenario.name,
				"AHE_TEST_PARENT_NETNS="+netNS,
				"AHE_TEST_PARENT_MOUNTNS="+mountNS)
			output, err := command.CombinedOutput()
			t.Logf("%s", output)
			if err != nil {
				t.Fatalf("isolated scenario failed: %v", err)
			}
		})
	}
}

func Test_loopbackProbeLinuxChild(t *testing.T) {
	name := os.Getenv("AHE_TEST_SCENARIO")
	if name == "" {
		t.Skip("invoked only by the network-namespace parent test")
	}
	for _, namespace := range []struct {
		path string
		env  string
	}{
		{"/proc/self/ns/net", "AHE_TEST_PARENT_NETNS"},
		{"/proc/self/ns/mnt", "AHE_TEST_PARENT_MOUNTNS"},
	} {
		current, err := os.Readlink(namespace.path)
		if err != nil {
			t.Fatal(err)
		}
		parent := os.Getenv(namespace.env)
		if parent == "" || current == parent {
			t.Fatalf("refusing to modify non-isolated namespace %s", namespace.path)
		}
	}
	var scenario *loopbackScenario
	for i := range loopbackScenarios {
		if loopbackScenarios[i].name == name {
			scenario = &loopbackScenarios[i]
		}
	}
	if scenario == nil {
		t.Fatalf("unknown scenario %q", name)
	}
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		t.Fatal(err)
	}
	hostsFile := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(hostsFile, []byte(scenario.hosts), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(hostsFile, "/etc/hosts", "", syscall.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Unmount("/etc/hosts", 0); err != nil {
			t.Error(err)
		}
	})
	runNamespaceCommand(t, "ip", "link", "set", "lo", "up")
	if scenario.disableIPv6 {
		for _, iface := range []string{"all", "lo"} {
			path := "/proc/sys/net/ipv6/conf/" + iface + "/disable_ipv6"
			if err := os.WriteFile(path, []byte("1\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if scenario.dropIPv6 {
		runNamespaceCommand(t, "tc", "qdisc", "add", "dev", "lo", "clsact")
		runNamespaceCommand(t, "tc", "filter", "add", "dev", "lo", "ingress",
			"protocol", "ipv6", "pref", "1", "matchall", "action", "drop")
	}
	addresses, err := net.LookupHost("localhost")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("localhost=%v; server=%s; IPv6-drop=%t; IPv6-disabled=%t",
		addresses, scenario.listen, scenario.dropIPv6, scenario.disableIPv6)
	for _, protocol := range []string{"tcp", "http", "https"} {
		t.Run(protocol, func(t *testing.T) {
			listener, err := net.Listen("tcp", net.JoinHostPort(scenario.listen, "0"))
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != fmt.Sprintf("localhost:%d", listener.Addr().(*net.TCPAddr).Port) ||
					r.URL.Path != "/health" {
					http.Error(w, "unexpected request target", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			server.Listener.Close()
			server.Listener = listener
			if protocol == "https" {
				if scenario.tls13 {
					server.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
				}
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			port := listener.Addr().(*net.TCPAddr).Port
			var probe HealthProbe
			if protocol == "tcp" {
				probe = &TcpHealthProbe{Address: net.JoinHostPort("localhost", fmt.Sprint(port))}
			} else {
				httpProbe := NewHttpHealthProbe(protocol, "/health", port)
				httpProbe.HttpClient.Timeout = 2 * time.Second
				defer httpProbe.HttpClient.CloseIdleConnections()
				probe = httpProbe
			}
			start := time.Now()
			state, err := probe.evaluate(log.NewContext(log.NewLogfmtLogger(os.Stdout)))
			elapsed := time.Since(start)
			t.Logf("state=%s error=%v elapsed=%s", state, err, elapsed)
			if state != Healthy || err != nil {
				t.Errorf("reachable loopback endpoint should be healthy: %s, %v", state, err)
			}
			if elapsed >= 2*time.Second {
				t.Errorf("healthy loopback fallback took %s; expected under 2s", elapsed)
			}
		})
	}
}

func runNamespaceCommand(t *testing.T, name string, args ...string) {
	t.Helper()
	if output, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, output)
	}
}
