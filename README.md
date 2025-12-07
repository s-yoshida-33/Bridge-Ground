# Bridge-Ground (BridgeGround)

## 概要
**Bridge-Ground** は、外部のモールデータAPIからデータを同期し、ローカル環境でREST APIおよび静的ファイルサーバーとして機能するElectronアプリケーションです。
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
- **メディアダウンロード**: 各データに関連付けられた画像ファイルを自動的にダウンロードし、ローカルに保存します。
- **同期モード**:
  - **手動同期**: UIまたはAPI経由で即時実行。
  - **起動時同期**: アプリ起動時にバックグラウンドで実行（設定可）。
  - **自動定期同期**: 設定された間隔（分単位）で定期的に実行（設定可）。

### 2. ローカルAPIサーバー機能
同期したデータをJSON形式で提供するREST APIサーバーを内蔵しています。
- **ポート**: デフォルト `8080` (設定可能)
- **静的ファイル配信**: ダウンロードした画像ファイルを `/files` パスで配信します。
- **ダッシュボード**: ルートURL (`/`) にアクセスすると、利用可能なAPIエンドポイントの一覧を表示します。

### 3. デスクトップ統合
- **システムトレイ**: アプリはタスクトレイに常駐し、ウィンドウを閉じてもバックグラウンドで動作を継続します。
- **設定管理**: API接続先や同期スケジュールをGUIまたは設定ファイルで管理します。

## 技術スタック

- **Platform**: Electron
- **Runtime**: Node.js
- **Server Framework**: Express
- **Database**: SQLite (via `better-sqlite3`)
- **HTTP Client**: Axios
- **XML Parser**: xml2js

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
- Node.js (推奨: v18以上)
- npm

### インストール
依存パッケージをインストールします。
```bash
npm install
# または
npm ci
```

### 開発モードでの実行
Electronアプリを起動します。
```bash
npm start
```

### ビルド
インストーラー（Windows向け `exe`）を作成します。
```bash
npm run build
```

## 設定

設定はアプリケーション内の機能、または `src/config/api_config_default.json` をベースにしたユーザー設定で管理されます。

```json
{
  "apiSettings": {
    "baseUrl": "https://target-api-domain.com/api", // 同期元APIのベースURL
    "username": "", // Basic認証用 (必要な場合)
    "password": ""  // Basic認証用 (必要な場合)
  },
  "serverSettings": {
    "port": 8080 // ローカルAPIサーバーのポート
  },
  "syncSettings": {
    "autoSyncEnabled": false,     // 自動同期の有効/無効
    "syncIntervalMinutes": 60,    // 自動同期の間隔(分)
    "syncOnStartup": false        // 起動時の同期実行
  }
}
```

## データベーススキーマ
データは `src/db/setup.sql` で定義されたスキーマに従って `shops`, `event_news`, `shop_news`, `specials`, `genres` テーブルに格納されます。
