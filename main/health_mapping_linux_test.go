package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
)

func Test_loopbackProbePreservesIPv4Mapping(t *testing.T) {
	if os.Getenv("AHE_MAPPING_CHILD") == "" {
		if testing.Short() {
			t.Skip("network-namespace integration test")
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
		for _, scenario := range []string{"hosts", "slow-dns"} {
			t.Run(scenario, func(t *testing.T) {
				cmd := exec.Command("unshare", "-Urnm", "--", os.Args[0],
					"-test.run=^Test_loopbackProbePreservesIPv4Mapping$", "-test.v", "-test.timeout=10s")
				cmd.Env = append(os.Environ(), "AHE_MAPPING_CHILD=1",
					"AHE_MAPPING_SCENARIO="+scenario,
					"AHE_MAPPING_PARENT_NETNS="+netNS, "AHE_MAPPING_PARENT_MOUNTNS="+mountNS)
				output, err := cmd.CombinedOutput()
				t.Logf("%s", output)
				if err != nil {
					t.Fatal(err)
				}
			})
		}
		return
	}
	for _, namespace := range []struct{ path, env string }{
		{"/proc/self/ns/net", "AHE_MAPPING_PARENT_NETNS"},
		{"/proc/self/ns/mnt", "AHE_MAPPING_PARENT_MOUNTNS"},
	} {
		value, err := os.Readlink(namespace.path)
		if err != nil || os.Getenv(namespace.env) == "" || value == os.Getenv(namespace.env) {
			t.Fatalf("refusing non-isolated namespace: %s, %v", namespace.path, err)
		}
	}
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		t.Fatal(err)
	}
	runNamespaceCommand(t, "ip", "link", "set", "lo", "up")
	hosts := filepath.Join(t.TempDir(), "hosts")
	contents := "127.0.0.1 localhost\n"
	slowDNS := os.Getenv("AHE_MAPPING_SCENARIO") == "slow-dns"
	if slowDNS {
		contents = "# Resolve localhost through the isolated test DNS server.\n"
	}
	if err := os.WriteFile(hosts, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(hosts, "/etc/hosts", "", syscall.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	defer syscall.Unmount("/etc/hosts", 0)

	ipv4, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ipv4.Addr().(*net.TCPAddr).Port
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.Listener.Close()
	server.Listener = ipv4
	server.StartTLS()
	defer server.Close()

	// An unadvertised IPv6 listener must not displace a working IPv4 target.
	ipv6, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	exited := make(chan struct{})
	accepted := make(chan struct{})
	go func() {
		defer close(exited)
		conn, err := ipv6.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		<-stop
	}()
	defer func() {
		close(stop)
		ipv6.Close()
		<-exited
	}()
	var dnsQueries func() int32
	if slowDNS {
		dnsQueries = delayedIPv4Resolver(t, 500*time.Millisecond)
		control := NewHttpHealthProbe("https", "/health", port)
		control.HttpClient.Transport.(*http.Transport).DialContext = (&net.Dialer{}).DialContext
		control.HttpClient.Timeout = 2 * time.Second
		defer control.HttpClient.CloseIdleConnections()
		state, err := control.evaluate(log.NewContext(log.NewNopLogger()))
		if state != Healthy || err != nil {
			t.Fatalf("normal hostname dialing control failed: %s, %v", state, err)
		}
		if dnsQueries() != 1 {
			t.Fatalf("control used %d A queries, want 1", dnsQueries())
		}
	}
	probe := NewHttpHealthProbe("https", "/health", port)
	probe.HttpClient.Timeout = 2 * time.Second
	defer probe.HttpClient.CloseIdleConnections()
	state, err := probe.evaluate(log.NewContext(log.NewNopLogger()))
	if state != Healthy || err != nil {
		t.Fatalf("working configured IPv4 endpoint was displaced: %s, %v", state, err)
	}
	if slowDNS && dnsQueries() != 2 {
		t.Fatalf("control and candidate used %d A queries, want one each", dnsQueries())
	}
	select {
	case <-accepted:
		t.Fatal("unadvertised IPv6 endpoint was contacted despite successful IPv4")
	default:
	}
}
