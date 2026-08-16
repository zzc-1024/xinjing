package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	// httptest.NewRecorder 是一个"假"的 ResponseWriter，
	// 它记住所有写入的响应内容，供测试检查。
	w := httptest.NewRecorder()

	JSON(w, 200, map[string]string{"status": "ok"})

	// 检查状态码
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	// 检查 Content-Type 头
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// 反序列化响应体，检查内容
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body.status = %q, want ok", body["status"])
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, 404, "not found")

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}

	var errBody ErrorBody
	if err := json.Unmarshal(w.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if errBody.Error.Code != 404 {
		t.Errorf("error.code = %d, want 404", errBody.Error.Code)
	}
	if errBody.Error.Message != "not found" {
		t.Errorf("error.message = %q, want not found", errBody.Error.Message)
	}
}
