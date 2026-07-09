# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

BridgeGround は、**フロアガイドアプリ（Gido / Gido-Touch / Gido-Touch-Mini / Grain-Link）とセットで同一のSTB（Set-Top Box）にインストールされるローカルミドルウェア**。1施設に1台ではなく、STB（=フロアガイドアプリの実行機）1台につき1インスタンスが動く。外部の店舗API（各施設独自のテナント情報、XML形式）を定期的に同期し、ローカルSQLiteにキャッシュした上で、REST APIとSSEで**同一コンピュータ上の**フロアガイドアプリに配信する。

さらに、フロアガイドアプリとportal-cms（Firebase製の管理コンソール）の**橋渡し役**も担う（フロアガイドアプリ ⇄ Bridge-Ground ⇄ portal-cms）。フロアガイドアプリはportal-cmsと直接通信せず、Bridge-Groundのローカルapps API（`/api/apps/*`）に自己登録し、Bridge-Groundが自分自身の分と合わせてportal-cmsへ登録・ハートビート・ログ・設定・スクリーンショットを代理で送信する。

Go製、UIはLorca（Chrome/Edge埋め込み）、Windows exeとしてパッケージ配布される。

## Repository layout

- `cmd/bridgeground/` — エントリポイント（`main.go`、`versioninfo.json`でexeのバージョン情報埋め込み、`resource.syso`）
- `cmd/debug/` — デバッグ用ビルドの別エントリポイント
- `internal/config/` — 設定管理（`config.json`の読み書き、`Version`定数もここ）
- `internal/db/` — SQLite操作（`modernc.org/sqlite`、WALモード）
- `internal/logging/` — ファイルログ
- `internal/models/` — データ構造体（shops/news/specials/sales/genres/floors等）
- `internal/portal/` — portal-cms（Cloudflare Worker経由）への登録・ハートビート送信ロジック（`manager.go`）
- `internal/server/` — HTTPサーバー・ルーティング・SSE配信（`server.go`がルート定義の起点）
- `internal/sync/` — 上流APIからのXML取得・パース・DB同期ロジック
- `src/ui/` — 設定画面（Lorca）のフロントエンドアセット
- `build/` — `build-installer.ps1`（Inno Setupでインストーラ生成）、`installer.iss`
- `files/`（実行時生成） — ダウンロードしたメディアファイルのキャッシュ
- `xml/`, `mock_data/`, `mock_server.py` — 開発用のモックXML/モックサーバー

## Development commands

Go 1.24 + GCC（SQLiteのCGOビルドに必要）。Docker不使用、ローカルGo環境で直接ビルドする。

```bash
go mod download          # 依存取得
go run cmd/bridgeground/main.go   # 開発時の直接実行
```

反復開発は `run.bat` が早い（`build/`にビルド＋`config.json`/`src/`をコピーしてから起動まで一括）:
```bat
run.bat
```

バージョンを上げる場合（`internal/config/config.go`のVersion・`versioninfo.json`・`build/installer.iss`を一括更新):
```bat
bump-version.bat 4.1.1
```

リリース用インストーラ生成（Inno Setup 6が必要）:
```powershell
build/build-installer.ps1
```
→ `release/BridgeGroundSetup-x64-v<version>.exe` が生成される。

## Known gotchas

- **SQLiteはCGO経由**（`modernc.org/sqlite`）。WALモードで動作するため、`.db`ファイルと一緒に`-wal`/`-shm`ファイルが生成される。DBファイル単体をコピーして持ち回らない。
- **設定ファイルの実配置**は開発時カレントディレクトリの`config.json`だが、インストール後は`%APPDATA%\TTI\BridgeGround\config.json`。パス解決ロジック（`internal/config`）はexeからの相対パスと`%APPDATA%`を状況に応じて使い分けている点に注意。
- **Windows Task Scheduler自動起動**をインストーラが登録する（`systemSettings.runOnStartup`）。動作確認時、常駐プロセスが残っていると新しいビルドのポート(8090)を掴めず起動失敗することがある。`tasklist`/`taskkill`で旧プロセスの残存を疑う。
- **`/api/portal/clear-device`・`/api/apps/*`はREADME.mdに未記載**。portal-cms連携時の機器登録解除・外部アプリ登録用の比較的新しいエンドポイントのため、README更新が追いついていない。実装の正は`internal/server/server.go`のルーティングと`internal/portal/manager.go`。
- **ポート8090は同一マシン内の他ツールと衝突しうる**。設定画面またはconfig.jsonの`serverSettings.port`で変更可能。

## Branches & deploy flow

- 作業は `dev` を起点に `hotfix/<内容>` または `feature/<内容>` ブランチを作成して行う（git worktreeで作業ディレクトリを分けるのが基本、`C:\dev\floor-guide-Issue\#000.md`参照）
- 作業完了後はそのブランチをpushしてPRを作成し、`dev`へのマージが完了した時点で対応するIssueをクローズする
- 過去は`dev`に直接作業・pushする運用だったが、複数リポジトリ・複数タスクの並行作業に対応するため上記のブランチ運用に移行した
- `main`はリリース用（`origin/HEAD`は`main`を指す）
- リリースは `bump-version.bat` でバージョン更新 → `build/build-installer.ps1` でインストーラ生成 → 配布、という手動フロー（CI/CDパイプラインは無し）

## Architecture

同一STB上でフロアガイドアプリ（Gido等）とBridge-Groundが1:1でペアになる。Bridge-Groundは(a)店舗データ配信、(b)portal-cmsへの橋渡し、の2役を持つ。

```
上流の店舗API（施設独自、XML） ← internal/sync が定期ポーリング
        │
        ▼
   SQLite（internal/db、WALモード）
        │
        ▼
   internal/server（REST + SSE、:8090、同一コンピュータ内のみで待ち受け）
        │
        ├──► フロアガイドアプリ（Gido/Gido-Touch/Gido-Touch-Mini/Grain-Linkのいずれか1つ、同一STB上）
        │       shops/shop-news/event-news/specials/sales/genres/floors を取得、
        │       /api/events のSSEで更新通知を受信
        │       ↑ /api/apps/register 等でBridge-Groundに自己登録（AppRegistry）
        │
        └──► internal/portal（manager.go）
                registerSelf(): Bridge-Ground自身をAppName="Bridge-Ground"としてportal-cmsに登録
                registerNewApps(): AppRegistryに登録されたフロアガイドアプリを代理でportal-cmsに登録
                sendHeartbeat() / uploadLogsForDevice() / uploadSettingsForDevice() / uploadScreenshotForDevice():
                  自分自身とフロアガイドアプリ両方の分をまとめてportal-cmsへ中継
```

- フロアガイドアプリはportal-cmsの存在を知らない（直接通信しない）。すべてBridge-Ground経由。
- 設定画面はLorca（Chrome/Edgeを外部プロセスとして埋め込み、Go側とJSでやり取り）。ネイティブGUIツールキットは使っていない。
- タスクトレイ常駐（`getlantern/systray`）。ウィンドウを閉じてもプロセスは常駐継続しうる点に注意（README記載の「タスクトレイやバックグラウンドではなく、ウィンドウとして動作」という説明は要検証——systray依存がある以上、実際の挙動をコードで確認してから記述を更新すること）。

## Code conventions

- 標準的なGoの慣習に従う（`gofmt`前提）。このリポジトリ固有のlinter設定は未確認 — 追加で見つかり次第ここに追記する。
- コミットメッセージは変更内容が明確に伝わるものにする。複数ファイルの変更を1コミットにまとめても構わない。

## Project context

Bridge-Groundは「フロアガイド」製品群（Gido/Gido-Touch/Gido-Touch-Mini/Grain-Link/portal-cms）の中核ミドルウェアで、フロアガイドアプリとセットで各STB（Set-Top Box）にインストールされる（施設に1台ではなくSTB台数分稼働する）。他の5リポジトリと合わせて `s-yoshida-33` 配下でホストされている姉妹プロジェクト。ワークフロー運用ルールは `C:\dev\floor-guide-Issue\#000.md` を参照（wonder-screen-frontendプロジェクトで確立した手法を踏襲）。
