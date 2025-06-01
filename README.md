# update-ssm-calendar

yaml ファイルの定義に従い AWS System Manager の Change Calendar のイベントを登録するコマンドラインツール

## Usage

yaml ファイルとして、下表で示した構造を配列で用意する。

|No.|key|description|example|
|---|---|---|---|
|1|name|イベントの名前|`event`|
|2|start_day_of_week|イベントを開始する曜日|`monday`|
|3|start_time|イベントを開始する時間 (hh:MM)|`08:00`|
|4|end_day_of_week|イベントを終了する曜日|`monday`|
|5|end_time|イベントを終了する時間 (hh:MM)|`22:00`|

下記は yaml ファイルのサンプルとなる。

```yaml
- name: "monday"
  start_day_of_week: "monday"
  start_time: "08:00"
  end_day_of_week: "monday"
  end_time: "22:00"
- name: "tuesday"
  start_day_of_week: "tuesday"
  start_time: "08:00"
  end_day_of_week: "tuesday"
  end_time: "22:00"
- name: "wednesday"
  start_day_of_week: "wednesday"
  start_time: "08:00"
  end_day_of_week: "wednesday"
  end_time: "22:00"
- name: "thursday"
  start_day_of_week: "thursday"
  start_time: "08:00"
  end_day_of_week: "thursday"
  end_time: "22:00"
- name: "friday"
  start_day_of_week: "friday"
  start_time: "08:00"
  end_day_of_week: "friday"
  end_time: "22:00"
```

下記のように実行する

```bash
update-ssm-calendar --calendar test-calendar --config config.yaml
```
