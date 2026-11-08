package stream

import "testing"

// Apache Kafka's own cases for Utils.murmur2 (UtilsTest.testMurmur2), its
// int results here as the same bits.
func TestMurmur2(t *testing.T) {
	for key, want := range map[string]int32{
		"21":                         -973932308,
		"foobar":                     -790332482,
		"a-little-bit-long-string":   -985981536,
		"a-little-bit-longer-string": -1486304829,
		"lkjh234lh9fiuh90y23oiuhsafujhadof229phr9h19h89h8": -58897971,
		"abc": 479470107,
	} {
		if got := int32(murmur2([]byte(key))); got != want {
			t.Errorf("murmur2(%q) = %d, want %d", key, got, want)
		}
	}
}

func TestPartitionOf(t *testing.T) {
	for _, n := range []int32{0, 1} {
		if p := PartitionOf([]byte("SPY"), n); p != 0 {
			t.Errorf("PartitionOf(SPY, %d) = %d", n, p)
		}
	}
	// murmur2("21"), -973932308, with its sign bit cleared, mod 16
	h := int32(-973932308)
	if p, want := PartitionOf([]byte("21"), 16), int32(uint32(h)&0x7fffffff)%16; p != want {
		t.Errorf("PartitionOf(21, 16) = %d, want %d", p, want)
	}
}
