# 心境(xinjing) 常用开发命令集合。
# 用法：make <目标>，例如 make build、make test、make run。
# 注意：每个「命令行」前必须是 Tab 键缩进（不能用空格），否则 make 会报错。
# 本机若没有 make（Windows 常见），可执行等价的 PowerShell 命令（见注释说明）。

# ---- 根据操作系统决定可执行文件后缀 ----
ifeq ($(OS),Windows_NT)
	SERVER_BIN := bin/server.exe
	AUTH_BIN := bin/auth.exe
else
	SERVER_BIN := bin/server
	AUTH_BIN := bin/auth
endif

# 不带参数执行 make 时，默认执行第一个目标（build）
.DEFAULT_GOAL := build

# .PHONY 声明这些名字只是命令别名，不对应磁盘上的真实文件
.PHONY: help build test vet fmt run run-auth clean keygen

help: ## 显示所有可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-8s %s
", $$1, $$2}'

build: ## 编译网关服务(cmd/server)与认证服务(cmd/auth)到 bin/
	@mkdir -p bin
	go build -o $(SERVER_BIN) ./cmd/server
	go build -o $(AUTH_BIN) ./cmd/auth

test: ## 运行全部单元测试（-count=1 跳过缓存强制重跑）
	go test -count=1 ./...

vet: ## 运行 go vet 静态检查
	go vet ./...

fmt: ## 检查代码格式，列出不符合 gofmt 规范的文件（修复：gofmt -w .）
	gofmt -l .

run: build ## 启动网关服务（认证服务用 run-auth）
	./$(SERVER_BIN)

run-auth: build ## 启动认证服务
	./$(AUTH_BIN)

clean: ## 清理构建产物
	rm -rf bin

keygen: ## 生成 RSA 密钥对到 keys/（等价于 go run ./cmd/keygen）
	go run ./cmd/keygen -dir ./keys
