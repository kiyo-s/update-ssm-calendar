# 設計書

## 1. 概要

本ドキュメントは、AWS Systems Manager Change Calendarのイベント管理ツール `update-ssm-calendar` の設計を定義する。

## 2. アーキテクチャ

### 2.1 アーキテクチャスタイル

**パッケージベースアーキテクチャ**を採用する。

- 機能ごとにパッケージを分割し、適度な責務分離を実現
- 過度な抽象化を避け、Go言語らしいシンプルな設計を目指す
- テストのしやすさと保守性を重視

### 2.2 アーキテクチャ選択の理由

| 要件 | 対応 |
|------|------|
| 明確な機能分離(YAML, バリデーション, AWS操作) | ✓ パッケージごとに責務を分離 |
| 将来的な拡張可能性 | ✓ パッケージ追加で対応可能 |
| テストのしやすさ | ✓ インターフェースによるモック化 |
| 過度な複雑さの回避 | ✓ レイヤー分離より軽量 |

## 3. ディレクトリ構成

```
update-ssm-calendar/
├── cmd/
│   └── update-ssm-calendar/
│       └── main.go              # エントリーポイント
├── internal/
│   ├── config/                  # 設定・環境変数の読み込み
│   │   └── config.go
│   ├── models/                  # データ構造定義
│   │   └── event.go
│   ├── parser/                  # YAMLファイルのパース
│   │   ├── yaml.go
│   │   └── testdata/            # YAMLテストデータ
│   │       ├── valid.yaml
│   │       └── invalid.yaml
│   ├── validator/               # バリデーション
│   │   └── validator.go
│   ├── generator/               # イベント生成(週次→日次への展開)
│   │   └── generator.go
│   ├── calendar/                # Change Calendar操作(AWS SDK)
│   │   ├── calendar.go          # インターフェース + 実装
│   │   └── mock.go              # モック実装(テスト用)
│   ├── logger/                  # ログ出力
│   │   └── logger.go
│   └── errors/                  # エラー定義
│       └── errors.go
├── docs/
│   ├── 01_requirements.md
│   └── 02_designs.md
├── go.mod
├── go.sum
├── README.md
└── CLAUDE.md
```

### 3.1 `internal/` ディレクトリの使用

- すべてのパッケージを `internal/` 配下に配置
- 外部パッケージからのインポートを防止
- ツール専用のコードであり、外部公開の必要がないため

## 4. パッケージ設計

### 4.1 パッケージ一覧と責務

| パッケージ | 責務 | 主要な機能 |
|-----------|------|-----------|
| `cmd/update-ssm-calendar` | エントリーポイント | コマンドライン引数の解析、各パッケージの呼び出し、エラーハンドリング |
| `internal/config` | 設定管理 | 環境変数の読み込み、設定値の保持・提供 |
| `internal/models` | データ構造定義 | イベント定義、Calendarイベントなどの構造体 |
| `internal/parser` | YAMLパース | YAMLファイルの読み込み・パース |
| `internal/validator` | バリデーション | YAML形式、曜日、時刻、ドキュメントサイズのチェック |
| `internal/generator` | イベント生成 | 週次イベント定義から日次イベントへの展開 |
| `internal/calendar` | Change Calendar操作 | AWS SDKを使用したCalendar操作、API通信 |
| `internal/logger` | ログ出力 | ログレベルに応じた出力制御 |
| `internal/errors` | エラー定義 | アプリケーション固有のエラー型定義 |

### 4.2 パッケージ間の依存関係

```
main.go
  ↓
config ← parser → models
  ↓       ↓
  ↓    validator
  ↓       ↓
  ↓    generator → models
  ↓       ↓
  ↓    calendar → models
  ↓
logger

errors (すべてのパッケージから参照可能)
```

**依存の方向性:**

- `models` はどのパッケージにも依存しない(基本データ構造)
- `errors` は `models` のみに依存
- 各パッケージは `logger` を使用してログ出力

## 5. 主要な構造体とインターフェース

### 5.1 `internal/models` - データ構造

#### EventDefinition - YAMLから読み込む週次イベント定義

```go
package models

// EventDefinition はYAMLファイルから読み込む週次イベント定義
type EventDefinition struct {
    Name           string `yaml:"name"`
    StartDayOfWeek string `yaml:"start_day_of_week"`
    StartTime      string `yaml:"start_time"`
    EndDayOfWeek   string `yaml:"end_day_of_week"`
    EndTime        string `yaml:"end_time"`
}
```

**フィールド説明:**

- `Name`: イベント名
- `StartDayOfWeek`: 開始曜日 (例: "Monday", 大文字小文字区別なし)
- `StartTime`: 開始時刻 (形式: "HH:MM")
- `EndDayOfWeek`: 終了曜日
- `EndTime`: 終了時刻 (形式: "HH:MM")

#### CalendarEvent - Change Calendarに登録する実際のイベント

```go
// CalendarEvent はChange Calendarに登録する実際のイベント
type CalendarEvent struct {
    Name      string
    StartTime time.Time  // UTC時刻
    EndTime   time.Time  // UTC時刻
}
```

**フィールド説明:**

- `Name`: イベント名
- `StartTime`: イベント開始日時(UTC)
- `EndTime`: イベント終了日時(UTC)

#### 定数定義

```go
// MaxDocumentSize はSSM Documentの最大サイズ(バイト)
// 参考: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_UpdateDocument.html
const MaxDocumentSize = 64 * 1024 // 64 KB
```

**設計ノート:**

- `EventDefinition` はYAML構造をそのまま表現
- `CalendarEvent` はAWS APIに渡す形式
- `generator` パッケージが `EventDefinition` → `CalendarEvent` への変換を担当
- `MaxDocumentSize` は`generator`パッケージでiCalendar生成時のサイズバリデーションに使用

### 5.2 `internal/config` - 設定管理

```go
package config

// Config はアプリケーション全体の設定を保持
type Config struct {
    // 必須設定項目
    CalendarName string
    ConfigFile   string

    // オプション設定項目
    DryRun       bool
    LogLevel     string
    DurationDays int    // イベント登録期間(日数)
    Timezone     string // タイムゾーン (IANA形式)
}
```

**フィールド説明:**

- `CalendarName`: 対象のChange Calendar名 (必須)
- `ConfigFile`: YAMLファイルのパス (必須)
- `DryRun`: ドライランモード (デフォルト: false)
- `LogLevel`: ログ出力レベル ("silent", "normal", "verbose")、デフォルト: "normal"
- `DurationDays`: イベント登録期間(日数)、デフォルト: 365
- `Timezone`: タイムゾーン、デフォルト: "UTC"

**設定の優先順位:**

`DurationDays`と`Timezone`は以下の優先順位で設定される:
1. **コマンドライン引数** (`--calendar-duration-days`, `--calendar-timezone`)
2. **環境変数** (`SSM_CALENDAR_DURATION_DAYS`, `SSM_CALENDAR_TIMEZONE`)
3. **デフォルト値** (365, "UTC")

**関数設計:**
```go
// LoadConfig はコマンドライン引数と環境変数から設定を読み込む
// 優先順位: コマンドライン引数 > 環境変数 > デフォルト値
func LoadConfig() (*Config, error)

// Validate は設定値の妥当性を検証
func (c *Config) Validate() error
```

**設計ノート:**

- AWS関連の設定(リージョン、認証情報)はAWS SDKのデフォルト設定に任せる
- 環境変数: `AWS_REGION`, `AWS_PROFILE` など
- `Validate()`メソッドで日数範囲(1-1827)とIANA形式タイムゾーンを検証

### 5.3 `internal/calendar` - Change Calendar操作

#### 背景: Change CalendarとAWS API

Change CalendarはSystems Managerドキュメント(タイプ: `ChangeCalendar`)として管理され、iCalendar 2.0形式でイベントデータを保存する。

**API操作:**

- `UpdateDocument`: ドキュメント全体を更新(既存イベントは自動的に置き換えられる)
- `DescribeDocument`: ドキュメントの存在確認と情報取得

**参考資料:**

- [AWS Systems Manager Change Calendar](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-change-calendar.html)
- [UpdateDocument API](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_UpdateDocument.html)

#### インターフェース定義

```go
package calendar

import (
    "context"
)

// Client はChange Calendar操作のインターフェース
type Client interface {
    // CalendarExists は指定されたCalendarが存在するかを確認
    CalendarExists(ctx context.Context, name string) (bool, error)

    // UpdateCalendar はCalendarの内容を更新(iCalendar形式)
    // 既存のイベントはすべて置き換えられる
    // ドライランモードの場合は実際の更新を行わない
    UpdateCalendar(ctx context.Context, name string, iCalContent string) error
}
```

**設計ノート:**

- `UpdateDocument` APIはドキュメント全体を置き換えるため、個別のイベント削除APIは不要
- インターフェースを定義することで、テスト時にモック実装を使用可能
- `context.Context` を受け取り、タイムアウトやキャンセル処理に対応
- 実装は `calendar.go` に、モックは `mock.go` に配置

#### 実装(概要)

```go
// AWSClient はClient インターフェースの実装
type AWSClient struct {
    ssmClient *ssm.Client
    dryRun    bool
    logger    *logger.Logger
}

// NewAWSClient は新しいAWSClientを作成
func NewAWSClient(cfg aws.Config, dryRun bool, log *logger.Logger) *AWSClient

// UpdateCalendar の実装
func (c *AWSClient) UpdateCalendar(ctx context.Context, name string, iCalContent string) error {
    if c.dryRun {
        c.logger.Info("Dry run: would update calendar")
        return nil
    }
    // 実際のUpdateDocument API呼び出し
}
```

**ドライランモードの制御:**

- `AWSClient`がドライランフラグを保持
- `UpdateCalendar`内でドライラン時は実際のAPI呼び出しをスキップ
- `CalendarExists`は常に実行(ドライラン時も存在確認は必要)

### 5.4 `internal/validator` - バリデーション

```go
package validator

import "github.com/kiyo-s/update-ssm-calendar/internal/models"

// Validator はバリデーション機能を提供
type Validator struct{}

// NewValidator は新しいValidatorを作成
func NewValidator() *Validator

// ValidateConfig は設定値のバリデーションを実行
func (v *Validator) ValidateConfig(durationDays int, timezone string) error

// ValidateEventDefinitions はイベント定義のバリデーションを実行
func (v *Validator) ValidateEventDefinitions(events []models.EventDefinition) error
```

**バリデーション項目:**

**1. 設定値のバリデーション (`ValidateConfig`)**
- イベント登録期間の範囲チェック (1 ≤ days ≤ 1827)
- タイムゾーンのIANA形式チェック (例: "UTC", "Asia/Tokyo")

**2. イベント定義のバリデーション (`ValidateEventDefinitions`)**
- 必須フィールドの存在確認
- 曜日の妥当性チェック (Monday-Sunday)
- 時刻フォーマットのチェック (HH:MM形式、時:00-23、分:00-59)
- イベント期間の妥当性チェック (終了日時 > 開始日時)

**内部関数設計:**
```go
// validateDurationDays はイベント登録期間の妥当性を検証
func validateDurationDays(days int) error

// validateTimezone はタイムゾーンのIANA形式を検証
func validateTimezone(tz string) error

// validateRequiredFields は必須フィールドの存在を確認
func validateRequiredFields(event models.EventDefinition) error

// validateDayOfWeek は曜日の妥当性を確認
func validateDayOfWeek(day string) error

// validateTimeFormat は時刻フォーマットを確認
func validateTimeFormat(timeStr string) error

// validateEventPeriod はイベント期間の妥当性を確認
func validateEventPeriod(event models.EventDefinition) error
```

**設計ノート:**
- タイムゾーン検証には標準ライブラリ `time.LoadLocation()` を使用
- 曜日は大文字小文字を区別しないため、正規化してから比較
- エラーメッセージは具体的かつ人間が理解しやすい形式
- ドキュメントサイズのバリデーションは `generator` パッケージで実施

### 5.5 `internal/logger` - ログ出力

```go
package logger

import "io"

// LogLevel はログ出力レベル
type LogLevel int

const (
    Silent  LogLevel = iota  // エラー、警告、ドライランのみ
    Normal                    // 処理サマリーを含む
    Verbose                   // すべての詳細情報
)

// Logger はログ出力を管理
type Logger struct {
    level LogLevel
    out   io.Writer
}

// NewLogger は新しいLoggerを作成
func NewLogger(level LogLevel, out io.Writer) *Logger

// Error はエラーメッセージを出力 (すべてのレベル)
func (l *Logger) Error(msg string)

// Warn は警告メッセージを出力 (すべてのレベル)
func (l *Logger) Warn(msg string)

// Info は情報メッセージを出力 (Normal, Verbose)
func (l *Logger) Info(msg string)

// Debug はデバッグメッセージを出力 (Verbose のみ)
func (l *Logger) Debug(msg string)
```

**設計ノート:**

- `io.Writer` を受け取ることで、テスト時に出力先を変更可能
- 各レベルに応じて出力を制御
- フォーマット付き出力用に `Errorf`, `Infof` なども追加可能

### 5.6 `cmd/update-ssm-calendar/main.go` - エントリーポイント

**コマンドライン引数:**

```bash
update-ssm-calendar --calendar <name> --config <file> [options]
```

**必須オプション:**
- `--calendar`: Change Calendar名
- `--config`: YAMLファイルのパス

**任意オプション:**
- `--dry-run`: ドライランモード
- `--log-level`: ログレベル (silent/normal/verbose)、デフォルト: normal
- `--calendar-duration-days`: イベント登録期間(日数)、デフォルト: 365
- `--calendar-timezone`: タイムゾーン(IANA形式)、デフォルト: UTC

**処理フロー:**

```go
func main() {
    // 1. コマンドライン引数のパース
    // 2. 設定の読み込み (config.LoadConfig)
    // 3. Loggerの初期化
    // 4. 設定のバリデーション (config.Validate)
    // 5. YAMLファイルの読み込み (parser.ParseYAML)
    // 6. イベント定義のバリデーション (validator.ValidateEventDefinitions)
    // 7. イベント生成 (generator.GenerateEvents)
    // 8. AWS Clientの初期化
    // 9. Change Calendarの存在確認
    // 10. Change Calendarの更新 (ドライラン考慮)
    // 11. 結果の出力
    // エラーハンドリング: 各ステップでエラーが発生した場合、適切な終了コードで終了
}
```

**終了コード:**
- 0: 成功
- 1: 一般的なエラー
- 2: コマンドライン引数エラー
- 3: YAMLファイル読み込みエラー
- 4: バリデーションエラー
- 5: AWS認証エラー
- 6: AWS API呼び出しエラー

**コマンドライン引数パースライブラリ:**
- **候補1**: 標準ライブラリ `flag` パッケージ
  - メリット: 標準ライブラリ、依存なし、シンプル
  - デメリット: サブコマンド機能なし、ヘルプメッセージの自動生成が限定的
- **候補2**: `github.com/spf13/cobra`
  - メリット: 豊富な機能、ヘルプメッセージの自動生成、広く使われている
  - デメリット: 外部依存、このツールには過剰かもしれない

**推奨**: このツールの規模を考慮し、標準ライブラリの `flag` パッケージを推奨

**設計ノート:**
- 環境変数の読み込みは `config.LoadConfig()` 内で実施
- コマンドライン引数と環境変数の優先順位制御も `config.LoadConfig()` で実施
- エラーハンドリングは各ステップで行い、適切な終了コードを返す

## 6. テスト戦略

### 6.1 テスト対象とアプローチ

| パッケージ | テストアプローチ | モック対象 |
|-----------|----------------|-----------|
| `models` | 単体テスト | なし(データ構造のみ) |
| `config` | 単体テスト | 環境変数(テーブル駆動テスト) |
| `parser` | 単体テスト | `testdata/` のYAMLファイルを使用 |
| `validator` | 単体テスト | テーブル駆動テスト |
| `generator` | 単体テスト | 時刻計算のテスト |
| `calendar` | 単体テスト + モック | AWS SDK (モック実装を使用) |
| `logger` | 単体テスト | `io.Writer` をバッファに置き換え |
| `errors` | 単体テスト | エラーメッセージの検証 |

### 6.2 テストデータ

#### `testdata/` ディレクトリ

- 各パッケージ配下に配置(Go標準)
- 例: `internal/parser/testdata/valid.yaml`

#### モック実装

- `internal/calendar/mock.go`: AWS API呼び出しのモック
- テーブル駆動テストで複数のケースを効率的にカバー

### 6.3 カバレッジ目標

- 全体: 80%以上
- 重要なロジック(validator, generator): 90%以上

## 7. データフロー

### 7.1 メイン処理フロー

```
1. main.go
   ↓ コマンドライン引数解析

2. config
   ↓ 設定の読み込み・検証

3. parser
   ↓ YAMLファイル読み込み (EventDefinition配列)

4. validator
   ↓ YAMLデータのバリデーション実行

5. generator
   ↓ 週次定義 → 日次イベント展開 (CalendarEvent配列)
   ↓ iCalendar形式への変換
   ↓ ドキュメントサイズのバリデーション (MaxDocumentSize)

6. calendar.CalendarExists
   ↓ Change Calendarの存在確認

7. calendar.UpdateCalendar
   ↓ iCalendar形式でCalendar全体を更新
   (ドライランモードの場合はスキップ)

8. 成功/失敗の出力
```

**データ変換の流れ:**

```
YAML → EventDefinition → CalendarEvent → iCalendar文字列 → AWS API
```

### 7.2 エラーハンドリングフロー

各ステップでエラーが発生した場合:

1. 該当パッケージがエラーを返す
2. `main.go` でエラーをキャッチ
3. `logger.Error()` でエラーメッセージを出力
4. 適切な終了コードで終了

## 8. 未解決事項

以下の項目は詳細設計を進める中で決定する:

### 8.1 構造体・インターフェースの詳細

- [ ] `internal/parser` の関数シグネチャ
- [x] `internal/validator` のバリデーションルール構造 → セクション5.4で設計完了
- [ ] `internal/generator` のイベント生成ロジック
- [ ] `internal/errors` のエラー型定義

### 8.2 AWS関連

- [x] Change Calendarドキュメントサイズの制限値 → 64KB (UpdateDocument APIの制限)
- [ ] AWS SDK v2の具体的なAPI呼び出し方法
- [ ] 必要なIAM権限の詳細

### 8.3 その他

- [x] コマンドライン引数のパースライブラリ選定 → 標準ライブラリ `flag` を採用 (セクション5.6)
- [ ] タイムゾーン変換の実装詳細
- [ ] 曜日パースのロジック

## 9. 次のステップ

### 9.1 完了した項目

- [x] アーキテクチャスタイルの決定 (パッケージベース)
- [x] ディレクトリ構成の決定
- [x] 主要な構造体とインターフェースの設計
  - [x] `models`: EventDefinition, CalendarEvent, MaxDocumentSize
  - [x] `config`: Config、設定の優先順位ロジック、LoadConfig/Validate関数
  - [x] `calendar`: Client インターフェース、ドライランモード制御
  - [x] `validator`: Validator、バリデーション項目、内部関数設計
  - [x] `logger`: Logger, LogLevel
  - [x] `cmd/update-ssm-calendar/main.go`: コマンドライン引数、処理フロー、終了コード
- [x] AWS API仕様の調査と設計への反映
- [x] 要件定義書の更新反映 (コマンドライン引数追加、バリデーション要件追加)

### 9.2 次回の進め方(候補)

以下のいずれかから作業を開始:

#### 案1: 残りのパッケージ詳細設計

- [ ] `internal/parser` の設計
  - YAMLファイル読み込みの関数シグネチャ
  - エラーハンドリング
- [ ] `internal/validator` の設計
  - バリデーションルールの構造
  - 各バリデーション関数の定義
- [ ] `internal/generator` の設計
  - イベント生成ロジック (週次→日次)
  - iCalendar形式への変換
  - タイムゾーン処理
  - 曜日パースロジック
- [ ] `internal/errors` の設計
  - エラー型の定義
  - エラーメッセージの標準化

#### 案2: プロトタイプ実装計画

- [ ] 実装の優先順位決定
  - コアロジック優先 vs エンドツーエンド優先
- [ ] マイルストーン設定
  - Phase 1: 基本機能
  - Phase 2: エラーハンドリング
  - Phase 3: テスト
- [ ] 技術的な調査項目
  - コマンドライン引数パースライブラリ (cobra, flag, etc.)
  - iCalendarライブラリ (既存 or 自作)
  - AWS SDK v2の使用方法

#### 案3: 実装開始

- [ ] `models`パッケージから実装 (依存なし)
- [ ] `logger`パッケージの実装
- [ ] `config`パッケージの実装
- [ ] 順次他のパッケージを実装

### 9.3 推奨アプローチ

**案1 (残りのパッケージ詳細設計) を推奨**

理由:

- 特に`generator`パッケージは複雑なロジック (週次→日次変換、iCalendar生成) を含む
- 設計を先に固めることで、実装時の手戻りを削減
- `validator`と`generator`のバリデーション責務を明確にする必要がある
