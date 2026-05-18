# web-switch

Nintendo SwitchをHDMIキャプチャ経由でWebからプレイするための個人用システム。

## アーキテクチャ

- **映像配信**: mediamtx でHDMIキャプチャデバイスの映像をWebRTC配信
- **コントローラー操作**: nuxbt でBluetooth経由のProコントローラーエミュレーション
- **Webフロントエンド**: 映像表示 + コントローラー入力の統合UI

## リファレンス

- `ref/mediamtx/` - 映像配信サーバー (Go)
- `ref/nuxbt/` - Switchコントローラーエミュレーター (Python, Flask+SocketIO)

## 方針

- 個人専用ソフトウェア。fail fast、YAGNI。
- 実装単位ごとに `_docs/yyyy-mm-dd-タイトル.md` でドキュメントを作成する。
- 日本語でコミュニケーション。
