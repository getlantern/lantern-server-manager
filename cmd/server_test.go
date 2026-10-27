package main

import (
	"net/url"
	"strconv"
	"testing"
)

func TestGetNewServerURL(t *testing.T) {
	c := &ServerConfig{ExternalIP: "203.0.113.7", Port: 8443, AccessToken: "a.b+c/d="}

	u, err := url.Parse(c.GetNewServerURL())
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if u.Scheme != "lantern" || u.Host != "private-server" {
		t.Fatalf("expected lantern://private-server, got %s://%s", u.Scheme, u.Host)
	}
	q := u.Query()
	if q.Get("ip") != c.ExternalIP {
		t.Errorf("ip = %q, want %q", q.Get("ip"), c.ExternalIP)
	}
	if q.Get("port") != strconv.Itoa(c.Port) {
		t.Errorf("port = %q, want %d", q.Get("port"), c.Port)
	}
	if q.Get("token") != c.AccessToken {
		t.Errorf("token = %q, want %q", q.Get("token"), c.AccessToken)
	}
	if q.Get("exp") != strconv.FormatInt(AdminExpirationTime.Unix(), 10) {
		t.Errorf("exp = %q, want %d", q.Get("exp"), AdminExpirationTime.Unix())
	}
}
