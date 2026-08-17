package objectstore

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func TestHashingReader(t *testing.T) {
	data := "hello, xinjing"
	want := sha256.Sum256([]byte(data))
	wantHex := hex.EncodeToString(want[:])

	hr := NewHashingReader(strings.NewReader(data))

	// 用小缓冲区多次读取，模拟真实流式场景
	buf := make([]byte, 3)
	for {
		_, err := hr.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}

	if got := hr.SumHex(); got != wantHex {
		t.Fatalf("SumHex() = %q, want %q", got, wantHex)
	}
	if got := hr.Size(); got != int64(len(data)) {
		t.Fatalf("Size() = %d, want %d", got, len(data))
	}
}

func TestValidateKey(t *testing.T) {
	valid := []string{"a", "a/b.txt", "functions/abc/main.go"}
	for _, k := range valid {
		if err := validateKey(k); err != nil {
			t.Errorf("validateKey(%q) unexpected error: %v", k, err)
		}
	}
	invalid := []string{"", "/abs", "a/../b", "..", "a\\b", "a/.."}
	for _, k := range invalid {
		if err := validateKey(k); err == nil {
			t.Errorf("validateKey(%q) = nil, want error", k)
		}
	}
}
