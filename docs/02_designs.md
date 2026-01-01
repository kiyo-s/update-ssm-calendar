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
    // コマンドライン引数から設定される項目
    CalendarName string
    ConfigFile   string
    DryRun       bool
    LogLevel     string

    // 環境変数から設定される項目
    DurationDays int    // SSM_CALENDAR_DURATION_DAYS
    Timezone     string // SSM_CALENDAR_TIMEZONE (IANA形式)
}
```

**フィールド説明:**

- `CalendarName`: 対象のChange Calendar名
- `ConfigFile`: YAMLファイルのパス
- `DryRun`: ドライランモード (true: 実際の変更を行わない)
- `LogLevel`: ログ出力レベル ("silent", "normal", "verbose")
- `DurationDays`: イベント登録期間(日数)、デフォルト: 365
- `Timezone`: タイムゾーン、デフォルト: "UTC"

**設計ノート:**

- AWS関連の設定(リージョン、認証情報)はAWS SDKのデフォルト設定に任せる
- 環境変数: `AWS_REGION`, `AWS_PROFILE` など

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

### 5.4 `internal/logger` - ログ出力

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
- [ ] `internal/validator` のバリデーションルール構造
- [ ] `internal/generator` のイベント生成ロジック
- [ ] `internal/errors` のエラー型定義

### 8.2 AWS関連

- [x] Change Calendarドキュメントサイズの制限値 → 64KB (UpdateDocument APIの制限)
- [ ] AWS SDK v2の具体的なAPI呼び出し方法
- [ ] 必要なIAM権限の詳細

### 8.3 その他

- [ ] コマンドライン引数のパースライブラリ選定 (cobra, flag, etc.)
- [ ] タイムゾーン変換の実装詳細
- [ ] 曜日パースのロジック

## 9. 次のステップ

### 9.1 完了した項目

- [x] アーキテクチャスタイルの決定 (パッケージベース)
- [x] ディレクトリ構成の決定
- [x] 主要な構造体とインターフェースの設計
  - [x] `models`: EventDefinition, CalendarEvent, MaxDocumentSize
  - [x] `config`: Config
  - [x] `calendar`: Client インターフェース、ドライランモード制御
  - [x] `logger`: Logger, LogLevel
- [x] AWS API仕様の調査と設計への反映

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
