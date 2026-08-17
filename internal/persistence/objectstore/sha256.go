package objectstore

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
)

// HashingReader 在读取数据的同时累计 sha256 摘要。
// 把它包在数据源外层（如 io.Copy(dst, NewHashingReader(src))），
// 传输结束后用 Size/SumHex 取字节数与十六进制摘要。
type HashingReader struct {
	r io.Reader
	h hash.Hash
	n int64
}

// NewHashingReader 用 sha256 包装 r。
func NewHashingReader(r io.Reader) *HashingReader {
	return &HashingReader{r: r, h: sha256.New()}
}

// Read 从底层 reader 读取，并把读到的字节喂给哈希器。
func (h *HashingReader) Read(p []byte) (int, error) {
	n, err := h.r.Read(p)
	if n > 0 {
		h.n += int64(n)
		_, _ = h.h.Write(p[:n])
	}
	return n, err
}

// Size 返回已读取的字节数。
func (h *HashingReader) Size() int64 {
	return h.n
}

// SumHex 返回已读数据的 sha256 十六进制摘要。
func (h *HashingReader) SumHex() string {
	return hex.EncodeToString(h.h.Sum(nil))
}
