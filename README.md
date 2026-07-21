# statusboard

簡易なステータスページを作成するツールです

## 起動方法

`--toml` と `--data` は必須です。

```sh
./statusboard --toml /path/to/statusboard.toml --data /path/to/data
```

デフォルトでは `:8080` で待ち受けるため、ブラウザで `http://localhost:8080/` にアクセスします。

### 設定ファイルの文法チェックのみ実行

```sh
./statusboard --toml /path/to/statusboard.toml --data /path/to/data --check
```

問題なければ `syntax OK` を出力して終了します。

## オプション

| オプション | 必須 | デフォルト | 説明 |
| --- | --- | --- | --- |
| `-l`, `--listen` | 任意 | `:8080` | バインドするアドレス (`address:port`) |
| `--toml` | 必須 | なし | TOML設定ファイルへのパス |
| `--data` | 必須 | なし | ログ出力先ディレクトリへのパス |
| `--check` | 任意 | `false` | 設定の文法チェックのみ実行して終了 |
| `-v`, `--version` | 任意 | `false` | バージョンを表示して終了 |

## TOMLファイルについて

`--toml` で指定する設定ファイルです。サンプルは `demo.toml` を参照してください。

### 最低限必要な構成

```toml
[[category]]
name = "グローバル"

[[category.service]]
name = "API"
command = ["sh", "-c", "exit 0"]
```

`category.service.command` は必須です。空だと起動時にエラーになります。

### 主な設定項目

- `lang`: HTMLの`lang`属性。未指定時は `ja`
- `title`: ページタイトル。未指定時は `Status Board`
- `favicon`: favicon URL
- `nav_title`: ナビゲーションタイトル (Markdown可)
- `nav_button_name`: ナビゲーションボタン名
- `nav_button_link`: ナビゲーションボタンのリンク先
- `header_message`: ヘッダーメッセージ (Markdown可)
- `footer_message`: フッターメッセージ (Markdown可)
- `powered_by`: フッター下部テキスト (Markdown可)
- `num_of_worker`: ヘルスチェック並列数 (デフォルト `4`)
- `max_check_attempts`: 失敗時のリトライ回数 (デフォルト `3`)
- `retry_interval`: リトライ間隔 (`time.ParseDuration` 形式、例: `"5s"`)
- `worker_interval`: ヘルスチェック間隔 (`time.ParseDuration` 形式、例: `"5m"`)
- `worker_timeout`: ヘルスチェックのタイムアウト (`time.ParseDuration` 形式、例: `"30s"`)
- `latest_time_range`: 最新状態として扱う期間 (`time.ParseDuration` 形式、例: `"1h"`)

### category / service

- `[[category]]`
- `name`: カテゴリ名
- `comment`: カテゴリ説明
- `hide`: `true` にすると画面上でカテゴリを非表示

- `[[category.service]]`
- `name`: サービス名
- `command`: 実行コマンド配列。例: `["sh", "-c", "curl -fsS https://example.com"]`

### dataディレクトリ

`--data` で指定したディレクトリ配下に、日付ごとのログファイルが作られます。

- ファイル名形式: `logYYYYMMDD.txt`
- 各行はJSON形式のログ

