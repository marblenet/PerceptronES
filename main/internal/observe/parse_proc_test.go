package observe

import "testing"

func TestParseStat(t *testing.T) {
	snap, err := parseStat("cpu  100 0 50 800 30 5 5 10 0 0\ncpu0 1 1 1 1 1 1 1 1\ncpu1 1 1 1 1 1 1 1 1\nctxt 12345\nprocs_running 3\nprocs_blocked 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if snap.total != 1000 || snap.system != 60 || snap.iowait != 30 || snap.steal != 10 || snap.cpus != 2 || snap.contextSwitches != 12345 || snap.running != 3 || snap.blocked != 2 {
		t.Fatalf("unexpected snapshot %#v", snap)
	}
	if _, err := parseStat("nothing here"); err == nil {
		t.Fatal("expected error without aggregate cpu line")
	}
}

func TestParseMeminfoAndVMStat(t *testing.T) {
	info := parseMeminfoBytes("MemTotal:       1000 kB\nHugePages_Total:       4\n")
	if info["MemTotal"] != 1024000 || info["HugePages_Total"] != 4 {
		t.Fatalf("unexpected meminfo %#v", info)
	}
	if vm := parseKeyValueCounters("pswpin 7\noom_kill 2\nbad x\n"); vm["pswpin"] != 7 || vm["oom_kill"] != 2 {
		t.Fatalf("unexpected vmstat %#v", vm)
	}
}

func TestParseDiskstatsOnlyCountsWholeDisks(t *testing.T) {
	content := "   8 0 sda 10 0 100 5 20 0 200 6 0 40 11 0\n   8 1 sda1 9 0 90 5 19 0 190 6 0 30 11 0\n"
	total, per, err := parseDiskstats(content, map[string]bool{"sda": true})
	if err != nil {
		t.Fatal(err)
	}
	if total.reads != 10 || total.readSectors != 100 || total.writes != 20 || total.writeSectors != 200 || total.ioMillis != 40 || per["sda"] != 40 {
		t.Fatalf("unexpected counters %#v %#v", total, per)
	}
	if _, _, err := parseDiskstats(content, map[string]bool{}); err == nil {
		t.Fatal("expected error when no disks match")
	}
}

func TestParseNetworkSources(t *testing.T) {
	dev := "Inter-|   Receive\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n    lo: 999 1 9 9 0 0 0 0 999 1 9 9 0 0 0 0\n  eth0: 1000 10 1 2 0 0 0 0 2000 20 3 4 0 0 0 0\n"
	total, err := parseNetDevExternal(dev)
	if err != nil || total.rxBytes != 1000 || total.txBytes != 2000 || total.errors != 4 || total.dropped != 6 {
		t.Fatalf("unexpected totals %#v err=%v", total, err)
	}
	if _, err := parseNetDevExternal("    lo: 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1\n"); err == nil {
		t.Fatal("loopback-only host must be unavailable")
	}
	tcp, err := parseSNMPTcp("Tcp: RtoAlgorithm CurrEstab RetransSegs\nTcp: 1 42 7\n")
	if err != nil || tcp["CurrEstab"] != 42 || tcp["RetransSegs"] != 7 {
		t.Fatalf("unexpected tcp %#v err=%v", tcp, err)
	}
}

func TestParseSmallSources(t *testing.T) {
	if v, err := parsePSIAvg10("some avg10=12.50 avg60=1.00 avg300=0.00 total=1\nfull avg10=3.00 avg60=0 avg300=0 total=1\n"); err != nil || v != 12.5 {
		t.Fatalf("psi %v %v", v, err)
	}
	if _, err := parsePSIAvg10("garbage"); err == nil {
		t.Fatal("expected PSI error")
	}
	if a, m, err := parseFileNr("1200\t0\t12000\n"); err != nil || a != 1200 || m != 12000 {
		t.Fatalf("file-nr %v %v %v", a, m, err)
	}
	if _, _, err := parseFileNr("1 0 0"); err == nil {
		t.Fatal("zero maximum must be rejected")
	}
	state, threads, err := parseProcStat("42 (my (odd) proc) Z 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 4 0")
	if err != nil || state != "Z" || threads != 4 {
		t.Fatalf("proc stat %q %d %v", state, threads, err)
	}
}

func TestMountIsReadOnlyUsesLongestMatch(t *testing.T) {
	mounts := "/dev/sda1 / ext4 rw,relatime 0 0\n/dev/sdb1 /data ext4 ro,relatime 0 0\n"
	if ro, err := mountIsReadOnly(mounts, "/data/logs"); err != nil || !ro {
		t.Fatalf("expected read-only data mount: %v %v", ro, err)
	}
	if ro, err := mountIsReadOnly(mounts, "/var"); err != nil || ro {
		t.Fatalf("expected writable root mount: %v %v", ro, err)
	}
}
