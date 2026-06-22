package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scriptPath resolves luks-dhcp.sh relative to this test file so the test does
// not depend on the working directory.
func scriptPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "luks-dhcp.sh")
}

// runHook executes luks-dhcp.sh with the given dhclient env vars, stubbing out
// the ip/ipcalc commands so the test never touches the host network, and
// returns the contents of the resolv.conf it wrote (empty string if none).
func runHook(t *testing.T, env map[string]string) string {
	t.Helper()

	binDir := t.TempDir()
	writeExec(t, filepath.Join(binDir, "ip"), "#!/bin/sh\nexit 0\n")
	writeExec(t, filepath.Join(binDir, "ipcalc"), "#!/bin/sh\necho PREFIX=24\n")

	resolv := filepath.Join(t.TempDir(), "resolv.conf")

	cmd := exec.Command("/bin/sh", scriptPath(t))
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"RESOLV_CONF="+resolv,
	)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}

	data, err := os.ReadFile(resolv)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("reading resolv.conf: %v", err)
	}
	return string(data)
}

func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestWritesResolvConfFromLease(t *testing.T) {
	got := runHook(t, map[string]string{
		"new_ip_address":          "10.37.30.79",
		"new_subnet_mask":         "255.255.255.0",
		"new_routers":             "10.37.30.254",
		"interface":               "ens3f0",
		"new_domain_name_servers": "10.32.0.13 10.32.0.14 10.32.0.15",
		"new_domain_name":         "vinted.net",
	})

	for _, want := range []string{
		"nameserver 10.32.0.13",
		"nameserver 10.32.0.14",
		"nameserver 10.32.0.15",
		"search vinted.net",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("resolv.conf missing %q, got:\n%s", want, got)
		}
	}
}

func TestSearchPrefersDomainSearch(t *testing.T) {
	got := runHook(t, map[string]string{
		"new_ip_address":          "10.37.30.79",
		"new_subnet_mask":         "255.255.255.0",
		"new_routers":             "10.37.30.254",
		"interface":               "ens3f0",
		"new_domain_name_servers": "10.32.0.13",
		"new_domain_name":         "vinted.net",
		"new_domain_search":       "ams1.vinted.infra vinted.net",
	})

	if !strings.Contains(got, "search ams1.vinted.infra vinted.net") {
		t.Fatalf("expected domain-search to win, got:\n%s", got)
	}
}

func TestLeavesResolvConfWhenLeaseHasNoDNS(t *testing.T) {
	got := runHook(t, map[string]string{
		"new_ip_address":  "10.37.30.79",
		"new_subnet_mask": "255.255.255.0",
		"new_routers":     "10.37.30.254",
		"interface":       "ens3f0",
	})

	if got != "" {
		t.Fatalf("resolv.conf should be untouched when lease carries no DNS, got:\n%s", got)
	}
}
