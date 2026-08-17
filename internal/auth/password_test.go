package auth

import "testing"

func TestHashPasswordAndVerify(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatalf("哈希不应为空")
	}
	if !VerifyPassword("s3cret-password", hash) {
		t.Errorf("正确密码应校验通过")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Errorf("错误密码不应校验通过")
	}
}

func TestHashPasswordRandomSalt(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")
	if h1 == h2 {
		t.Errorf("相同密码两次哈希应不同（盐随机），否则说明盐未生效")
	}
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	if VerifyPassword("anything", "not-a-valid-hash") {
		t.Errorf("非法哈希应返回 false")
	}
}
