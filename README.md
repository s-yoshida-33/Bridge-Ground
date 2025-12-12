# BridgeGround (Local Data Sync Server)

BridgeGround は、外部 API からデータを定期的に同期し、ローカルアプリケーション向けに最適化された API とイベント通知を提供するミドルウェアサーバーです。
Golang と Lorca (Chrome/Edge UI) を使用して構築されています。

## 主な機能

1.  **データ同期**: 外部 API からショップ情報、ニュース、特集などの XML データを取得し、ローカル SQLite データベースに保存します。
2.  **メディア管理**: 画像や動画などのメディアファイルを自動的にダウンロードし、ローカルパスとして管理します。
3.  **ローカル API**: 同期されたデータを取得するための REST API を提供します。
4.  **リアルタイム通知**: データの更新を検知した際に Server-Sent Events (SSE) でクライアントに通知します。

## 開発環境

-   Go 1.21+
-   GCC (for SQLite CGO)

## 起動方法（開発時）

```bash
# 依存関係のインストール
go mod download

# 実行
go run cmd/bridgeground/main.go
```

## ビルドと配布

### 1. ビルド
`run.bat` を実行すると、`build` フォルダに実行ファイルと必要なリソースが生成され、アプリが起動します。

### 2. 配布用パッケージの作成
`package.bat` を実行すると、`BridgeGround_v*` というフォルダに配布に必要なファイル一式がコピーされます。
このフォルダを圧縮して配布してください。

**配布物に含まれるもの:**
- `Bridge Ground.exe`: アプリ本体
- `config.json`: 設定ファイル
- `src/`: 画面リソースフォルダ

## インストール手順（利用者向け）

1. 配布されたフォルダを任意の場所に展開します。
2. `Bridge Ground.exe` をダブルクリックして起動します。
3. 画面が表示されれば起動成功です。タスクトレイやバックグラウンドではなく、ウィンドウとして動作します。
4. 設定画面で必要なAPI設定を行ってください。

## API 仕様

### ベース URL

デフォルト: `http://localhost:8090` (設定画面で変更可能)

### REST API

#### 店舗データ関連

-   `GET /api/shops`: 全店舗データ
-   `GET /api/shop-news`: 店舗ニュース
-   `GET /api/event-news`: イベントニュース
-   `GET /api/specials`: 特集・セール情報
-   `GET /api/genres`: ジャンルマスタ

※ レスポンス形式は JSON です。

### Server-Sent Events (SSE)

データの更新をリアルタイムに検知するために、SSE エンドポイントを提供しています。
クライアントアプリケーションは、このエンドポイントに接続し、`update` イベントを受信したタイミングで上記の REST API から最新データを再取得することを推奨します。

-   **エンドポイント**: `/api/events`

#### イベント一覧

| イベント名 | 説明 | データ例 |
| :--- | :--- | :--- |
| `connected` | 接続確立時に送信されます。 | `{"type": "connected", "message": "..."}` |
| `heartbeat` | 接続維持のため 30 秒ごとに送信されます。 | `{"timestamp": "...", "clients": 1}` |
| `update` | **同期処理によってデータの追加・更新・削除が検知された場合のみ**送信されます。 | `{"type": "update", "timestamp": "...", "message": "..."}` |

#### クライアント実装例 (JavaScript)

```javascript
const eventSource = new EventSource("http://localhost:8090/api/events");

// 接続確立
eventSource.addEventListener("connected", (e) => {
    console.log("Connected:", JSON.parse(e.data));
});

// データ更新通知
eventSource.addEventListener("update", (e) => {
    console.log("Data updated:", JSON.parse(e.data));
    // ここで最新データを再取得する処理を実行
    // fetchShops();
});

// ハートビート (接続確認用)
eventSource.addEventListener("heartbeat", (e) => {
    // console.log("Heartbeat:", JSON.parse(e.data));
});

// エラーハンドリングと再接続
eventSource.onerror = (e) => {
    console.error("SSE Error:", e);
    eventSource.close();
    // 適切な待機時間を置いて再接続
    setTimeout(() => {
        // 再接続ロジック
    }, 5000);
};
```

## 設定

アプリ起動時に表示される設定画面、または `config.json` で以下の設定が可能です。

-   **API Settings**: 外部 CMS のベース URL と認証情報
-   **Server Settings**: ローカルサーバーのポート番号
-   **Sync Settings**: 自動同期の有効化と間隔（分）

## ディレクトリ構成

-   `cmd/`: エントリーポイント
-   `internal/`: アプリケーションロジック
    -   `config/`: 設定管理
    -   `db/`: データベース操作
    -   `server/`: HTTP サーバーと SSE
    -   `sync/`: データ同期ロジック
    -   `models/`: データ構造体
-   `src/ui/`: 設定画面用フロントエンドアセット
-   `files/`: ダウンロードされたメディアファイル (実行時に生成)
