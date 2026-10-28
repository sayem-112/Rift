package library

import "testing"

func TestLayout(t *testing.T) {
	const MiB = 1 << 20
	tests := []struct {
		size       int64
		parts      int32
		lastPart   int64
		cipherSize int64
	}{
		{size: 1, parts: 1, lastPart: 1 + 28, cipherSize: 1 + 28},
		{size: MiB, parts: 1, lastPart: MiB + 28, cipherSize: MiB + 28},
		{size: MiB + 1, parts: 1, lastPart: MiB + 1 + 2*28, cipherSize: MiB + 1 + 2*28},
		{size: 8 * MiB, parts: 1, lastPart: 8*MiB + 8*28, cipherSize: 8*MiB + 8*28},
		{size: 8*MiB + 5, parts: 2, lastPart: 5 + 28, cipherSize: 8*MiB + 8*28 + 5 + 28},
		{size: 50 * MiB, parts: 7, lastPart: 2*MiB + 2*28, cipherSize: 50*MiB + 50*28},
	}
	for _, tt := range tests {
		if got := PartCount(tt.size); got != tt.parts {
			t.Errorf("PartCount(%d) = %d, want %d", tt.size, got, tt.parts)
		}
		if got := PartCipherSize(tt.size, tt.parts); got != tt.lastPart {
			t.Errorf("last part of %d = %d, want %d", tt.size, got, tt.lastPart)
		}
		if got := CipherSize(tt.size); got != tt.cipherSize {
			t.Errorf("CipherSize(%d) = %d, want %d", tt.size, got, tt.cipherSize)
		}
	}
	// Every part except the last must meet S3's 5 MiB minimum.
	for n := int32(1); n < PartCount(50*MiB); n++ {
		if PartCipherSize(50*MiB, n) < 5*MiB {
			t.Fatalf("part %d is below the S3 minimum part size", n)
		}
	}
}
