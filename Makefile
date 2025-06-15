.DEFAULT_GOAL := help
BINARY_NAME=backup-log-to-s3
VERSION=1.0.0
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"
STATIC_LDFLAGS=-ldflags "-extldflags '-static' -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# PHONY declarations
.PHONY: help build test run clean
.PHONY: dev/setup dev/deps dev/all dev/test dev/watch
.PHONY: code-quality/format code-quality/lint code-quality/lint/code code-quality/lint/security code-quality/lint/vuln
.PHONY: test/unit test/integration test/all test/coverage test/coverage/html test/watch
.PHONY: build/dev build/prod build/linux build/docker build/all
.PHONY: deploy/check deploy/build deploy/package
.PHONY: run/help run/dry run/example
.PHONY: install/local install/system
.PHONY: ci/test ci/lint ci/build ci/release
.PHONY: docs/generate docs/serve
.PHONY: debug/test debug/run
.PHONY: version/show version/bump

# ==============================================================================
# Primary Commands (最もよく使うコマンド)
# ==============================================================================

help: ## ヘルプメッセージ表示
	@echo "Makefile Commands"
	@echo ""
	@# Primary commands (no slash)
	@primary_commands=$$(grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -v '/' | grep -v '## \[DEPRECATED\]'); \
	if [ -n "$$primary_commands" ]; then \
		echo "$$primary_commands" | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'; \
		echo ""; \
	fi
	@# Hierarchical commands (with slash) - group by prefix
	@grep -E '^[a-zA-Z_-]+/[a-zA-Z_/-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	grep -v '## \[DEPRECATED\]' | \
	sort | \
	awk 'BEGIN {FS = ":.*?## "; current_group = ""} \
	{ \
		split($$1, parts, "/"); \
		group = parts[1]; \
		if (group != current_group) { \
			if (current_group != "") print ""; \
			print group ":"; \
			current_group = group; \
		} \
		printf "  %-20s %s\n", $$1, $$2; \
	}'

build: build/dev ## アプリケーションのビルド

test: test/unit ## テストの実行

run: run/help ## アプリケーションの実行

clean: ## クリーンアップ
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out coverage.html
	rm -rf dist/

# ==============================================================================
# Development Commands (開発者向けコマンド)
# ==============================================================================

# 開発環境
dev/setup: ## 開発環境の初期セットアップ
	@echo "Setting up development environment..."
	@$(MAKE) dev/deps
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development setup complete!"

dev/deps: ## 依存関係の管理
	$(GOMOD) download
	$(GOMOD) tidy

dev/all: dev/deps code-quality/lint test/unit build/dev ## 完全な開発ワークフロー（lint + test + build）

dev/test: ## 開発用テスト（race検出付き）
	@echo "Running development tests..."
	$(GOTEST) -race -v ./... -count=1

dev/watch: ## ファイル変更監視＆自動テスト
	@echo "Watching for changes... (requires 'entr' command)"
	@if ! command -v entr >/dev/null 2>&1; then \
		echo "Error: 'entr' command not found. Install it first:"; \
		echo "  macOS: brew install entr"; \
		echo "  Ubuntu: apt-get install entr"; \
		exit 1; \
	fi
	find . -name "*.go" | entr -c make test

# ==============================================================================
# Code Quality Commands (コード品質)
# ==============================================================================

code-quality/format: ## コードフォーマット
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	gofmt -s -w .

code-quality/lint: code-quality/lint/code ## リンター実行（全て）

code-quality/lint/code: ## コードの静的解析
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run

code-quality/lint/security: ## セキュリティチェック
	go run github.com/securego/gosec/v2/cmd/gosec ./...

code-quality/lint/vuln: ## 脆弱性チェック
	go run golang.org/x/vuln/cmd/govulncheck ./...

# ==============================================================================
# Test Commands (テスト)
# ==============================================================================

test/unit: ## ユニットテスト
	$(GOTEST) -v ./... -count=1

test/integration: ## 統合テスト（Docker必要）
	@echo "Running integration tests (requires Docker)..."
	@if ! docker version >/dev/null 2>&1; then \
		echo "Error: Docker is not running. Please start Docker first."; \
		exit 1; \
	fi
	$(GOTEST) -tags=integration -v -timeout 10m ./... -count=1

test/all: ## 全テスト実行
	@echo "Running all tests..."
	@$(MAKE) test/unit
	@echo "Running file operations tests..."
	@$(GOTEST) -v -run "TestFileOperations|TestLockFileOperations|TestFindTargetFilesWithRealFS" ./... -count=1
	@$(MAKE) test/integration

test/coverage: ## カバレッジ測定
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=coverage.out ./... -count=1
	$(GOCMD) tool cover -func=coverage.out | tail -1

test/coverage/html: ## HTMLカバレッジレポート
	@echo "Generating HTML coverage report..."
	$(GOTEST) -coverprofile=coverage.out ./... -count=1
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@if command -v open >/dev/null 2>&1; then \
		open coverage.html; \
	elif command -v xdg-open >/dev/null 2>&1; then \
		xdg-open coverage.html; \
	else \
		echo "Please open coverage.html in your browser"; \
	fi

test/watch: dev/watch ## ファイル監視＆自動テスト（エイリアス）

# ==============================================================================
# Build Commands (ビルド)
# ==============================================================================

build/dev: ## 開発用ビルド
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

build/prod: ## 本番用ビルド（静的バイナリ）
	CGO_ENABLED=0 $(GOBUILD) -a $(STATIC_LDFLAGS) -o $(BINARY_NAME)-static .

build/linux/amd: ## Linux用 amd64 ビルド
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a $(STATIC_LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .

build/linux/arm: ## Linux arm64用ビルド
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -a $(STATIC_LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .

build/docker: ## Dockerイメージビルド
	@echo "Docker image build not implemented yet"
	@echo "TODO: Add Dockerfile and docker build command"

build/all: ## 全プラットフォーム向けビルド
	# Linux
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .
	
	# macOS
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	
	# Windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

# ==============================================================================
# Operations Commands (運用者向けコマンド)
# ==============================================================================

# デプロイ
deploy/check: ## デプロイ前チェック
	@echo "Checking deployment readiness..."
	@$(MAKE) test/all
	@$(MAKE) code-quality/lint
	@echo "Deployment check complete!"

deploy/build: ## デプロイ用ビルド
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a $(STATIC_LDFLAGS) -o $(BINARY_NAME)-deploy .

deploy/package: clean build/all ## 配布パッケージ作成
	@echo "Creating distribution packages..."
	@mkdir -p dist
	@tar -czf dist/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 README.md
	@tar -czf dist/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64 README.md
	@tar -czf dist/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 README.md
	@tar -czf dist/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64 README.md
	@zip -r dist/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe README.md
	@echo "Packages created in dist/ directory"

# 実行
run/help: build/dev ## ヘルプ表示
	./$(BINARY_NAME) -help

run/dry: build/dev ## ドライラン実行
	./$(BINARY_NAME) -dry-run -verbose "*YYYYMMDD.log.gz"

run/example: build/dev ## サンプル実行
	@echo "Creating test files..."
	@mkdir -p /tmp/backup-log-to-s3-test
	@date_str=$$(date -d "1 month ago" '+%Y%m%d' 2>/dev/null || date -v-1m '+%Y%m%d' 2>/dev/null || echo "20231215"); \
	echo "test log content $$date_str" > /tmp/backup-log-to-s3-test/app$$date_str.log.gz
	@echo "Running example with test files..."
	./$(BINARY_NAME) -bucket test-bucket -prefix test -region us-east-1 -output /tmp/test.log -dry-run -verbose "/tmp/backup-log-to-s3-test/*YYYYMMDD.log.gz"
	@echo "Cleaning up test files..."
	@rm -rf /tmp/backup-log-to-s3-test

# インストール
install/local: build/dev ## ローカルインストール
	@echo "Installing to ~/bin..."
	@mkdir -p ~/bin
	cp $(BINARY_NAME) ~/bin/
	chmod +x ~/bin/$(BINARY_NAME)
	@echo "Installation complete! Make sure ~/bin is in your PATH"

install/system: build/dev ## システムインストール
	@echo "Installing to /usr/local/bin..."
	sudo cp $(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete!"

# ==============================================================================
# CI/CD Commands (CI/CD向けコマンド)
# ==============================================================================

ci/test: ## CI用テスト（カバレッジ付き）
	$(GOTEST) -race -coverprofile=coverage.out ./... -count=1

ci/lint: ## CI用リント（全チェック）
	@$(MAKE) code-quality/lint/code
	@$(MAKE) code-quality/lint/security
	@$(MAKE) code-quality/lint/vuln

ci/build: ## CI用ビルド
	@$(MAKE) build/all

ci/release: ## リリース準備
	@echo "Preparing release..."
	@$(MAKE) ci/test
	@$(MAKE) ci/lint
	@$(MAKE) ci/build
	@$(MAKE) deploy/package
	@echo "Release preparation complete!"

# ==============================================================================
# Utility Commands (ユーティリティ)
# ==============================================================================

# ドキュメント
docs/generate: ## ドキュメント生成
	@echo "Document generation not implemented yet"
	@echo "TODO: Add document generation commands"

docs/serve: ## ドキュメントサーバー起動
	@echo "Document server not implemented yet"
	@echo "TODO: Add document server commands"

# デバッグ
debug/test: ## デバッグモードテスト
	$(GOTEST) -v -run TestExtractDateFromFilename ./...
	$(GOTEST) -v -run TestCalculateTargetMonth ./...

debug/run: build/dev ## デバッグモード実行
	./$(BINARY_NAME) -verbose -dry-run "*YYYYMMDD.log.gz"

# バージョン管理
version/show: ## バージョン表示
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

version/bump: ## バージョン更新
	@echo "Version bump not implemented yet"
	@echo "TODO: Add version bump functionality"

# ==============================================================================
# Aliases for backward compatibility
# ==============================================================================

# Old command aliases (for backward compatibility)
deps: dev/deps ## [DEPRECATED] 依存関係の管理（dev/depsを使用してください）
fmt: code-quality/format ## [DEPRECATED] コードフォーマット（code-quality/formatを使用してください）
lint: code-quality/lint ## [DEPRECATED] リント（code-quality/lintを使用してください）
coverage: test/coverage ## [DEPRECATED] カバレッジ（test/coverageを使用してください）
all: dev/all ## [DEPRECATED] 完全ワークフロー（dev/allを使用してください）