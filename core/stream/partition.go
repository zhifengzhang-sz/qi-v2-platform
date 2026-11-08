package stream

// PartitionOf is the partition Kafka's default partitioner gives a key
// among n partitions: the key's murmur2 hash, its sign bit cleared, mod n
// (Apache Kafka's DefaultPartitioner and Utils.murmur2; franz-go's
// StickyKeyPartitioner for a record with a key). Every implementation
// places a record so, by its partition key, so that a publisher in any
// language with Kafka's default partitioner agrees with this one.
func PartitionOf(key []byte, n int32) int32 {
	if n <= 1 {
		return 0
	}
	return int32(murmur2(key)&0x7fffffff) % n
}

// murmur2 is Kafka's (Utils.murmur2): seed 0x9747b28c, the key's bytes
// read as little-endian 32-bit words.
func murmur2(b []byte) uint32 {
	const (
		seed uint32 = 0x9747b28c
		m    uint32 = 0x5bd1e995
		r           = 24
	)
	h := seed ^ uint32(len(b))
	for len(b) >= 4 {
		k := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		b = b[4:]
		k *= m
		k ^= k >> r
		k *= m
		h *= m
		h ^= k
	}
	switch len(b) {
	case 3:
		h ^= uint32(b[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(b[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(b[0])
		h *= m
	}
	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}
