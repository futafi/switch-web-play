# Screenshot API 最適化

## 概要

`/api/screenshot` のレイテンシを 960ms → 200ms に改善した後、さらに常駐ffmpegによるJPEGキャッシュ方式へ変更した。
通常のスクリーンショット取得では、リクエスト時にRTSPへ接続せず、メモリ上の最新JPEGを返す。

## 変更内容

### 2026-05-19: ワンショット取得の最適化

ffmpegに以下のオプションを追加した。

```
-threads 1 -probesize 32 -analyzeduration 0 -fflags nobuffer
```

H.265ストリームの途中から1フレームだけ取得すると壊れた画像になることがあったため、最新コミットで以下も追加した。

```
-skip_frame nokey
```

ただし、この指定はkeyframe/IDRだけをデコード対象にするため、`keyframe-period=30` / 30fps の現在設定では、タイミングによって最大約1秒待つ可能性がある。

### 2026-05-20: 常駐ffmpeg + メモリキャッシュ

`nxmc-server` 起動時にスクリーンショット用ffmpegを常駐させ、RTSPを連続デコードしてJPEGを `image2pipe` で受け取る。

```
ffmpeg
  -threads 1 -probesize 32 -analyzeduration 0 -fflags nobuffer
  -rtsp_transport tcp -i <rtsp-url>
  -an -vf fps=${SCREENSHOT_CACHE_FPS:-10}
  -f image2pipe -c:v mjpeg -q:v 2 pipe:1
```

Go側はstdoutからJPEGフレームを読み、最新1枚だけをメモリに保持する。

```go
type screenshotCache struct {
    jpeg      []byte
    updatedAt time.Time
}
```

`SCREENSHOT_CACHE_FPS` のデフォルトは `10`。通常リクエストでは最大およそ100ms前のJPEGを返す想定で、レスポンスヘッダ `X-Screenshot-Age-Ms` にキャッシュの経過時間を入れる。

`width` / `height` 指定時は、キャッシュJPEGを入力としてffmpegでリサイズする。キャッシュ未準備または3秒以上更新されていない場合のみ、従来のワンショット取得へフォールバックする。

## 処理フロー

現在の通常フロー:

```
nxmc-server起動
  → 常駐ffmpegでRTSP TCP接続
    → H.265を連続デコード
      → fps=10でJPEG化
        → Goメモリ上の最新JPEGを上書き

API リクエスト
  → 最新JPEGをメモリからコピー
    → HTTPレスポンス
```

フォールバック時のフロー:

```
API リクエスト
  → Go exec.Command で ffmpeg プロセス起動
    → RTSP TCP接続 (mediamtx)
      → IDRフレーム受信 + H.265デコード
        → JPEG エンコード
          → pipe経由でHTTPレスポンス
```

常駐キャッシュ方式では、通常リクエスト時のffmpegプロセス起動、RTSP接続、keyframe待ちを避ける。代わりに、RTSPクライアントが1つ増え、H.265ソフトウェアデコードとJPEGエンコードが常時発生する。

## 最適化前に960msかかっていた原因

### 主因: H.265マルチスレッドデコーダのフレームバッファリング (~530ms)

ffmpegのH.265デコーダはデフォルトでCPUコア数分(=16)のスレッドを起動し、フレームレベル並列デコードを行う。最初の1フレームを出力する前に、スレッド数分のフレームをリアルタイムストリームから先読みする。

- 16スレッド × 33ms/frame (30fps) = 約530ms の待ち時間
- `-threads 1` で解消

```
threads= 1: avg= 273ms  (3 packets / 1 frame decoded)
threads= 4: avg= 374ms  (6 packets / 4 frames decoded)  
threads=16: avg= 784ms  (18 packets / 16 frames decoded)
```

### 副因: ffmpegストリーム解析 (analyzeduration) (~230ms)

ffmpegはデフォルトでストリームのfps推定のために複数フレームを読み込んで分析する(rfps計算)。スクリーンショット用途では不要。

- `-analyzeduration 0 -probesize 32` で解消

### 解像度(1080p vs 720p)は影響しない

ボトルネックがI/Oバウンド(ストリームからのフレーム受信待ち)であるため、スケーリングのCPU処理コスト(数ms)は誤差範囲。

## 最適化後の計測結果 (~200ms)

```
native: avg=200ms  med=200ms  min=198ms  max=201ms  stdev=1ms   size=16.6KB
1080p:  avg=200ms  med=200ms  min=197ms  max=203ms  stdev=2ms   size=15.6KB
720p:   avg=210ms  med=199ms  min=187ms  max=305ms  stdev=34ms  size=6.9KB
```

### 200msの内訳 (推定)

| フェーズ | 時間 | 備考 |
|---------|------|------|
| ffmpegプロセス起動 | ~50ms | Alpine Linux上、ライブラリロード含む |
| RTSP接続 (TCP + DESCRIBE/SETUP/PLAY) | ~5ms | Docker内部ネットワーク |
| IDRフレーム受信 | ~30ms | mediamtxがPLAY応答時にIDRから配信開始するため待ちほぼゼロ |
| H.265デコード (1フレーム, threads=1) | ~40ms | ソフトウェアデコード |
| JPEGエンコード + パイプ出力 | ~5ms | |
| Go exec.Command + HTTPレスポンス | ~70ms | プロセス生成、パイプ読み出し |

現在の200msのうち、ffmpegプロセス起動(~50ms)とGo exec overhead(~70ms)で約6割を占める。ストリームからのフレーム取得・デコード自体は~75ms程度。
