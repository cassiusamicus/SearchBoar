package netsearch

import "testing"

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
