package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"syscall"
	"testing"
	"time"
)

type loopbackProbeScenario struct {
	name          string
	hosts         string
	addresses     []string
	listen        string
	dropIPv6      bool
	disableIPv6   bool
	noListener    bool
	responseState HealthStatus
	invalidJSON   bool
}

var loopbackProbeScenarios = []loopbackProbeScenario{
	{name: "both-families-ipv4-listener", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "127.0.0.1"},
	{name: "both-families-ipv6-listener", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "::1"},
	{name: "both-families-ipv6-drop", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "127.0.0.1", dropIPv6: true},
	{name: "both-families-ipv6-disabled", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "127.0.0.1", disableIPv6: true},
	{name: "ipv6-only-name-ipv4-listener", hosts: "::1 localhost\n", addresses: []string{"::1"}, listen: "127.0.0.1"},
	{name: "ipv6-only-name-ipv6-drop", hosts: "::1 localhost\n", addresses: []string{"::1"}, listen: "127.0.0.1", dropIPv6: true},
	{name: "ipv6-only-name-ipv6-disabled", hosts: "::1 localhost\n", addresses: []string{"::1"}, listen: "127.0.0.1", disableIPv6: true},
	{name: "ipv4-only-name-ipv6-listener", hosts: "127.0.0.1 localhost\n", addresses: []string{"127.0.0.1"}, listen: "::1"},
	{name: "alternate-ipv4-loopback", hosts: "127.0.0.2 localhost\n", addresses: []string{"127.0.0.2"}, listen: "127.0.0.2"},
	{name: "both-families-no-listener", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "127.0.0.1", noListener: true},
	{name: "application-reports-unhealthy", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "127.0.0.1", responseState: Unhealthy},
	{name: "invalid-health-json", hosts: "::1 localhost\n127.0.0.1 localhost\n", addresses: []string{"::1", "127.0.0.1"}, listen: "127.0.0.1", invalidJSON: true},
}

func TestLoopbackProbeLinuxScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("network-namespace integration test")
	}
	for _, tool := range []string{"unshare", "ip", "tc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("network-namespace scenarios require %s: %v", tool, err)
		}
	}
	if output, err := exec.Command("unshare", "-Urnm", "true").CombinedOutput(); err != nil {
		t.Skipf("unprivileged namespaces unavailable: %v: %s", err, output)
	}
	netNS, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	mountNS, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range loopbackProbeScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			command := exec.Command("unshare", "-Urnm", "--", os.Args[0],
				"-test.run=^TestLoopbackProbeLinuxChild$", "-test.v", "-test.timeout=115s")
			command.Env = append(os.Environ(),
				"AHE_LOOPBACK_SCENARIO="+scenario.name,
				"AHE_LOOPBACK_PARENT_NETNS="+netNS,
				"AHE_LOOPBACK_PARENT_MOUNTNS="+mountNS)
			output, err := command.CombinedOutput()
			t.Logf("%s", output)
			if err != nil {
				t.Errorf("isolated scenario failed: %v", err)
			}
		})
	}
}

func TestLoopbackProbeLinuxChild(t *testing.T) {
	name := os.Getenv("AHE_LOOPBACK_SCENARIO")
	if name == "" {
		t.Skip("only invoked inside the parent's isolated namespaces")
	}
	for _, namespace := range []struct{ path, env string }{
		{"/proc/self/ns/net", "AHE_LOOPBACK_PARENT_NETNS"},
		{"/proc/self/ns/mnt", "AHE_LOOPBACK_PARENT_MOUNTNS"},
	} {
		current, err := os.Readlink(namespace.path)
		parent := os.Getenv(namespace.env)
		if err != nil || parent == "" || current == parent {
			t.Fatalf("refusing to alter non-isolated namespace %s: %v", namespace.path, err)
		}
	}
	var scenario *loopbackProbeScenario
	for i := range loopbackProbeScenarios {
		if loopbackProbeScenarios[i].name == name {
			scenario = &loopbackProbeScenarios[i]
			break
		}
	}
	if scenario == nil {
		t.Fatalf("unknown loopback scenario %q", name)
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
	defer func() {
		if err := syscall.Unmount("/etc/hosts", 0); err != nil {
			t.Error(err)
		}
	}()
	runNamespaceCommand(t, "ip", "link", "set", "lo", "up")
	if scenario.disableIPv6 {
		for _, iface := range []string{"all", "lo"} {
			if err := os.WriteFile("/proc/sys/net/ipv6/conf/"+iface+"/disable_ipv6", []byte("1\n"), 0600); err != nil {
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
	got := append([]string(nil), addresses...)
	want := append([]string(nil), scenario.addresses...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolution does not match the scenario: got %v, want %v", got, want)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, protocol := range []string{"tcp", "http", "https"} {
		t.Run(protocol, func(t *testing.T) {
			network := "tcp6"
			if net.ParseIP(scenario.listen).To4() != nil {
				network = "tcp4"
			}
			listener, err := net.Listen(network, net.JoinHostPort(scenario.listen, "0"))
			if err != nil {
				t.Fatal(err)
			}
			port := listener.Addr().(*net.TCPAddr).Port
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" || r.Method != http.MethodGet {
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if scenario.invalidJSON {
					fmt.Fprint(w, "not-json")
					return
				}
				state := scenario.responseState
				if state == "" {
					state = Healthy
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"applicationHealthState":%q,"customMetrics":"{\"test\":1}"}`, state)
			}))
			server.Listener.Close()
			server.Listener = listener
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			if protocol == "https" {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			if scenario.noListener {
				server.Close()
			}
			wantState := Healthy
			wantError := false
			if scenario.noListener {
				wantState, wantError = Unknown, true
				if protocol == "tcp" {
					wantState = Unhealthy
				}
			} else if protocol != "tcp" {
				if scenario.invalidJSON {
					wantState, wantError = Unknown, true
				} else if scenario.responseState != "" {
					wantState = scenario.responseState
				}
			}

			// Compare literal-IP and localhost probes against the same endpoint.
			for _, target := range []string{scenario.listen, "localhost"} {
				var probe HealthProbe
				if protocol == "tcp" {
					probe = &TcpHealthProbe{Address: net.JoinHostPort(target, strconv.Itoa(port))}
				} else {
					httpProbe := NewHttpHealthProbe(protocol, "/health", port)
					httpProbe.Address = protocol + "://" + net.JoinHostPort(target, strconv.Itoa(port)) + "/health"
					defer httpProbe.HttpClient.CloseIdleConnections()
					probe = httpProbe
				}
				start := time.Now()
				response, err := probe.evaluate(logger)
				elapsed := time.Since(start)
				t.Logf("RESULT scenario=%s protocol=%s target=%s resolved=%v state=%s elapsed=%s error=%v",
					name, protocol, target, addresses, response.ApplicationHealthState, elapsed, err)
				if response.ApplicationHealthState != wantState || (err != nil) != wantError {
					t.Errorf("expected state=%s error=%t, got state=%s error=%v",
						wantState, wantError, response.ApplicationHealthState, err)
				}
				if !scenario.noListener && !scenario.invalidJSON && protocol != "tcp" &&
					response.ApplicationHealthState == wantState && response.CustomMetrics != `{"test":1}` {
					t.Error("custom metrics were not preserved")
				}
				if !wantError && err == nil && elapsed >= 2*time.Second {
					t.Errorf("reachable endpoint took %s, expected under 2s", elapsed)
				}
			}
		})
	}
}

func runNamespaceCommand(t *testing.T, name string, args ...string) {
	t.Helper()
	if output, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v: %s", name, args, err, output)
	}
}
