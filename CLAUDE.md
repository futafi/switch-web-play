# web-switch

Nintendo SwitchをHDMIキャプチャ経由でWebからプレイするための個人用システム。

## アーキテクチャ

- **映像配信**: mediamtx でHDMIキャプチャデバイスの映像をWebRTC配信
- **コントローラー操作**: nxmc で有線USB UART経由のコントローラー操作 (Go)
  - PC <-> FTDI FT232R (USB UART) <-> Arduino Leonardo <-> Switch
  - NX2プロトコル (0xAB, 11バイト固定長)
- **Webフロントエンド**: 映像表示 + コントローラー入力の統合UI

## リファレンス / 実験

- `ref/nuxbt/` - Switchコントローラーエミュレーター (Python, Flask+SocketIO) ※凍結
- `nuxbt-test/` - Bluetooth接続実験用。現行本線では使わないが、再検証用に保持。
- `nxmc/firmware/` - Arduino側ファームウェア参考コード
- ファームウェアリポジトリ: https://github.com/our-holyland/NXMCForLeonardo.git

## 方針

- 個人専用ソフトウェア。fail fast、YAGNI。
- 実装単位ごとに `_docs/yyyy-mm-dd-タイトル.md` でドキュメントを作成する。
- 日本語でコミュニケーション。
