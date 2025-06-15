# 開発者向けドキュメント

## 開発環境の準備

### 前提条件

- Go 1.21以降
- Make
- Docker (統合テスト用)

### 初期セットアップ

```bash
git clone https://github.com/signity/backup-log-to-s3.git
cd backup-log-to-s3
make dev/setup
```

## 開発用コマンド

### ビルド

```bash
make build/dev          # 開発用ビルド
make build/all          # 全プラットフォーム向けビルド
make build/prod         # 本番用ビルド（静的バイナリ）
make build/docker       # Dockerイメージビルド
```

### テスト実行

```bash
make test/unit          # ユニットテスト
make test/integration   # 統合テスト（Docker必要）
make test/all           # 全テスト実行
make test/coverage      # カバレッジ測定
make test/coverage/html # HTMLカバレッジレポート
```

### コード品質

```bash
make code-quality/lint  # リンター実行（全て）
make code-quality/format # コードフォーマット
make code-quality/lint/security # セキュリティチェック
make code-quality/lint/vuln # 脆弱性チェック
```

### 開発ワークフロー

```bash
make dev/all            # 完全な開発ワークフロー（lint + test + build）
make dev/watch          # ファイル変更監視＆自動テスト
make dev/deps           # 依存関係の管理
```

### デバッグ

```bash
make debug/run          # デバッグモード実行
make debug/test         # デバッグモードテスト
```

## S3バケット作成

開発・テスト用のS3バケットを作成する場合：

```bash
# AWS CLIを使用
aws s3 mb s3://your-test-bucket-name

# 必要な権限を設定
aws s3api put-bucket-policy --bucket your-test-bucket-name --policy '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::YOUR_ACCOUNT_ID:user/YOUR_USER"},
      "Action": [
        "s3:PutObject",
        "s3:PutObjectAcl",
        "s3:HeadBucket"
      ],
      "Resource": [
        "arn:aws:s3:::your-test-bucket-name",
        "arn:aws:s3:::your-test-bucket-name/*"
      ]
    }
  ]
}'
```

## テスト実行

### ユニットテスト

```bash
# 基本的なユニットテスト
make test/unit

# race検出を含むテスト
make dev/test

# 特定のテストのみ実行
go test -v ./internal/... -run TestSpecificFunction
```

### 統合テスト

統合テストにはDockerとLocalStackが必要です：

```bash
# Dockerが起動していることを確認
docker info

# 統合テストを実行
make test/integration

# 環境変数を設定して実行
TEST_S3_BUCKET=test-bucket make test/integration
```

## トラブルシューティング

### ビルドエラー

```bash
# 依存関係をクリーンアップ
make clean
go mod tidy
go mod download

# ビルドキャッシュをクリア
go clean -cache
```

### テストの失敗

```bash
# verboseモードでテスト実行
go test -v ./...

# 特定のパッケージのみテスト
go test -v ./internal/s3/...

# テストのタイムアウトを増やす
go test -v -timeout 30m ./...
```

### LocalStackの問題

```bash
# LocalStackコンテナを確認
docker ps | grep localstack

# LocalStackのログを確認
docker logs localstack_main

# LocalStackを再起動
docker-compose down
docker-compose up -d
```

## リリースプロセス

```bash
# バージョンを更新
make version/bump

# リリース準備
make ci/release

# タグを作成してプッシュ
git tag v1.0.0
git push origin v1.0.0
```

## CI/CD

GitHub Actionsを使用した自動化：

```bash
# CI用ビルド
make ci/build

# CI用テスト（カバレッジ付き）
make ci/test

# CI用リント（全チェック）
make ci/lint
```

## 貢献

1. リポジトリをフォーク
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add amazing feature'`)
4. ブランチにプッシュ (`git push origin feature/amazing-feature`)
5. プルリクエストを作成

### コーディング規約

- `gofmt`でフォーマット
- `golangci-lint`でリント
- テストカバレッジ80%以上を維持
- すべての公開関数にドキュメントを記載

### コミットメッセージ

以下の形式を使用：

```
type(scope): subject

body

footer
```

Type:
- feat: 新機能
- fix: バグ修正
- docs: ドキュメントのみの変更
- style: コードの意味に影響しない変更
- refactor: バグ修正や機能追加を伴わないコード変更
- test: テストの追加や修正
- chore: ビルドプロセスやツールの変更