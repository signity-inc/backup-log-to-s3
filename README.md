# backup-log-to-s3

[![Go Report Card](https://goreportcard.com/badge/github.com/signity/backup-log-to-s3)](https://goreportcard.com/report/github.com/signity/backup-log-to-s3)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/signity/backup-log-to-s3)](https://github.com/signity/backup-log-to-s3/releases)

日付ベースのフィルタリングでログファイルをAmazon S3にバックアップするコマンドラインツール。

## 使い方

```bash
backup-log-to-s3 [OPTIONS] <period> <glob_pattern>
```

### 基本的な使用例

```bash
# 7日以上経過したログファイルをアップロード（ローカルファイルは保持）
backup-log-to-s3 -bucket my-logs -prefix archived "7 days" "/var/log/app-YYYYMMDD.log.gz"

# アップロード成功後にローカルファイルを削除
backup-log-to-s3 -bucket my-logs -prefix archived -delete "1 month" "/var/log/nginx-YYYY-MM-DD.log.gz"

# ドライランで処理対象を確認
backup-log-to-s3 -bucket my-logs -prefix test -dry-run "1 day" "*YYYYMMDD.log"

# アンダースコア形式の日付ファイル
backup-log-to-s3 -bucket my-logs -prefix "logs/YYYY/MM" "1 month" "system_YYYY_MM_DD.log.gz"
```

### 引数

- `period`: 対象期間（例: "1 day", "7 days", "1 month", "1 year"）
- `glob_pattern`: YYYYMMDD、YYYY-MM-DD、YYYY/MM/DD、またはYYYY_MM_DD形式の日付を含むファイルパターン

### オプション

| オプション | 説明 | デフォルト | 必須 |
|-----------|------|------------|------|
| `-bucket` | S3バケット名 | - | ✓ |
| `-prefix` | S3プレフィックス（日付フォーマット対応） | - | ✓ |
| `-region` | AWSリージョン | AWS_DEFAULT_REGION | |
| `-output` | ログファイル出力先 | stdout | |
| `-lock` | ロックファイルパス | /var/run/backup-log-to-s3.lock | |
| `-storage-class` | S3ストレージクラス | STANDARD_IA | |
| `-dry-run` | ドライランモード | false | |
| `-verbose` | 詳細ログ出力 | false | |
| `-delete` | アップロード成功後にローカルファイルを削除 | false | |
| `-help` | ヘルプ表示 | | |
| `-version` | バージョン表示 | | |

### AWS CLI互換オプション

| オプション | 説明 |
|-----------|------|
| `-profile` | AWS認証プロファイル |
| `-endpoint-url` | カスタムエンドポイントURL |
| `-no-verify-ssl` | SSL証明書の検証を無効化 |
| `-ca-bundle` | CA証明書バンドルのパス |
| `-cli-read-timeout` | ソケット読み取りタイムアウト（秒） |
| `-cli-connect-timeout` | ソケット接続タイムアウト（秒） |

### 期間の例

- `"1 day"` - 1日以上経過したファイル
- `"7 days"` - 7日以上経過したファイル  
- `"1 month"` - 1ヶ月以上経過したファイル
- `"2 months"` - 2ヶ月以上経過したファイル
- `"1 year"` - 1年以上経過したファイル

## 日付ベースディレクトリ構造

`-prefix`オプションでは、ファイル名から抽出した日付を使用して、S3内に日付ベースのディレクトリ構造を作成できます。

### 日付フォーマットトークン

| トークン | 説明 | 例 |
|----------|------|-----|
| `YYYY` | 4桁の年 | 2024 |
| `MM` | 2桁の月（ゼロ埋め） | 12 |
| `DD` | 2桁の日（ゼロ埋め） | 15 |

### プレフィックスの例

| プレフィックス | 保存先パス | 説明 |
|---------------|------------|------|
| `"logs"` | `bucket/logs/filename.log.gz` | 従来通り（日付置換なし） |
| `"logs/YYYY"` | `bucket/logs/2024/filename.log.gz` | 年別ディレクトリ |
| `"logs/YYYY/MM"` | `bucket/logs/2024/12/filename.log.gz` | 年月別ディレクトリ |
| `"logs/YYYY/MM/DD"` | `bucket/logs/2024/12/15/filename.log.gz` | 年月日別ディレクトリ |
| `"backup/YYYY/MM"` | `bucket/backup/2024/12/filename.log.gz` | カスタムベースパス |

### 注意事項

- 日付はファイル名から自動抽出されます（対応パターン：`YYYYMMDD`, `YYYY-MM-DD`, `YYYY/MM/DD`, `YYYY_MM_DD`）
- ファイル名に日付パターンが含まれていない場合、アップロードはエラーで失敗します
- プレフィックスに日付トークン（`YYYY`, `MM`, `DD`）が含まれていない場合は、従来通りの動作となります

### パターンの例

- `"*YYYYMMDD.log.gz"` - `app20241215.log.gz`にマッチ
- `"YYYY-MM-DD.gz"` - `2024-12-15.gz`にマッチ
- `"YYYY/MM/DD.gz"` - `2024/12/15.gz`にマッチ
- `"YYYY_MM_DD.gz"` - `2024_12_15.gz`にマッチ
- `"/var/log/app*YYYYMMDD.gz"` - `/var/log/app20241215.gz`にマッチ
- `"nginx-YYYY-MM-DD.log.gz"` - `nginx-2024-12-15.log.gz`にマッチ
- `"access_YYYY/MM/DD.log.gz"` - `access_2024/12/15.log.gz`にマッチ
- `"system_YYYY_MM_DD.log.gz"` - `system_2024_12_15.log.gz`にマッチ

## インストール

### Homebrew (macOS/Linux)

```bash
brew install signity/tap/backup-log-to-s3
```

### APT (Ubuntu/Debian)

```bash
# x86_64
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3_0.4.0_amd64.deb -o backup-log-to-s3.deb
sudo dpkg -i backup-log-to-s3.deb

# ARM64
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3_0.4.0_arm64.deb -o backup-log-to-s3.deb
sudo dpkg -i backup-log-to-s3.deb
```

### DNF/YUM (RedHat/CentOS/Fedora)

```bash
# リポジトリを追加してインストール
sudo dnf config-manager --add-repo https://signity-inc.github.io/backup-log-to-s3/rpm/backup-log-to-s3.repo
sudo dnf install backup-log-to-s3

# または直接RPMをダウンロード
# x86_64
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3-0.4.0-1.x86_64.rpm -o backup-log-to-s3.rpm
sudo rpm -i backup-log-to-s3.rpm
```

### バイナリダウンロード

[GitHub Releases](https://github.com/signity-inc/backup-log-to-s3/releases)

```bash
# Linux x86_64
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3-Linux-x86_64.tar.gz | tar xz

# Linux ARM64
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3-Linux-arm64.tar.gz | tar xz

# macOS x86_64
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3-Darwin-x86_64.tar.gz | tar xz

# macOS ARM64 (Apple Silicon)
curl -L https://github.com/signity-inc/backup-log-to-s3/releases/latest/download/backup-log-to-s3-Darwin-arm64.tar.gz | tar xz
```

## 使用例

### 基本的な使用例

```bash
# 従来通り（日付ディレクトリなし）
backup-log-to-s3 -bucket my-logs -prefix logs "7 days" "*YYYYMMDD.log.gz"

# 年別ディレクトリ構造
backup-log-to-s3 -bucket my-logs -prefix "logs/YYYY" "1 month" "*YYYYMMDD.log.gz"

# 年月別ディレクトリ構造
backup-log-to-s3 -bucket my-logs -prefix "logs/YYYY/MM" "1 month" "*YYYYMMDD.log.gz"

# 年月日別ディレクトリ構造
backup-log-to-s3 -bucket my-logs -prefix "logs/YYYY/MM/DD" "1 month" "*YYYYMMDD.log.gz"

# アンダースコア形式のファイル名
backup-log-to-s3 -bucket my-logs -prefix "logs/YYYY/MM" "1 month" "system_YYYY_MM_DD.log.gz"
```

### 高度な使用例

```bash
# 特定のAWSプロファイルとリージョンを使用
backup-log-to-s3 -profile production -region eu-west-1 -bucket eu-logs -prefix "app/YYYY/MM" "30 days" "*YYYYMMDD.log.gz"

# 詳細ログをファイルに出力
backup-log-to-s3 -bucket logs -prefix "archived/YYYY/MM" -verbose -output /var/log/backup.log "1 month" "/logs/*YYYYMMDD.log.gz"

# S3互換ストレージ用カスタムエンドポイント
backup-log-to-s3 -bucket minio-bucket -prefix "backups/YYYY" -endpoint-url http://localhost:9000 "7 days" "*YYYYMMDD.log"

# アップロード後にローカルファイルを削除
backup-log-to-s3 -bucket my-logs -prefix "logs/YYYY/MM/DD" -delete "30 days" "*YYYYMMDD.log.gz"
```

## AWS認証

標準的なAWS認証方法に対応:

- AWS認証ファイル (`~/.aws/credentials`)
- 環境変数 (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
- IAMロール (EC2インスタンス用)
- AWSプロファイル (`-profile`フラグ)

### 必要なS3権限

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:PutObjectAcl",
        "s3:HeadBucket"
      ],
      "Resource": [
        "arn:aws:s3:::your-bucket-name",
        "arn:aws:s3:::your-bucket-name/*"
      ]
    }
  ]
}
```

## ライセンス

このプロジェクトはMITライセンスの下でライセンスされています。詳細は[LICENSE](LICENSE)ファイルを参照してください。
