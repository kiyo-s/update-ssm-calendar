# 要件定義書

## 1. 概要

### 1.1 目的
開発・テスト環境のサーバーを稼働させる時間を管理するため、YAMLファイルの定義に従ってAWS Systems Manager Change Calendarのイベントを登録するコマンドラインツールを開発する。

### 1.2 対象ユーザー
- 開発担当者
- 運用担当者

## 2. 機能要件

### 2.1 基本機能

#### 2.1.1 イベント登録
- YAMLファイルで定義された週次イベントを、指定された期間分、Change Calendarに登録する
- 一週間単位の設定をひとかたまりとして、指定された日数分のイベントを生成・登録する
- 実行日を起点として、指定日数分のイベントを登録する(デフォルト: 365日)

#### 2.1.2 既存イベントの扱い
- Change Calendarに既存のイベントが登録されている場合、**すべて削除してから新規作成**する

#### 2.1.3 ドライラン機能
- `--dry-run` オプションにより、実際にはAWSへの変更を行わず、実行内容の確認のみを行う

### 2.2 入力仕様

#### 2.2.1 YAMLファイル形式
イベント定義を配列形式で記述する。各イベントは以下の項目を持つ:

| No. | key | 説明 | 例 | 必須 |
|-----|-----|------|-----|------|
| 1 | name | イベントの名前 | `Monday Event` | ✓ |
| 2 | start_day_of_week | イベントを開始する曜日 | `Monday` | ✓ |
| 3 | start_time | イベントを開始する時間 (HH:MM) | `08:00` | ✓ |
| 4 | end_day_of_week | イベントを終了する曜日 | `Monday` | ✓ |
| 5 | end_time | イベントを終了する時間 (HH:MM) | `22:00` | ✓ |

**曜日の指定:**
- 英語表記: `Monday`, `Tuesday`, `Wednesday`, `Thursday`, `Friday`, `Saturday`, `Sunday`
- 大文字小文字は区別しない(`monday`, `MONDAY`, `Monday` すべて有効)

**時刻の指定:**
- 24時間形式: `HH:MM` (例: `08:00`, `22:30`)
- YAMLファイル内の時刻は、環境変数で指定されたタイムゾーンとして解釈される

**YAMLファイル例:**
```yaml
- name: "Monday Event"
  start_day_of_week: "Monday"
  start_time: "08:00"
  end_day_of_week: "Monday"
  end_time: "22:00"
- name: "Tuesday Event"
  start_day_of_week: "Tuesday"
  start_time: "08:00"
  end_day_of_week: "Tuesday"
  end_time: "22:00"
```

#### 2.2.2 コマンドライン引数

```bash
update-ssm-calendar --calendar <calendar-name> --config <config-file> [options]
```

**必須オプション:**
- `--calendar`: Change Calendar名
- `--config`: YAMLファイルのパス

**任意オプション:**
- `--dry-run`: ドライランモード(実際の変更は行わない)
- `--log-level`: ログ出力レベル (`silent`, `normal`, `verbose`) デフォルト: `normal`

#### 2.2.3 環境変数

| 環境変数名 | 説明 | デフォルト値 | 例 |
|-----------|------|-------------|-----|
| `SSM_CALENDAR_DURATION_DAYS` | イベント登録期間(日数) | `365` | `365` |
| `SSM_CALENDAR_TIMEZONE` | タイムゾーン(IANA形式) | `UTC` | `Asia/Tokyo` |

**AWS認証情報:**
- AWS SDKの標準的な認証情報取得方法に従う
  - 環境変数: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
  - 認証情報ファイル: `~/.aws/credentials`
  - IAMロール など

### 2.3 バリデーション

#### 2.3.1 YAMLファイルのバリデーション
以下の項目をAWS変更前にチェックし、不正があれば異常終了する:

1. **必須フィールドの存在確認**
   - `name`, `start_day_of_week`, `start_time`, `end_day_of_week`, `end_time` がすべて存在するか

2. **曜日の妥当性チェック**
   - 指定された曜日が有効な値(`Monday`〜`Sunday`)であるか

3. **時刻フォーマットのチェック**
   - `HH:MM` 形式であるか
   - 時は `00`〜`23`、分は `00`〜`59` の範囲内であるか

4. **イベント期間の妥当性チェック**
   - 終了日時が開始日時より後であるか(同一曜日の場合は終了時刻が開始時刻より後)
   - 複数日にまたがるイベントの場合、論理的に正しいか

5. **Change Calendarドキュメントサイズの制限チェック**
   - 生成されるイベントの総サイズがChange Calendarの制限(詳細は実装時に調査)を超えないか

#### 2.3.2 AWS関連のバリデーション
以下の項目をチェックし、問題があれば異常終了する:

1. **AWS認証情報の確認**
   - 有効な認証情報が設定されているか

2. **Change Calendarの存在確認**
   - 指定されたChange Calendarが存在するか
   - ※将来的にはCalendar作成機能を追加する可能性がある

### 2.4 出力・ログ

#### 2.4.1 ログレベル

| レベル | 説明 | 出力内容 |
|--------|------|----------|
| `silent` | 最小限の出力 | エラー、警告、ドライランモード表示のみ |
| `normal` | 標準出力(デフォルト) | エラー、警告、ドライランモード、処理サマリー |
| `verbose` | 詳細出力 | すべての情報(個別イベント登録状況、API呼び出し詳細など) |

#### 2.4.2 出力情報マトリックス

| 情報の種類 | silent | normal | verbose |
|-----------|--------|--------|---------|
| エラーメッセージ | ✓ | ✓ | ✓ |
| 警告メッセージ | ✓ | ✓ | ✓ |
| ドライランモード表示 | ✓ | ✓ | ✓ |
| YAMLファイル読み込み件数 | | ✓ | ✓ |
| バリデーション結果サマリー | | ✓ | ✓ |
| 既存イベント削除件数 | | ✓ | ✓ |
| 新規イベント登録件数 | | ✓ | ✓ |
| 最終的な成功/失敗 | | ✓ | ✓ |
| バリデーション詳細 | | | ✓ |
| 個別イベントの登録状況 | | | ✓ |
| AWS API呼び出しの詳細 | | | ✓ |
| Calendar情報(リージョンなど) | | | ✓ |

#### 2.4.3 終了コード

| 終了コード | 説明 |
|-----------|------|
| 0 | 成功 |
| 1 | 一般的なエラー |
| 2 | コマンドライン引数エラー |
| 3 | YAMLファイル読み込みエラー |
| 4 | バリデーションエラー |
| 5 | AWS認証エラー |
| 6 | AWS API呼び出しエラー |

### 2.5 エラーハンドリング

#### 2.5.1 エラー発生時の動作
- すべてのエラーは標準エラー出力に出力する
- エラーメッセージは人間が理解しやすい形式とする
- 適切な終了コードを返す

#### 2.5.2 エラーシナリオ

| シナリオ | 動作 | 終了コード |
|---------|------|-----------|
| YAMLファイルが存在しない | エラーメッセージを出力して終了 | 3 |
| YAMLファイルの形式が不正 | エラーメッセージを出力して終了 | 3 |
| バリデーションエラー | エラー内容を出力して終了 | 4 |
| AWS認証情報が未設定 | エラーメッセージを出力して終了 | 5 |
| 指定されたCalendarが存在しない | エラーメッセージを出力して終了 | 6 |
| AWS API呼び出しエラー | エラー内容を出力して終了 | 6 |
| ドキュメントサイズ超過 | エラーメッセージを出力して終了 | 4 |

## 3. 非機能要件

### 3.1 パフォーマンス
- 処理時間は十分短い想定のため、進捗表示は不要

### 3.2 拡張性
- 将来的にChange Calendarの作成機能を追加する可能性を考慮した設計とする

### 3.3 開発言語
- Go言語を使用

### 3.4 依存関係
- AWS SDK for Go v2を使用
- その他、必要に応じて標準ライブラリまたはサードパーティライブラリを使用

## 4. 制約事項

### 4.1 Change Calendar制約
- AWS Systems Manager Change Calendarのドキュメントサイズ制限に従う
- 登録可能なイベント数には実質的な上限が存在する

### 4.2 実行環境
- AWS認証情報が適切に設定されている必要がある
- 必要なIAM権限:
  - `ssm:GetCalendarState`
  - `ssm:DescribeDocument`
  - `ssm:UpdateDocument`
  - その他、Change Calendar操作に必要な権限

## 5. 使用例

### 5.1 基本的な使用例

```bash
# 標準的な実行(1年間、Asia/Tokyoタイムゾーン)
export SSM_CALENDAR_TIMEZONE=Asia/Tokyo
update-ssm-calendar --calendar my-calendar --config events.yaml

# ドライラン実行
update-ssm-calendar --calendar my-calendar --config events.yaml --dry-run

# 詳細ログ出力
update-ssm-calendar --calendar my-calendar --config events.yaml --log-level verbose

# 期間を指定(6ヶ月間)
export SSM_CALENDAR_DURATION_DAYS=180
export SSM_CALENDAR_TIMEZONE=Asia/Tokyo
update-ssm-calendar --calendar my-calendar --config events.yaml
```

### 5.2 出力例

#### normal レベル (デフォルト)
```
Loaded 5 events from events.yaml
Validation passed
Target Calendar: my-calendar (ap-northeast-1)
Deleted 30 existing events
Registered 260 new events (from 2025-10-13 to 2026-10-12)
Success!
```

#### verbose レベル
```
Loaded 5 events from events.yaml
  - Monday Event (Monday 08:00 - Monday 22:00)
  - Tuesday Event (Tuesday 08:00 - Tuesday 22:00)
  ...
Validation: checking required fields... OK
Validation: checking day of week format... OK
Validation: checking time format... OK
Validation: checking document size limit... OK (estimated 45KB / 100KB)
Target Calendar: my-calendar
Region: ap-northeast-1
Document Version: 3
Deleting existing events...
  Deleted: 30 events
Registering new events...
  2025-10-13 08:00 - 22:00 (Monday Event) ... OK
  2025-10-14 08:00 - 22:00 (Tuesday Event) ... OK
  ...
Registered 260 new events (from 2025-10-13 to 2026-10-12)
Success!
```

## 6. 補足事項

### 6.1 今後の検討事項
- Change Calendar自動作成機能
- イベント定義の差分更新機能
- 複数のChange Calendarへの一括登録
- JSONフォーマットのサポート

### 6.2 参考資料
- [AWS Systems Manager Change Calendar](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-change-calendar.html)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/)
