# Screenshot API 最適化

## 概要

`/api/screenshot` のレイテンシを 960ms → 200ms に改善した後、GStreamerのJPEG分岐から最新JPEGをキャッシュする方式へ変更した。
通常のスクリーンショット取得では、リクエスト時にRTSPへ接続せず、nxmc-serverのメモリ上にある最新JPEGを返す。

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

### 2026-05-20: GStreamer JPEG分岐 + メモリキャッシュ

常駐ffmpeg方式は `/api/screenshot` を高速化できたが、nxmcコンテナでH.265ソフトウェアデコードが常時発生した。これを避けるため、GStreamerパイプラインでエンコード前のraw videoを `tee` で分岐し、スクリーンショット用JPEG streamをTCPでnxmcへ渡す方式に変更した。

GStreamer側:

```
... ! capsfilter name=scaler caps=video/x-raw,width=W,height=H
    ! tee name=video_split

video_split. ! queue
    ! vaapih265enc ! h265parse ! rtspclientsink

video_split. ! queue leaky=downstream max-size-buffers=1
    ! videorate ! video/x-raw,framerate=${SCREENSHOT_CACHE_FPS:-10}/1
    ! jpegenc quality=95
    ! multipartmux boundary=frame
    ! tcpserversink host=0.0.0.0 port=${SCREENSHOT_STREAM_PORT:-9001} sync=false
```

nxmc側:

```
net.Dial("gstreamer:9001")
  → JPEG streamからSOI/EOIを探して1枚ずつ読み出し
    → Goメモリ上の最新JPEGを上書き
```

```go
type screenshotCache struct {
    jpeg      []byte
    updatedAt time.Time
}
```

`SCREENSHOT_CACHE_FPS` のデフォルトは `10`。通常リクエストでは最大およそ100ms前のJPEGを返す想定で、レスポンスヘッダ `X-Screenshot-Age-Ms` にキャッシュの経過時間を入れる。Docker bridge構成ではnxmcは `gstreamer:9001` に接続する。host network構成では `127.0.0.1:9001` に接続する。明示的に変える場合は `SCREENSHOT_STREAM_ADDR` を使う。

`width` / `height` 指定時は、キャッシュJPEGをGo内でdecode/resize/re-encodeする。リサイズには `golang.org/x/image/draw` の `ApproxBiLinear` を使い、JPEG品質は95にしている。キャッシュ未準備または3秒以上更新されていない場合のみ、従来のワンショット取得へフォールバックする。

## 処理フロー

現在の通常フロー:

```
nxmc-server起動
  → gstreamer:9001 のJPEG streamへTCP接続
    → JPEGフレームを読み続ける
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

GStreamer JPEG分岐方式では、通常リクエスト時のffmpegプロセス起動、RTSP接続、keyframe待ちを避ける。nxmc側でH.265を再デコードせず、JPEG streamを読むだけにする。

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
