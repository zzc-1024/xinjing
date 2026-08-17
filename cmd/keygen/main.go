// keygen 是心境平台生成 RSA 密钥对的命令行工具。
// 用法：go run ./cmd/keygen [-dir ./keys] [-bits 2048]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"xinjing/internal/auth"
)

func main() {
	dir := flag.String("dir", "./keys", "密钥输出目录")
	bits := flag.Int("bits", 2048, "RSA 密钥位数（2048 或 4096）")
	flag.Parse()

	if *bits != 2048 && *bits != 4096 {
		log.Fatalf("unsupported bits %d (support 2048 / 4096)", *bits)
	}

	key, err := auth.GenerateRSAKeyPair(*bits)
	if err != nil {
		log.Fatalf("generate key pair: %v", err)
	}

	if err := os.MkdirAll(*dir, 0o700); err != nil {
		log.Fatalf("create dir %s: %v", *dir, err)
	}

	privPath := filepath.Join(*dir, "private.pem")
	if err := os.WriteFile(privPath, auth.MarshalRSAPrivateKeyPEM(key), 0o600); err != nil {
		log.Fatalf("write private key: %v", err)
	}

	pubPath := filepath.Join(*dir, "public.pem")
	if err := os.WriteFile(pubPath, auth.MarshalRSAPublicKeyPEM(&key.PublicKey), 0o644); err != nil {
		log.Fatalf("write public key: %v", err)
	}

	fmt.Println("key pair generated:")
	fmt.Printf("  private key (issuer only): %s\n", privPath)
	fmt.Printf("  public key  (distribute)  : %s\n", pubPath)
	fmt.Println()
	fmt.Println("configure in .env:")
	fmt.Printf("  XINJING_JWT_PRIVATE_KEY=%s\n", privPath)
	fmt.Printf("  XINJING_JWT_PUBLIC_KEY=%s\n", pubPath)
}
