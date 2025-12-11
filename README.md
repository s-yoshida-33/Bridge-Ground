# Bridge-Ground (Go Version)

## 概要
**Bridge-Ground** は、外部のモールデータAPIからデータを同期し、ローカル環境でREST APIおよび静的ファイルサーバーとして機能するアプリケーションです。
以前はElectronを使用していましたが、現在は完全に **Golang** に移行されました。
外部システム（BridgeWebPopper等）のデータをローカルデータベース（SQLite）に取り込み、オフラインやローカルネットワーク内でのデータ利用を可能にします。また、画像データもローカルにダウンロードして配信します。

## 主な機能

### 1. データ同期機能
外部APIからXML形式のデータを取得し、ローカルデータベースに保存します。
- **対象データ**:
  - ショップ情報 (`/shoplist`)
  - ジャンルマスタ (`/genrelist`)
  - イベントニュース (`/eventnewslist`)
  - ショップニュース (`/shopnewslist`)
  - 特集・セール情報 (`/speciallist`)
- **メディアダウンロード**: 各データに関連付けられた画像ファイルを自動的にダウンロードし、ローカルに保存します（Goの並行処理により高速化）。
- **同期モード**:
  - **手動同期**: UIまたはAPI経由で即時実行。
  - **起動時同期**: アプリ起動時にバックグラウンドで実行（設定可）。
  - **自動定期同期**: 設定された間隔（分単位）で定期的に実行（設定可）。

### 2. ローカルAPIサーバー機能
同期したデータをJSON形式で提供するREST APIサーバーを内蔵しています。
- **ポート**: デフォルト `8080` (設定可能)
- **静的ファイル配信**: ダウンロードした画像ファイルを `/files` パスで配信します。
- **ダッシュボード**: ルートURL (`/`) にアクセスすると、UIが表示されます。

### 3. デスクトップUI
- **Lorca**: Chrome/Edgeを利用した軽量なGUIを提供します。

## 技術スタック

- **Language**: Go (Golang)
- **GUI Library**: Lorca (Uses installed Chrome/Edge)
- **Database**: SQLite (modernc.org/sqlite - CGO-free)
- **HTTP Server**: net/http (Standard Library)

## API仕様

ローカルサーバー（デフォルト: `http://localhost:8080`）は以下のエンドポイントを提供します。

### データ取得API
全てのレスポンスはJSON形式です。

| エンドポイント | メソッド | 説明 |
| --- | --- | --- |
| `/api/shops` | GET | ショップ情報の全件リストを取得 |
| `/api/shop-news` | GET | ショップニュースの全件リストを取得 |
| `/api/event-news` | GET | イベントニュースの全件リストを取得 |
| `/api/specials` | GET | 特集・セール情報の全件リストを取得 |
| `/api/sales` | GET | セール一覧を取得（`/api/specials` と同等） |
| `/api/genres` | GET | ジャンルマスタを取得 |

### 静的ファイル
ダウンロードされた画像は以下のパスでアクセス可能です。
- URL: `http://localhost:8080/files/<path_to_image>`
- 実体パス: `AppData/Roaming/TTI/BridgeGround/files/` (OSにより異なる)

## セットアップと実行

### 前提条件
- Go 1.21以上 (開発・ビルド時)
- Google Chrome または Microsoft Edge (実行時)

### ビルド
```bash
go build -o bridge-ground.exe ./cmd/bridgeground
```

### 実行
```bash
./bridge-ground.exe
```

## 設定

設定は `%APPDATA%/TTI/BridgeGround/config.json` に保存されます。
アプリ内の「設定」画面から変更可能です。
