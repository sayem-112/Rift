package library

// The encrypted file format, shared with the web client.
//
// A book's plaintext is split into 8 MiB parts, one S3 multipart part each
// (S3 and R2 require parts of at least 5 MiB, except the last). Each part is
// encrypted as a run of 1 MiB records:
//
//	record = nonce (12 bytes) || AES-256-GCM ciphertext || tag (16 bytes)
//
// so every record adds 28 bytes. Records keep decryption granular: a later
// version can fetch a byte range and decrypt just the records it needs.
// Because the layout is deterministic, the server knows the exact ciphertext
// size of every part from the declared plaintext size alone, and signs each
// part's upload URL for exactly that many bytes.
const (
	PartPlainSize   int64 = 8 << 20
	RecordPlainSize int64 = 1 << 20
	RecordOverhead  int64 = 12 + 16
)

// PartCount is how many parts a book of the given plaintext size uploads in.
func PartCount(size int64) int32 {
	return int32((size + PartPlainSize - 1) / PartPlainSize)
}

// PartCipherSize is the exact encrypted size of part n (1-based).
func PartCipherSize(size int64, n int32) int64 {
	start := int64(n-1) * PartPlainSize
	plain := min(PartPlainSize, size-start)
	records := (plain + RecordPlainSize - 1) / RecordPlainSize
	return plain + records*RecordOverhead
}

// CipherSize is the exact encrypted size of the whole file.
func CipherSize(size int64) int64 {
	var total int64
	for n := int32(1); n <= PartCount(size); n++ {
		total += PartCipherSize(size, n)
	}
	return total
}
