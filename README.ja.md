# 目標ディスク使用量ベースのファイルクリーニング

ディスク上の占有ブロックサイズを考慮したファイルクリーニングツールです。

キャッシュファイルやバックアップはシステムの稼働に伴い増加します。これらのファイルがディスクの空き容量を枯渇させないよう、定期的に削除するために利用します。

## 使い方

以下のコマンドを実行すると、ディスクの使用量が 80GB 以下以下になるように`/var/cache`ディレクトリにあるファイルを最終更新時刻の古い順に削除します。

```bash
file-cleaner 80gb /var/cache
```

## 引数・オプション

```bash
file-cleaner [options] <targetSize> <rootPath>
```

- `<targetSize>` ディスクの使用量の目標値を指定します(例 `80gb`, `100mb`)。この引数は必須です。
- `<rootPath>` ファイル削除の対象となるディレクトリを指定します。この引数は必須です。

`[options]`には以下のオプションを指定できます。

- `-a`, `--atime` 最終更新時刻ではなく最終アクセス時刻を基準に古いファイルを削除します。
- `-c`, `--concurrency` ディレクトリの走査やファイル削除の処理を並列で実行します。0 を指定すると CPU コア数を取得して適用します。
- `-b`, `--time-bin` 集計とソートのメモリ節約のため、ファイルの最終時刻を切り上げる単位を指定します(例 `1h`, `60m`)。デフォルトは 1 時間です。
- `-p`, `--pattern` 指定ディレクトリ内で削除対象となるファイルのパターンを指定します(例 `*/backup/**`)。
- `--block-size` ディスクにおけるブロックサイズをバイト単位で指定します。デフォルトは `4096` で、UNIX においては自動検出を試みます。
- `--dry-run` 実際にファイルは削除せず、削除されるファイルを表示します。
- `--examples` 実際に削除されたファイルを指定した件数表示します。

`-p`によるパターンの評価には [doublestar](https://github.com/bmatcuk/doublestar) を利用しています。

## ライブラリとして利用

`file-cleaner`はライブラリとしても利用できます。以下のようにインポートして利用できます。

```go
package main

import (
  filecleaner "github.com/ideamans/go-file-cleaner"
)

func main() {
  output, err := filecleaner.Clean(filecleaner.CleaningInput{
    TargetSize: 1024 * 1024 * 1024 * 80,
    RootPath:   "/var/cache",
  })

  fmt.Println(output)

  if err != nil {
    panic(err)
  }
}
```
