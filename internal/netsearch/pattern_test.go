package netsearch

import "testing"

func TestMatchGlobSimple(t *testing.T) {
	if !matchGlob("*.txt", "notes.txt") {
		t.Error("expected *.txt to match notes.txt")
	}
	if matchGlob("*.txt", "notes.md") {
		t.Error("expected *.txt not to match notes.md")
	}
}

func TestMatchGlobBraceExpansion(t *testing.T) {
	pattern := "*.{jpg,jpeg,png}"
	for _, name := range []string{"photo.jpg", "photo.jpeg", "photo.png"} {
		if !matchGlob(pattern, name) {
			t.Errorf("expected %q to match %q", pattern, name)
		}
	}
	if matchGlob(pattern, "photo.gif") {
		t.Error("expected photo.gif not to match")
	}
}

func TestMatchGlobCaseInsensitive(t *testing.T) {
	if !matchGlob("*.TXT", "notes.txt") {
		t.Error("expected case-insensitive match")
	}
}

func TestHostsInCIDR(t *testing.T) {
	hosts, err := hostsInCIDR("192.168.1.0/30", 20)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"192.168.1.0", "192.168.1.1", "192.168.1.2", "192.168.1.3"}
	if len(hosts) != len(want) {
		t.Fatalf("got %v, want %v", hosts, want)
	}
	for i := range want {
		if hosts[i] != want[i] {
			t.Errorf("hosts[%d] = %s, want %s", i, hosts[i], want[i])
		}
	}
}

func TestHostsInCIDRRespectsCap(t *testing.T) {
	hosts, err := hostsInCIDR("10.0.0.0/16", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 20 {
		t.Errorf("expected cap of 20 hosts, got %d", len(hosts))
	}
}
