# nuxbt Bluetooth接続テスト (2026-05-19)

Nintendo Switchをnuxbtで操作するため、Dockerコンテナ内のBlueZからPro Controllerエミュレーションを試した。

## 構成

- nuxbt: `ref/nuxbt`
- 実験用Compose: `nuxbt-test/docker-compose.yml`
- 実験用Dockerfile: `nuxbt-test/Dockerfile`
- Bluetoothアダプタ: Intel USB/内蔵アダプタ
  - MAC: `C8:5E:A9:C4:3B:31`
  - HCI: `hci0`
- Switch側MAC: `B8:68:70:27:F6:CD`

## Docker構成の確認

以下は確認済み。

- `docker compose -f nuxbt-test/docker-compose.yml config` 成功
- frontend stageのno-cache build 成功
- 最終イメージ build 成功
- compose経由で `nuxbt-test-nuxbt` イメージ build 成功
- イメージ内で `nuxbt --help` 成功
- Web asset配置確認済み
  - `/app/nuxbt/web/templates/index.html`
  - `/app/nuxbt/web/static/dist/.vite/manifest.json`

追加した調整:

- `.dockerignore` を追加し、nuxbtビルドに不要な `ref/mediamtx` などをDocker build contextから除外。
- `nuxbt-test/docker-compose.yml` に `/var/lib/bluetooth` 永続化用ボリュームを追加。

```yaml
volumes:
  - ./bluetooth-state:/var/lib/bluetooth
```

## 実行した確認

### WebUI

WebUIは起動し、ブラウザからアクセスできた。

Socket.IOのWebSocket upgradeで `403` が出ることはあったが、pollingで通信できており、今回のBluetooth接続失敗とは別問題。

### nuxbt check

コンテナ内で成功。

```text
NUXBT Plugin Enabled
```

### Bluetoothアダプタ状態

`Create Pro Controller` 後、コンテナ内ではPro Controllerとして公開できていた。

```text
Alias: Pro Controller
Powered: yes
Discoverable: yes
Pairable: yes
UUID: Human Interface Device
Class: 0x00002508
```

`hciconfig -a` でも以下を確認。

```text
UP RUNNING PSCAN ISCAN
Name: 'Pro Controller'
Device Class: Peripheral, Gamepad
Link mode: PERIPHERAL ACCEPT
```

### nuxbt test

`-r` は使わず、以下で実行。

```bash
docker compose -f nuxbt-test/docker-compose.yml run --rm nuxbt nuxbt -d test --timeout 60
```

結果:

- NUXBT初期化: 成功
- Bluetooth adapter検出: 成功
- Controller作成: 成功
- Switch接続待ち: タイムアウト

## btmonでの切り分け

`btmon` でHCIレベルのログを確認したところ、Switchから接続要求は来ていた。

```text
Connect Request: B8:68:70:27:F6:CD (Nintendo Co.,Ltd)
Connect Complete: Success
Disconnect Complete: Reason: Authentication Failure
```

つまり、問題は「Switchからnuxbtが見えていない」ではない。

Switchは `Pro Controller` として見えているBluetoothアダプタへ接続しに来ているが、Bluetooth認証で失敗して即切断している。

## 試したリセット

- Switch側:
  - `設定 > コントローラーとセンサー > コントローラーとの通信を切る`
  - Switch本体再起動
- Linux/nuxbt側:
  - `nuxbt-test/bluetooth-state` 削除
  - ホスト側 `/var/lib/bluetooth` 確認

ホスト側 `/var/lib/bluetooth` は存在しなかった。この環境ではDocker volume側の `nuxbt-test/bluetooth-state` が主な保存先。

上記を実施しても `Authentication Failure` は継続した。

## 推定原因

以前一度接続に成功したあと、コンテナ削除によりBlueZ側のペアリング情報が消えた。その後、Switch側またはアダプタ側の状態とnuxbt側の状態が食い違い、同じBluetooth MAC `C8:5E:A9:C4:3B:31` に対して認証失敗している可能性が高い。

ただし、Switch側の通信解除・再起動、Linux側の保存情報削除後も失敗するため、Intel Bluetoothアダプタとnuxbt/BlueZの相性も疑う。

nuxbtはBR/EDR HIDエミュレーションに依存するため、Bluetoothアダプタ相性が出やすい。

## 次回の方針

USB Bluetoothドングルを追加し、別アダプタで再テストする。

最初に確認するコマンド:

```bash
docker compose -f nuxbt-test/docker-compose.yml run --rm nuxbt hciconfig -a
```

複数アダプタが出た場合、nuxbtが新しいUSBドングル側を使うように調整する。

確認ポイント:

- 新しいアダプタのMACアドレス
- `hci0` / `hci1` の割り当て
- `nuxbt test` でどのadapter pathが使われるか
- `btmon` で `Authentication Failure` が消えるか

再テスト候補:

```bash
docker compose -f nuxbt-test/docker-compose.yml run --rm nuxbt nuxbt -d test --timeout 60
```

必要なら `btmon` を併用する。

```bash
docker compose -f nuxbt-test/docker-compose.yml run --rm nuxbt bash -lc 'btmon > /tmp/btmon.log 2>&1 & mon=$!; sleep 1; printf "\n" | nuxbt -d test --timeout 45; status=$?; kill $mon 2>/dev/null || true; sleep 1; grep -E "Connect Request|Connect Complete|Authentication Failure|Disconnect Complete|Nintendo Co" /tmp/btmon.log || true; exit $status'
```

## 追試: TP-Link UB5Aドングル (2026-05-19)

USB Bluetoothドングルとして TP-Link UB5A を追加した。

ホストでは以下として認識。

```text
Bus 003 Device 005: ID 2357:0604 TP-Link TP-Link UB5A Adapter
```

コンテナ内では以下の2アダプタ構成になった。

```text
hci0: C8:5E:A9:C4:3B:31 Intel
hci1: E8:48:B8:C8:40:00 TP-Link UB5A / Realtek
```

TP-Link側は初期状態では `DOWN` だったため、テスト時は明示的に以下を実行した。

```bash
hciconfig hci1 up
hciconfig hci0 down
```

`nuxbt test` CLIはadapter指定できないため、`nuxbt-test/test_adapter.py` を追加して `/org/bluez/hci1` を直接指定した。

```bash
docker compose -f nuxbt-test/docker-compose.yml run --rm \
  -v /root/web-switch/nuxbt-test/test_adapter.py:/tmp/test_adapter.py:ro \
  nuxbt bash -lc 'hciconfig hci1 up || true; hciconfig hci0 down || true; python /tmp/test_adapter.py /org/bluez/hci1 60'
```

結果:

- `/org/bluez/hci1` を指定してController作成: 成功
- Switch接続待ち: タイムアウト
- `btmon`: TP-Link `hci1` にSwitchから接続要求あり
- 失敗理由: Intel時と同じ `Authentication Failure`

btmon要約:

```text
New Index: E8:48:B8:C8:40:00 (Realtek Semiconductor Corporation) [hci1]
Connect Request: B8:68:70:27:F6:CD (Nintendo Co.,Ltd)
Connect Complete
Disconnect Complete
Reason: Authentication Failure (0x05)
```

つまり、TP-Linkへ変更しても「Switchから見えていない」問題ではなく、認証で落ちている。

## Agent capabilityの変更

nuxbtのBlueZ agentは元々 `DisplayYesNo` で登録されていた。

Switch相手のJust Works寄りにするため、`ref/nuxbt/nuxbt/agent.py` を変更し、環境変数でcapabilityを切り替え可能にしたうえで、デフォルトを `NoInputNoOutput` に変更した。

```python
capability = os.environ.get("NUXBT_AGENT_CAPABILITY", "NoInputNoOutput")
```

イメージを再ビルドしてTP-Link `hci1` で再テストしたが、結果は変わらず `Authentication Failure`。

## MACアドレス変更の調査

参考情報として、Nintendo Switchにコントローラーとして認識させるためにBluetoothアダプタのMAC先頭をNintendo OUIへ変更する手順がある。

例:

```bash
sudo bccmd psset -s 0 bdaddr ...
sudo bccmd warmreset
```

ただし、この `bccmd` 手順は主にCSR系Bluetoothチップ向け。今回のTP-Link UB5AはRealtekとして認識されるため、そのまま適用できない可能性が高い。

確認結果:

- コンテナ内に `bccmd` は存在しない
- TP-Link UB5A は `Manufacturer: Realtek Semiconductor Corporation (93)`
- nuxbt内の `replace_mac_addresses()` は `hcitool cmd 0x3f 0x001 ...` を使う
- コマンド自体は `replace ok` になるが、`hciconfig` 上のBD Addressは変わらなかった
- `btmgmt --index 1 public-addr 7C:BB:8A:12:34:56` は `Not Supported`

つまり、TP-Link UB5Aでは少なくとも現手順ではBluetooth public address変更はできていない。

## Device IDの調査

btmon上のExtended Inquiry ResponseではDevice IDがLinux Foundation由来に見える。

```text
Device ID: USB Implementer's Forum assigned
Vendor: Linux Foundation (0x1d6b)
Product: 0x0246
```

nuxbtのSDP record (`nuxbt/controller/sdp/switch-controller.xml`) は `Nintendo` 表記を含むが、EIR/DID側は別途 `btmgmt did` で設定できる可能性がある。

`btmgmt did` の構文:

```text
did <source>:<vendor>:<product>:<version>
```

Nintendo Pro Controller相当として以下を試そうとした。

```bash
btmgmt --index 1 did 0x0002:0x057e:0x2009:0x0001
```

ただしこの確認中にコマンドが戻らず、診断コンテナを停止した。未完了。

## toggleについて

`nuxbt toggle` はホスト側では実行していない。

Dockerコンテナ内では `entrypoint.sh` が実質的にtoggle相当の状態を作っている。

- `bluetoothd --compat --noplugin=*`
- `/run/systemd/system/bluetooth.service.d/nuxbt.conf` を作成
- Pythonに `cap_net_raw,cap_net_admin,cap_net_bind_service` を付与
- `nuxbt check` は `NUXBT Plugin Enabled`

ホスト側には `bluetooth.service` が存在しない環境だったため、ホストsystemdのBluetoothサービスには触っていない。

## 現時点の結論

- Intel `hci0` でもTP-Link `hci1` でも、Switchから接続要求は来る
- どちらも `Authentication Failure` で切断される
- `-r` は使っていない
- Switch側の「コントローラーとの通信を切る」と再起動は実施済み
- Docker側Bluetooth保存情報の削除も実施済み
- 保存情報は今後むやみに消さない方針
- TP-Link UB5AのMAC変更は現手順ではできていない
- `NoInputNoOutput` agentでも改善しなかった

次に見るなら:

- `btmgmt did` を安全に完了させてDevice IDをNintendoに寄せられるか
- Realtek系UB5AでBD_ADDR変更できる別手段があるか
- CSR系など `bccmd` でBD_ADDR変更できるドングルを使うか
- ホスト側BlueZで直接nuxbtを動かす構成との比較
