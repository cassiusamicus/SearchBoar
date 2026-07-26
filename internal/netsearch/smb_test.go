package netsearch

import (
	"context"
	"reflect"
	"testing"
)

// smbLsFixture mirrors smbclient's documented `ls` output shape (name,
// right-justified attribute letters, right-justified byte size, then a
// ctime-style date) -- see parseSMBLsOutput's own comment for why this is
// parsed field-by-field from the right rather than by fixed column offset.
const smbLsFixture = `  .                                   D        0  Sat Jan  1 00:00:00 2026
  ..                                  D        0  Sat Jan  1 00:00:00 2026
  Documents                           D        0  Sat Jan  1 00:00:00 2026
  Photos                              D        0  Sat Jan  1 00:00:00 2026
  readme.txt                          A        6  Sat Jan  1 00:00:00 2026

		9007199 blocks of size 1024. 1234567 blocks available
`

// smbShareListFixture is real observed smbclient -L output (a newer
// smbclient version's "Reconnecting with SMB1..." line included) --
// guards a real bug caught during live verification: both that line and
// the "---------" divider row were being misparsed as share names before
// parseSMBShareListOutput started filtering on the Type column.
const smbShareListFixture = "\tSharename       Type      Comment\n" +
	"\t---------       ----      -------\n" +
	"\tprint$          Disk      Printer Drivers\n" +
	"\tHOME            Disk      \n" +
	"\tDriveD1         Disk      \n" +
	"\tIPC$            IPC       IPC Service (Samba 4.24.4)\n" +
	"Reconnecting with SMB1 for workgroup listing.\n" +
	"\n" +
	"\tServer               Comment\n" +
	"\t---------            -------\n"

func TestParseSMBShareListOutputSkipsDividerAndInfoLines(t *testing.T) {
	got := parseSMBShareListOutput([]byte(smbShareListFixture))
	want := []string{"HOME", "DriveD1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSMBShareListOutput() = %v, want %v", got, want)
	}
}

func TestParseSMBShareListOutputEmpty(t *testing.T) {
	if got := parseSMBShareListOutput([]byte("")); got != nil {
		t.Errorf("parseSMBShareListOutput(\"\") = %v, want nil", got)
	}
}

func TestParseSMBLsOutputSeparatesDirsFromFiles(t *testing.T) {
	entries := parseSMBLsOutput([]byte(smbLsFixture))
	want := []smbEntry{
		{Name: "Documents", IsDir: true},
		{Name: "Photos", IsDir: true},
		{Name: "readme.txt", IsDir: false},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("parseSMBLsOutput() = %+v, want %+v", entries, want)
	}
}

func TestParseSMBLsOutputSkipsDotAndDotDot(t *testing.T) {
	entries := parseSMBLsOutput([]byte(smbLsFixture))
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			t.Errorf("parseSMBLsOutput() included %q, want it skipped", e.Name)
		}
	}
}

func TestParseSMBLsOutputEmpty(t *testing.T) {
	if entries := parseSMBLsOutput([]byte("")); entries != nil {
		t.Errorf("parseSMBLsOutput(\"\") = %+v, want nil", entries)
	}
}

func TestParseSMBLsOutputIgnoresSummaryLine(t *testing.T) {
	entries := parseSMBLsOutput([]byte("\t\t9007199 blocks of size 1024. 1234567 blocks available\n"))
	if entries != nil {
		t.Errorf("parseSMBLsOutput(summary only) = %+v, want nil", entries)
	}
}

// TestProbeSMBHostsTCPReturnsCleanly checks the dial loop's up/down
// bookkeeping doesn't error or hang against a single-host range -- not
// asserting on whether 127.0.0.1:445 itself is reachable, since that
// depends on whether the machine running this test happens to have a real
// SMB server bound to localhost (probeSMBHostsTCP always dials the fixed
// port 445, so a controlled "host is up" case isn't practical to set up in
// a unit test without root to bind it).
func TestProbeSMBHostsTCPReturnsCleanly(t *testing.T) {
	hosts, err := probeSMBHostsTCP(context.Background(), "127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) > 1 {
		t.Errorf("probeSMBHostsTCP(single-host range) = %v, want at most 1 host", hosts)
	}
}

func TestProbeSMBHostsTCPRespectsHostCap(t *testing.T) {
	hosts, err := probeSMBHostsTCP(context.Background(), "10.255.255.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) > maxNFSHostsScanned {
		t.Errorf("probeSMBHostsTCP() returned %d hosts, want at most %d", len(hosts), maxNFSHostsScanned)
	}
}
