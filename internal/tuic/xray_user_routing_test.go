package tuic_test

// End-to-end check for #6874 against the embedded xray-core: the TUIC relay's
// SOCKS5 inbound, configured the way injectTuicSocks generates it, must let a
// "user" routing rule match the TUIC client's email.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	_ "github.com/xtls/xray-core/app/router"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
	_ "github.com/xtls/xray-core/proxy/blackhole"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/socks"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startEcho(t *testing.T) *net.TCPAddr {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	return l.Addr().(*net.TCPAddr)
}

func startXray(t *testing.T, socksPort int, socksSettings []byte) {
	t.Helper()
	cfg := fmt.Sprintf(`{
	  "log": {"loglevel": "none"},
	  "inbounds": [{"tag": "tuic-in", "listen": "127.0.0.1", "port": %d, "protocol": "socks", "settings": %s}],
	  "outbounds": [
	    {"tag": "direct", "protocol": "freedom"},
	    {"tag": "block", "protocol": "blackhole"}
	  ],
	  "routing": {"rules": [{"type": "field", "user": ["blocked@example.test"], "outboundTag": "block"}]}
	}`, socksPort, socksSettings)
	jc := &conf.Config{}
	if err := json.Unmarshal([]byte(cfg), jc); err != nil {
		t.Fatalf("decode xray config: %v", err)
	}
	pb, err := jc.Build()
	if err != nil {
		t.Fatalf("build xray config: %v", err)
	}
	inst, err := core.New(pb)
	if err != nil {
		t.Fatalf("new xray instance: %v", err)
	}
	if err := inst.Start(); err != nil {
		t.Fatalf("start xray: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })
}

// reaches reports whether user's connection through the relay gets an echo.
func reaches(t *testing.T, relay *tuic.SocksRelay, user string, echo *net.TCPAddr) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	target := &tuic.Address{Type: tuic.AddrTypeIPv4, IP: echo.IP.To4(), Port: uint16(echo.Port)}
	conn, err := relay.DialTCP(ctx, user, target)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	msg := []byte("hello " + user)
	if _, err := conn.Write(msg); err != nil {
		return false
	}
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, got); err != nil {
		return false
	}
	return bytes.Equal(got, msg)
}

func TestTuicRelayUserRoutingWithPerEmailAccounts(t *testing.T) {
	echo := startEcho(t)
	port := freePort(t)
	password := tuic.SocksPassword()
	settings, err := tuic.SocksInboundSettings([]string{"allowed@example.test", "blocked@example.test"}, password)
	if err != nil {
		t.Fatal(err)
	}
	startXray(t, port, settings)
	relay := &tuic.SocksRelay{Addr: fmt.Sprintf("127.0.0.1:%d", port), Password: password}

	if !reaches(t, relay, "allowed@example.test", echo) {
		t.Fatal("allowed user did not reach the target through the relay")
	}
	if reaches(t, relay, "blocked@example.test", echo) {
		t.Fatal(`"user" routing rule did not match the TUIC client's email (#6874)`)
	}
	if reaches(t, relay, "unknown@example.test", echo) {
		t.Fatal("an email without a relay account must be refused")
	}
}

// TestTuicRelayNoauthCannotRouteByUser documents the pre-fix behaviour: with
// the old noauth relay inbound Xray never learns the user, so the same rule
// does not block anyone.
func TestTuicRelayNoauthCannotRouteByUser(t *testing.T) {
	echo := startEcho(t)
	port := freePort(t)
	startXray(t, port, json.RawMessage(`{"auth":"noauth","udp":true}`))
	relay := &tuic.SocksRelay{Addr: fmt.Sprintf("127.0.0.1:%d", port), Password: "unused"}
	if !reaches(t, relay, "blocked@example.test", echo) {
		t.Fatal("expected the noauth relay to ignore user rules (pre-fix behaviour)")
	}
}
