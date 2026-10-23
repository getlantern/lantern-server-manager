package common

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Run in a disposable root filesystem with the sing-box-extensions package
// installed. This exercises the config path consumed by its real service.
func TestPackagedProxyConfig(t *testing.T) {
	if os.Getenv("LANTERN_PACKAGE_TEST") != "1" {
		t.Skip("set LANTERN_PACKAGE_TEST=1 inside a disposable package-test container")
	}
	if noSystemd {
		t.Fatal("package test requires NO_SYSTEMD to be unset")
	}
	dataDir := t.TempDir()
	if _, err := GenerateBasicSingBoxServerConfig(dataDir, 18443); err != nil {
		t.Fatal(err)
	}
	serviceConfig, err := os.ReadFile("/etc/lantern-box/config.json")
	if err != nil {
		t.Fatal(err)
	}
	dataConfig, err := os.ReadFile(filepath.Join(dataDir, "sing-box-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serviceConfig, dataConfig) {
		t.Fatal("service config differs from manager's saved config")
	}
	if output, err := exec.Command(SingBoxExe, "check", "--config", "/etc/lantern-box/config.json").CombinedOutput(); err != nil {
		t.Fatalf("packaged proxy rejected its config: %v: %s", err, output)
	}
	if err := ValidateSingBoxConfig(dataDir); err != nil {
		t.Fatal(err)
	}
	config, err := ReadSingBoxServerConfig(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	inbound, err := GetShadowsocksInboundConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if inbound.ListenPort != 18443 {
		t.Fatalf("listen port = %d, want 18443", inbound.ListenPort)
	}
}
