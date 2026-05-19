# Screenshot API 最適化

## 概要

`/api/screenshot` のレイテンシを 960ms → 200ms に改善した。
ffmpegコマンドオプションの最適化のみで、アーキテクチャ変更なし。

## 変更内容

ffmpegに以下のオプションを追加:

```
-threads 1 -probesize 32 -analyzeduration 0 -fflags nobuffer
```

## 処理フロー

```
API リクエスト
  → Go exec.Command で ffmpeg プロセス起動
    → RTSP TCP接続 (mediamtx)
      → IDRフレーム受信 + H.265デコード
        → JPEG エンコード
          → pipe経由でHTTPレスポンス
```

毎回ffmpegプロセスを起動し、RTSPストリームに接続してフレームを取得する方式。

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
