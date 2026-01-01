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
// iCalendar形式への変換はgeneratorパッケージが担当する
type CalendarEvent struct {
    // UID はイベントの一意識別子 (iCalendar: UID)
    UID string

    // Summary はイベント名 (iCalendar: SUMMARY)
    Summary string

    // StartTime はイベント開始日時 (iCalendar: DTSTART)
    StartTime time.Time

    // EndTime はイベント終了日時 (iCalendar: DTEND)
    EndTime time.Time

    // Timezone はIANA形式のタイムゾーン (iCalendar: TZID parameter)
    // 例: "UTC", "Asia/Tokyo"
    Timezone string
}
```

**フィールド説明:**

- `UID`: イベントの一意識別子。UUIDなどを使用
- `Summary`: イベント名・簡潔な説明
- `StartTime`: イベント開始日時。Go標準の`time.Time`型で保持
- `EndTime`: イベント終了日時。`StartTime`より後でなければならない
- `Timezone`: IANA形式のタイムゾーン(例: "Asia/Tokyo", "UTC")

**iCalendarプロパティとの対応:**

| Goフィールド | iCalendarプロパティ | 備考 |
|-------------|-------------------|------|
| `UID` | `UID` | 必須。イベントの一意識別子 |
| `Summary` | `SUMMARY` | イベント名 |
| `StartTime` | `DTSTART` | DATE-TIME形式に変換 |
| `EndTime` | `DTEND` | DATE-TIME形式に変換 |
| `Timezone` | `TZID` parameter | DTSTARTとDTENDのパラメータとして使用 |

**自動生成されるプロパティ:**

`generator`パッケージがiCalendar生成時に自動的に追加:
- `DTSTAMP`: イベント作成タイムスタンプ(現在時刻のUTCを使用)

#### 定数定義

```go
// MaxDocumentSize はSSM Documentの最大サイズ(バイト)
// 参考: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_UpdateDocument.html
const MaxDocumentSize = 64 * 1024 // 64 KB
```

**設計ノート:**

- `EventDefinition` はYAML構造をそのまま表現(週次イベント定義)
- `CalendarEvent` はGoのドメインモデル(日次イベント)
- `generator` パッケージが以下の変換を担当:
  - `EventDefinition` → `CalendarEvent` (週次→日次への展開)
  - `CalendarEvent` → iCalendar形式文字列 (AWS APIへの渡し形式)
- `MaxDocumentSize` は`generator`パッケージでiCalendar生成時のサイズバリデーションに使用
- `CalendarEvent`のフィールド名はGoの可読性を優先し、iCalendarプロパティ名との対応はコメントで明記
- iCalendar DATE-TIME形式: `YYYYMMDDTHHmmss` (例: `20240812T090000`)

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

### 5.7 `internal/generator` - イベント生成とiCalendar変換

#### 責務

`generator`パッケージは以下の2つの主要な変換を担当:

1. **週次イベント定義 → 日次イベントへの展開**
   - `EventDefinition` (YAMLから読み込んだ週次定義) を指定日数分の `CalendarEvent` に展開
   - 曜日計算、日時計算、タイムゾーン処理

2. **CalendarEvent配列 → iCalendar形式文字列への変換**
   - `CalendarEvent` 配列をiCalendar 2.0 (RFC 5545) 準拠の文字列に変換
   - VCALENDARとVEVENTの生成
   - ドキュメントサイズのバリデーション

#### iCalendarライブラリの選定

**決定: 自作実装を採用**

**選定理由:**

| 観点 | 評価 |
|------|------|
| 要件の範囲 | VEVENTの生成のみで十分。RRULE、VALARM等は不要 |
| 外部依存 | 不要。標準ライブラリのみで実装可能 |
| 実装コスト | 初期実装4時間 + テスト3時間 = 1日以内で完了可能 |
| 保守性 | 要件が明確で限定的なため、将来の変更も予測可能 |

**検討した代替案:**

1. **arran4/golang-ical** (Apache-2.0)
   - メリット: 使いやすいAPI、活発なメンテナンス
   - デメリット: 外部依存、不要な機能を含む

2. **emersion/go-ical** (MIT)
   - メリット: RFC準拠の詳細な実装
   - デメリット: APIが冗長、外部依存

**自作実装の注意点:**

実装時に対応が必要な項目:

| 項目 | 対応方法 | 難易度 |
|------|---------|--------|
| 改行コード | CRLF (`\r\n`) に統一 | ★☆☆☆☆ |
| 日時フォーマット | `time.Format("20060102T150405")` | ★★☆☆☆ |
| テキストエスケープ | `;`, `,`, `\`, 改行をエスケープ | ★★☆☆☆ |
| 行の折り返し | 75オクテット超過時に折り返し | ★★★☆☆ |
| タイムゾーン検証 | `time.LoadLocation()` で検証 | ★★★☆☆ |

**品質保証:**

- AWS Change Calendarでの実動作確認を早期に実施
- [iCalendar.org Validator](https://icalendar.org/validator.html)での検証
- 特殊文字を含むテストケースの作成

#### 構造体設計

```go
package generator

import (
    "time"
    "github.com/kiyo-s/update-ssm-calendar/internal/models"
)

// Generator はイベント生成とiCalendar変換を担当
type Generator struct {
    timezone     string
    durationDays int
}

// NewGenerator は新しいGeneratorを作成
func NewGenerator(timezone string, durationDays int) *Generator

// GenerateEvents は週次イベント定義から日次イベントを生成
func (g *Generator) GenerateEvents(
    definitions []models.EventDefinition,
) ([]models.CalendarEvent, error)

// GenerateICalendar はCalendarEvent配列からiCalendar形式文字列を生成
func (g *Generator) GenerateICalendar(
    events []models.CalendarEvent,
    calendarType string,
) (string, error)
```

**設計ノート:**

- iCalendar生成は標準ライブラリの `strings.Builder` を使用
- エスケープ処理、行折り返しは内部関数として実装
- DTSTAMP(イベント作成タイムスタンプ)は自動生成
- SEQUENCE プロパティは不要(AWS Change Calendarでは使用しない)
- ドキュメントサイズは `models.MaxDocumentSize` (64KB) でバリデーション

#### 週次→日次変換ロジックの詳細

**1. 曜日計算アルゴリズム**

`GenerateEvents` 関数の内部処理フロー:

```go
func (g *Generator) GenerateEvents(
    definitions []models.EventDefinition,
) ([]models.CalendarEvent, error) {
    var events []models.CalendarEvent

    // 現在時刻をタイムゾーンに合わせて取得
    loc, err := time.LoadLocation(g.timezone)
    if err != nil {
        return nil, err
    }
    startDate := time.Now().In(loc)
    endDate := startDate.AddDate(0, 0, g.durationDays)

    // 各イベント定義について処理
    for _, def := range definitions {
        // startDate から endDate まで1日ずつ繰り返し
        for currentDate := startDate; currentDate.Before(endDate); currentDate = currentDate.AddDate(0, 0, 1) {
            // 曜日が一致する場合のみイベント生成
            if matchesDayOfWeek(currentDate, def.StartDayOfWeek) {
                event, err := createCalendarEvent(def, currentDate, g.timezone)
                if err != nil {
                    return nil, err
                }
                events = append(events, event)
            }
        }
    }

    return events, nil
}
```

**内部関数:**

```go
// matchesDayOfWeek は指定された日付が曜日に一致するかを判定
func matchesDayOfWeek(date time.Time, dayOfWeek string) bool {
    // 大文字小文字を区別せずに比較
    weekday := date.Weekday().String()
    return strings.EqualFold(weekday, dayOfWeek)
}

// createCalendarEvent はEventDefinitionとdateからCalendarEventを生成
func createCalendarEvent(
    def models.EventDefinition,
    date time.Time,
    timezone string,
) (models.CalendarEvent, error) {
    // 開始日時を計算
    startTime, err := combineDateTime(date, def.StartTime, timezone)
    if err != nil {
        return models.CalendarEvent{}, err
    }

    // 終了日時を計算（複数日にまたがる場合を考慮）
    endDate := date
    if needsDateAdjustment(def.StartDayOfWeek, def.EndDayOfWeek) {
        endDate = calculateEndDate(date, def.StartDayOfWeek, def.EndDayOfWeek)
    }
    endTime, err := combineDateTime(endDate, def.EndTime, timezone)
    if err != nil {
        return models.CalendarEvent{}, err
    }

    // UID を生成（決定論的）
    uid := generateUID(def.Name, date)

    return models.CalendarEvent{
        UID:       uid,
        Summary:   def.Name,
        StartTime: startTime,
        EndTime:   endTime,
        Timezone:  timezone,
    }, nil
}
```

**2. 複数日にまたがるイベントの処理**

金曜日18:00〜月曜日09:00のような週をまたぐイベントに対応:

```go
// needsDateAdjustment は終了日が開始日と異なる曜日かを判定
func needsDateAdjustment(startDayOfWeek, endDayOfWeek string) bool {
    return !strings.EqualFold(startDayOfWeek, endDayOfWeek)
}

// calculateEndDate は終了日を計算（曜日の差分を考慮）
func calculateEndDate(startDate time.Time, startDayOfWeek, endDayOfWeek string) time.Time {
    startWeekday := parseWeekday(startDayOfWeek)
    endWeekday := parseWeekday(endDayOfWeek)

    // 曜日の差分を計算
    diff := int(endWeekday) - int(startWeekday)
    if diff < 0 {
        // 週をまたぐ場合 (例: Friday -> Monday)
        diff += 7
    }

    return startDate.AddDate(0, 0, diff)
}

// parseWeekday は曜日文字列をtime.Weekdayに変換
func parseWeekday(dayOfWeek string) time.Weekday {
    weekdays := map[string]time.Weekday{
        "sunday":    time.Sunday,
        "monday":    time.Monday,
        "tuesday":   time.Tuesday,
        "wednesday": time.Wednesday,
        "thursday":  time.Thursday,
        "friday":    time.Friday,
        "saturday":  time.Saturday,
    }
    return weekdays[strings.ToLower(dayOfWeek)]
}
```

**3. タイムゾーン処理**

日付と時刻を結合し、指定されたタイムゾーンの`time.Time`を生成:

```go
// combineDateTime は日付と時刻文字列を結合してtime.Timeを生成
func combineDateTime(date time.Time, timeStr string, timezone string) (time.Time, error) {
    // タイムゾーンをロード
    loc, err := time.LoadLocation(timezone)
    if err != nil {
        return time.Time{}, fmt.Errorf("invalid timezone %s: %w", timezone, err)
    }

    // 時刻文字列をパース (HH:MM形式)
    parts := strings.Split(timeStr, ":")
    if len(parts) != 2 {
        return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
    }

    hour, err := strconv.Atoi(parts[0])
    if err != nil {
        return time.Time{}, fmt.Errorf("invalid hour: %s", parts[0])
    }

    minute, err := strconv.Atoi(parts[1])
    if err != nil {
        return time.Time{}, fmt.Errorf("invalid minute: %s", parts[1])
    }

    // 日付と時刻を結合
    combined := time.Date(
        date.Year(), date.Month(), date.Day(),
        hour, minute, 0, 0,
        loc,
    )

    return combined, nil
}
```

**4. UID生成方法**

**決定: 決定論的UIDを採用**

UUID v4 ではなく、イベント名と日付をもとにした決定論的な UID を生成します。

**採用理由:**

| 観点 | 決定論的UID | UUID v4 |
|------|------------|---------|
| 衝突の可能性 | ゼロ（入力が一意であれば必ず一意） | 実質ゼロだが理論的には存在 |
| 外部依存 | 不要 (crypto/sha256 は標準ライブラリ) | github.com/google/uuid が必要 |
| べき等性 | 同じ入力→同じUID（再実行時も同一） | 毎回異なるUIDが生成される |
| テスト容易性 | 高い（UIDを予測可能） | 低い（ランダムなため検証困難） |

**実装:**

```go
import "crypto/sha256"

// generateUID はイベント名と日付から決定論的なUIDを生成
// SHA-256ハッシュを用いることで、同じ入力に対して常に同じUIDを生成
func generateUID(eventName string, date time.Time) string {
    // 入力文字列を作成 (イベント名:日付)
    input := fmt.Sprintf("%s:%s", eventName, date.Format("2006-01-02"))

    // SHA-256ハッシュを計算
    hash := sha256.Sum256([]byte(input))

    // UUID形式のような文字列に整形 (8-4-4-4-12 の形式)
    return fmt.Sprintf("%x-%x-%x-%x-%x",
        hash[0:4],   // 8文字
        hash[4:6],   // 4文字
        hash[6:8],   // 4文字
        hash[8:10],  // 4文字
        hash[10:16], // 12文字
    )
}
```

**生成例:**

```
入力: eventName="Monday Event", date=2025-10-13
出力: "a1b2c3d4-e5f6-7890-abcd-ef0123456789"

入力: eventName="Monday Event", date=2025-10-20
出力: "f9e8d7c6-b5a4-3210-9876-543210fedcba"
```

**利点:**

1. **衝突保証**: イベント名と日付の組み合わせは必ず一意なので、UID衝突はゼロ
2. **べき等性**: 同じ定義を再実行しても同じUIDが生成され、Change Calendarの不要な更新を削減
3. **依存削減**: 標準ライブラリのみで実装可能
4. **テスト容易**: 期待されるUIDを事前に計算できる

#### iCalendar形式変換ロジックの詳細

`GenerateICalendar` 関数は、`CalendarEvent` 配列から iCalendar 2.0 (RFC 5545) 準拠の文字列を生成します。

**1. VCALENDAR と VEVENT の構造**

iCalendar 形式の基本構造:

```icalendar
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//kiyo-s//update-ssm-calendar//EN
CALSCALE:GREGORIAN
METHOD:PUBLISH
BEGIN:VEVENT
UID:a1b2c3d4-e5f6-7890-abcd-ef0123456789
DTSTAMP:20251001T120000Z
DTSTART;TZID=Asia/Tokyo:20251013T080000
DTEND;TZID=Asia/Tokyo:20251013T220000
SUMMARY:Monday Event
END:VEVENT
BEGIN:VEVENT
...
END:VEVENT
END:VCALENDAR
```

**GenerateICalendar 関数の実装:**

```go
func (g *Generator) GenerateICalendar(
    events []models.CalendarEvent,
    calendarType string,
) (string, error) {
    var builder strings.Builder

    // VCALENDAR ヘッダー
    builder.WriteString("BEGIN:VCALENDAR\r\n")
    builder.WriteString("VERSION:2.0\r\n")
    builder.WriteString("PRODID:-//kiyo-s//update-ssm-calendar//EN\r\n")
    builder.WriteString("CALSCALE:GREGORIAN\r\n")
    builder.WriteString("METHOD:PUBLISH\r\n")

    // 各イベントを VEVENT として追加
    for _, event := range events {
        vevent, err := generateVEvent(event)
        if err != nil {
            return "", err
        }
        builder.WriteString(vevent)
    }

    // VCALENDAR フッター
    builder.WriteString("END:VCALENDAR\r\n")

    // ドキュメントサイズのバリデーション
    content := builder.String()
    if len(content) > models.MaxDocumentSize {
        return "", fmt.Errorf(
            "document size %d bytes exceeds limit %d bytes",
            len(content),
            models.MaxDocumentSize,
        )
    }

    return content, nil
}
```

**VEVENT 生成関数:**

```go
func generateVEvent(event models.CalendarEvent) (string, error) {
    var builder strings.Builder

    builder.WriteString("BEGIN:VEVENT\r\n")

    // UID
    builder.WriteString(fmt.Sprintf("UID:%s\r\n", event.UID))

    // DTSTAMP (現在時刻のUTC)
    dtstamp := time.Now().UTC().Format("20060102T150405Z")
    builder.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))

    // DTSTART (タイムゾーン付き)
    dtstart := formatDateTime(event.StartTime, event.Timezone)
    builder.WriteString(fmt.Sprintf("DTSTART;TZID=%s:%s\r\n", event.Timezone, dtstart))

    // DTEND (タイムゾーン付き)
    dtend := formatDateTime(event.EndTime, event.Timezone)
    builder.WriteString(fmt.Sprintf("DTEND;TZID=%s:%s\r\n", event.Timezone, dtend))

    // SUMMARY (エスケープ処理)
    summary := escapeText(event.Summary)
    summaryLine := fmt.Sprintf("SUMMARY:%s\r\n", summary)
    // 行の折り返し処理
    foldedSummary := foldLine(summaryLine)
    builder.WriteString(foldedSummary)

    builder.WriteString("END:VEVENT\r\n")

    return builder.String(), nil
}
```

**2. 日時フォーマット変換**

`time.Time` から iCalendar の DATE-TIME 形式 (`YYYYMMDDTHHmmss`) に変換:

```go
// formatDateTime はtime.TimeをiCalendar DATE-TIME形式に変換
func formatDateTime(t time.Time, timezone string) string {
    // タイムゾーンに合わせて変換
    loc, err := time.LoadLocation(timezone)
    if err != nil {
        // エラー時はそのまま使用（バリデーション済みのため到達しない想定）
        loc = t.Location()
    }

    localTime := t.In(loc)
    return localTime.Format("20060102T150405")
}
```

**フォーマット例:**

```
入力: time.Time{2025, 10, 13, 8, 0, 0, 0, Asia/Tokyo}
出力: "20251013T080000"

入力: time.Time{2025, 12, 31, 23, 59, 0, 0, UTC}
出力: "20251231T235900"
```

**3. テキストエスケープ処理**

iCalendar では以下の文字をエスケープする必要があります:

| 文字 | エスケープ方法 |
|------|--------------|
| `;` (セミコロン) | `\;` |
| `,` (カンマ) | `\,` |
| `\` (バックスラッシュ) | `\\` |
| 改行 (`\n`) | `\n` (そのまま) |

```go
// escapeText はiCalendar TEXT型の値をエスケープ
func escapeText(text string) string {
    // バックスラッシュを最初にエスケープ（他のエスケープと干渉しないため）
    text = strings.ReplaceAll(text, "\\", "\\\\")
    // セミコロンをエスケープ
    text = strings.ReplaceAll(text, ";", "\\;")
    // カンマをエスケープ
    text = strings.ReplaceAll(text, ",", "\\,")
    // 改行はそのまま（iCalendarでは \n として扱われる）
    return text
}
```

**エスケープ例:**

```
入力: "Meeting; Tokyo, Japan"
出力: "Meeting\\; Tokyo\\, Japan"

入力: "Path: C:\\Users\\file.txt"
出力: "Path: C:\\\\Users\\\\file.txt"
```

**4. 行の折り返し処理**

RFC 5545 では、1行は75オクテット（バイト）を超えてはならず、超過する場合は次の行の先頭にスペースを入れて折り返します。

```go
// foldLine は75オクテットを超える行を折り返す
func foldLine(line string) string {
    const maxOctets = 75

    // 改行で終わる場合は除去してから処理
    line = strings.TrimSuffix(line, "\r\n")

    // 75オクテット以下ならそのまま返す
    if len(line) <= maxOctets {
        return line + "\r\n"
    }

    var builder strings.Builder
    remaining := line

    for len(remaining) > 0 {
        if len(remaining) <= maxOctets {
            // 残りがすべて75オクテット以下
            builder.WriteString(remaining)
            builder.WriteString("\r\n")
            break
        }

        // 75オクテット分を切り出し
        chunk := remaining[:maxOctets]
        builder.WriteString(chunk)
        builder.WriteString("\r\n ")  // 改行 + スペース

        // 次の行（残り）
        remaining = remaining[maxOctets:]
    }

    return builder.String()
}
```

**折り返し例:**

```
入力 (80文字):
"SUMMARY:This is a very long summary that exceeds the 75 octet limit for iCalendar"

出力 (折り返し後):
"SUMMARY:This is a very long summary that exceeds the 75 octet limit for i\r\n Calendar"
       ^75文字目で改行+スペース
```

**5. ドキュメントサイズのバリデーション**

AWS Systems Manager の UpdateDocument API は最大 64KB (65,536 bytes) のドキュメントを受け付けます。

```go
// GenerateICalendar 内でのバリデーション
content := builder.String()
if len(content) > models.MaxDocumentSize {
    return "", fmt.Errorf(
        "document size %d bytes exceeds limit %d bytes (remove %d events or reduce event names)",
        len(content),
        models.MaxDocumentSize,
        estimateEventsToRemove(len(content), models.MaxDocumentSize, len(events)),
    )
}
```

**推定削除イベント数の計算:**

```go
// estimateEventsToRemove は超過サイズから削除すべきイベント数を推定
func estimateEventsToRemove(currentSize, maxSize, eventCount int) int {
    if currentSize <= maxSize {
        return 0
    }

    excessSize := currentSize - maxSize
    // 1イベントあたりの平均サイズを計算
    avgEventSize := currentSize / eventCount
    // 必要な削除イベント数（+1は安全マージン）
    return (excessSize / avgEventSize) + 1
}
```

**設計上の考慮事項:**

| 項目 | 対応 |
|------|------|
| 改行コード | すべて CRLF (`\r\n`) に統一 |
| 文字エンコーディング | UTF-8 (Go標準) |
| タイムゾーン表現 | TZID パラメータを使用 (例: `DTSTART;TZID=Asia/Tokyo:...`) |
| プロパティの順序 | UID → DTSTAMP → DTSTART → DTEND → SUMMARY の順 |
| 必須プロパティ | UID, DTSTAMP, DTSTART のみ（DTEND と SUMMARY は任意だが常に出力） |

**テスト項目:**

1. **基本的なイベント生成**: 単純なイベントが正しく生成されるか
2. **特殊文字のエスケープ**: `;`, `,`, `\` を含むイベント名が正しくエスケープされるか
3. **長いイベント名の折り返し**: 75オクテット超のイベント名が正しく折り返されるか
4. **ドキュメントサイズ超過**: 64KBを超える場合にエラーが返されるか
5. **タイムゾーン処理**: 異なるタイムゾーンで正しくフォーマットされるか
6. **iCalendar バリデーター**: 生成された文字列が https://icalendar.org/validator.html で検証可能か

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
- [x] `internal/generator` のイベント生成ロジック → セクション5.7で詳細設計完了（週次→日次変換、iCalendar変換、決定論的UID）
- [ ] `internal/errors` のエラー型定義

### 8.2 AWS関連

- [x] Change Calendarドキュメントサイズの制限値 → 64KB (UpdateDocument APIの制限)
- [ ] AWS SDK v2の具体的なAPI呼び出し方法
- [ ] 必要なIAM権限の詳細

### 8.3 その他

- [x] コマンドライン引数のパースライブラリ選定 → 標準ライブラリ `flag` を採用 (セクション5.6)
- [x] タイムゾーン変換の実装詳細 → セクション5.7で設計完了 (combineDateTime, formatDateTime)
- [x] 曜日パースのロジック → セクション5.7で設計完了 (matchesDayOfWeek, parseWeekday)

## 9. 次のステップ

### 9.1 完了した項目

- [x] アーキテクチャスタイルの決定 (パッケージベース)
- [x] ディレクトリ構成の決定
- [x] 主要な構造体とインターフェースの設計
  - [x] `models`: EventDefinition, CalendarEvent (iCalendar仕様に基づく設計), MaxDocumentSize
  - [x] `config`: Config、設定の優先順位ロジック、LoadConfig/Validate関数
  - [x] `calendar`: Client インターフェース、ドライランモード制御
  - [x] `validator`: Validator、バリデーション項目、内部関数設計
  - [x] `logger`: Logger, LogLevel
  - [x] `generator`: Generator構造体、責務定義、iCalendar自作の決定、**詳細設計完了**
  - [x] `cmd/update-ssm-calendar/main.go`: コマンドライン引数、処理フロー、終了コード
- [x] AWS API仕様の調査と設計への反映
- [x] iCalendar 2.0 (RFC 5545)仕様の調査とCalendarEvent設計への反映
- [x] iCalendarライブラリの選定 → 自作実装を採用(セクション5.7)
- [x] 要件定義書の更新反映 (コマンドライン引数追加、バリデーション要件追加)
- [x] `internal/generator` パッケージの詳細設計
  - [x] 週次→日次変換ロジック (曜日計算、複数日にまたがるイベント処理)
  - [x] タイムゾーン処理 (combineDateTime, formatDateTime)
  - [x] 決定論的UID生成 (SHA-256ベース)
  - [x] iCalendar形式変換ロジック (VCALENDAR/VEVENT生成、エスケープ、行折り返し、サイズバリデーション)

### 9.2 次回の進め方(候補)

以下のいずれかから作業を開始:

#### 案1: 残りのパッケージ詳細設計

- [ ] `internal/parser` の設計
  - YAMLファイル読み込みの関数シグネチャ
  - エラーハンドリング
- [x] `internal/validator` の設計 → セクション5.4で完了
  - バリデーションルールの構造
  - 各バリデーション関数の定義
- [x] `internal/generator` の設計 → セクション5.7で完了
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
  - ~~iCalendarライブラリ (既存 or 自作)~~ → **決定: 自作 (詳細はセクション5.7参照)**
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
