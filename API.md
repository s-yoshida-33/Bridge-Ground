# WonderScreen Local API Documentation

WonderScreen は、外部アプリケーションとの連携のためにローカル REST API と Server-Sent Events (SSE) を提供します。

## 概要

- **ベース URL**: `http://localhost:8090` (デフォルト)
- **ポート**: 設定画面で変更可能
- **API モード**: 設定画面で有効/無効を切り替え可能

---

## REST API

### GET /api/timeline

現在のタイムライン全体を取得します。

#### レスポンス例

```json
{
  "count": 3,
  "retrieved_at": "2024-01-15T10:30:00.000Z",
  "current_time": "2024-01-15T10:30:00.000Z",
  "current_time_local": "2024-01-15 19:30:00",
  "status": {
    "initialized": true,
    "headless_mode": false,
    "updating_timeline": false,
    "schedule_id": "abc123",
    "current_item_index": 0,
    "next_item_index": 1,
    "scheduled_switch_time": "2024-01-15T10:31:00.000Z",
    "scheduled_switch_time_local": "19:31:00",
    "seconds_until_switch": 60.0
  },
  "timeline": [
    {
      "timeline_index": 0,
      "start_time": "2024-01-15T10:30:00.000Z",
      "end_time": "2024-01-15T10:31:00.000Z",
      "start_time_local": "19:30:00",
      "end_time_local": "19:31:00",
      "duration_seconds": 60,
      "schedule_id": "abc123",
      "event_id": "event1",
      "program_id": "program1",
      "program_item_index": 0,
      "is_current": true,
      "is_next": false,
      "remaining_seconds": 30.5,
      "elapsed_seconds": 29.5,
      "progress_percent": 49.2,
      "media_count": 1,
      "media": [
        {
          "id": "media-uuid-123",
          "name": "サンプル画像",
          "media_type": "image",
          "filename": "sample.jpg",
          "local_path": "C:/Users/.../assets/media-uuid-123.jpg",
          "duration": 60,
          "layer": {
            "x": 0,
            "y": 0,
            "width": 1920,
            "height": 1080,
            "sequence": 0
          }
        }
      ]
    }
  ]
}
```

#### フィールド説明

| フィールド           | 型     | 説明                          |
| -------------------- | ------ | ----------------------------- |
| `count`              | number | タイムラインアイテムの総数    |
| `retrieved_at`       | string | レスポンス取得時刻 (ISO 8601) |
| `current_time`       | string | 現在時刻 (UTC, ISO 8601)      |
| `current_time_local` | string | 現在時刻 (ローカル)           |
| `status`             | object | 再生状態情報                  |
| `timeline`           | array  | タイムラインアイテムの配列    |

##### status オブジェクト

| フィールド              | 型      | 説明                             |
| ----------------------- | ------- | -------------------------------- |
| `initialized`           | boolean | 初期化完了フラグ                 |
| `headless_mode`         | boolean | ヘッドレスモード有効フラグ       |
| `updating_timeline`     | boolean | タイムライン更新中フラグ         |
| `schedule_id`           | string  | 現在のスケジュール ID            |
| `current_item_index`    | number  | 現在再生中のアイテムインデックス |
| `next_item_index`       | number  | 次のアイテムインデックス         |
| `scheduled_switch_time` | string  | 次回切り替え予定時刻 (ISO 8601)  |
| `seconds_until_switch`  | number  | 次回切り替えまでの秒数           |

---

### GET /api/current-timeline

現在再生中のタイムラインアイテムの詳細情報を取得します。

#### レスポンス例

```json
{
  "retrieved_at": "2024-01-15T10:30:00.000Z",
  "current_timeline": {
    "timeline_index": 0,
    "start_time": "2024-01-15T10:30:00.000Z",
    "end_time": "2024-01-15T10:31:00.000Z",
    "schedule_id": "abc123",
    "data": {
      "schedule_id": "abc123",
      "event_id": "event1",
      "program_item_index": 0,
      "program_item_sequence": 1,
      "program_item_duration": 60,
      "start_time": "2024-01-15T10:30:00.000Z",
      "end_time": "2024-01-15T10:31:00.000Z",
      "generated_at": "2024-01-15T10:00:00.000Z",
      "media_names": ["サンプル画像.jpg"],
      "media_assets": [
        {
          "id": "media-uuid-123",
          "x": 0,
          "y": 0,
          "width": 1920,
          "height": 1080,
          "type": "image",
          "mediaType": "image",
          "sequence": 0,
          "duration": 60,
          "localPath": "C:/Users/.../assets/media-uuid-123.jpg"
        }
      ],
      "media_info": [
        {
          "id": "media-uuid-123",
          "filename": "sample.jpg"
        }
      ],
      "x_program": 0,
      "y_program": 0,
      "width_program": 1920,
      "height_program": 1080,
      "priority": "normal",
      "priority_value": 2,
      "timeline_index": 0
    },
    "start_time_local": "19:30:00",
    "end_time_local": "19:31:00"
  }
}
```

#### 再生中アイテムがない場合

```json
{
  "retrieved_at": "2024-01-15T10:30:00.000Z",
  "current_timeline": null
}
```

---

## Server-Sent Events (SSE)

### GET /api/events

リアルタイムでイベント通知を受信するための SSE エンドポイントです。

#### 接続方法 (JavaScript)

```javascript
const eventSource = new EventSource("http://localhost:8080/api/events");

// 接続時
eventSource.addEventListener("connected", (e) => {
  const data = JSON.parse(e.data);
  console.log("Connected:", data);
});

// コンテンツ切り替え時
eventSource.addEventListener("switch", (e) => {
  const data = JSON.parse(e.data);
  console.log("Content switched:", data);
});

// プリロード完了時
eventSource.addEventListener("preload", (e) => {
  const data = JSON.parse(e.data);
  console.log("Preload completed:", data);
});

// タイムライン更新時
eventSource.addEventListener("update", (e) => {
  const data = JSON.parse(e.data);
  console.log("Timeline updated:", data);
});

// ハートビート
eventSource.addEventListener("heartbeat", (e) => {
  const data = JSON.parse(e.data);
  console.log("Heartbeat:", data);
});

// エラー処理
eventSource.onerror = (e) => {
  console.error("SSE error:", e);
};

// 接続を閉じる
// eventSource.close();
```

---

### イベント一覧

| イベント名    | 説明                                       |
| ------------- | ------------------------------------------ |
| `connected`   | SSE 接続が確立された時                     |
| `switch`      | コンテンツが切り替わった時                 |
| `preload`     | 次のコンテンツのプリロードが完了/失敗した時 |
| `update`      | タイムラインが更新された時                 |
| `heartbeat`   | 30 秒ごとの接続維持用                      |

---

#### connected

SSE 接続が確立された時に送信されます。

```json
{
  "type": "connected",
  "timestamp": "2024-01-15T10:30:00.000Z",
  "current_timeline_index": 0,
  "schedule_id": "abc123"
}
```

---

#### switch

コンテンツが切り替わった時に送信されます。`/api/current-timeline` と同じ形式のデータを含みます。

```json
{
  "type": "switch",
  "timestamp": "2024-01-15T10:31:00.000Z",
  "current_timeline": {
    "timeline_index": 1,
    "start_time": "2024-01-15T10:31:00.000Z",
    "end_time": "2024-01-15T10:32:00.000Z",
    "schedule_id": "abc123",
    "data": {
      "schedule_id": "abc123",
      "event_id": "event1",
      "program_item_index": 1,
      "program_item_sequence": 2,
      "program_item_duration": 60,
      "media_names": ["動画ファイル.mp4"],
      "media_assets": [
        {
          "id": "media-uuid-456",
          "x": 0,
          "y": 0,
          "width": 1920,
          "height": 1080,
          "type": "video",
          "mediaType": "video",
          "sequence": 0,
          "duration": 60,
          "localPath": "C:/Users/.../assets/media-uuid-456.mp4"
        }
      ],
      "media_info": [
        {
          "id": "media-uuid-456",
          "filename": "video.mp4"
        }
      ],
      "x_program": 0,
      "y_program": 0,
      "width_program": 1920,
      "height_program": 1080,
      "priority": "normal",
      "priority_value": 2,
      "timeline_index": 1
    },
    "start_time_local": "19:31:00",
    "end_time_local": "19:32:00"
  }
}
```

---

#### preload

次のコンテンツのプリロードが完了/失敗した時に送信されます。切り替え前に次のコンテンツの情報を取得できます。

```json
{
  "timestamp": "2024-01-15T10:30:55.000Z",
  "success": true,
  "seconds_until_switch": 5.0,
  "scheduled_switch_time": "2024-01-15T10:31:00.000Z",
  "scheduled_switch_time_local": "19:31:00",
  "next_timeline": {
    "timeline_index": 1,
    "start_time": "2024-01-15T10:31:00.000Z",
    "end_time": "2024-01-15T10:32:00.000Z",
    "schedule_id": "abc123",
    "data": {
      "schedule_id": "abc123",
      "event_id": "event1",
      "program_item_index": 1,
      "program_item_sequence": 2,
      "program_item_duration": 60,
      "media_names": ["次の動画.mp4"],
      "media_assets": [
        {
          "id": "media-uuid-789",
          "x": 0,
          "y": 0,
          "width": 1920,
          "height": 1080,
          "type": "video",
          "mediaType": "video",
          "sequence": 0,
          "duration": 60,
          "localPath": "C:/Users/.../assets/media-uuid-789.mp4"
        }
      ],
      "media_info": [
        {
          "id": "media-uuid-789",
          "filename": "next_video.mp4"
        }
      ],
      "x_program": 0,
      "y_program": 0,
      "width_program": 1920,
      "height_program": 1080,
      "priority": "normal",
      "priority_value": 2,
      "timeline_index": 1
    },
    "start_time_local": "19:31:00",
    "end_time_local": "19:32:00"
  }
}
```

| フィールド                   | 型      | 説明                                   |
| ---------------------------- | ------- | -------------------------------------- |
| `success`                    | boolean | プリロード成功フラグ                   |
| `seconds_until_switch`       | number  | 切り替えまでの秒数                     |
| `scheduled_switch_time`      | string  | 切り替え予定時刻 (UTC, ISO 8601)       |
| `scheduled_switch_time_local`| string  | 切り替え予定時刻 (ローカル)            |
| `next_timeline`              | object  | 次のタイムラインアイテムの詳細情報     |

---

#### update

タイムラインが更新された時（スケジュール再取得時など）に送信されます。

```json
{
  "type": "update",
  "timestamp": "2024-01-15T10:30:00.000Z",
  "schedule_id": "abc123",
  "message": "Timeline updated"
}
```

---

#### heartbeat

30 秒ごとに送信される接続維持用のイベントです。

```json
{
  "timestamp": "2024-01-15T10:30:30.000Z",
  "clients": 2
}
```

---

## 使用例

### cURL

```bash
# タイムライン取得
curl http://localhost:8080/api/timeline

# 現在再生中アイテム取得
curl http://localhost:8080/api/current-timeline

# SSE接続
curl -N http://localhost:8080/api/events
```

### Python

```python
import requests
import sseclient

# REST API
response = requests.get('http://localhost:8080/api/current-timeline')
print(response.json())

# SSE
response = requests.get('http://localhost:8080/api/events', stream=True)
client = sseclient.SSEClient(response)
for event in client.events():
    print(f"Event: {event.event}, Data: {event.data}")
```

### C# (HttpClient)

```csharp
using System.Net.Http;
using System.Text.Json;

var client = new HttpClient();

// REST API
var response = await client.GetStringAsync("http://localhost:8080/api/current-timeline");
var data = JsonSerializer.Deserialize<JsonElement>(response);
Console.WriteLine(data);
```

---

## 注意事項

1. **CORS**: 全てのオリジンからのアクセスが許可されています
2. **ローカル接続のみ**: `localhost` でのみ接続可能です
3. **ポート競合**: 指定ポートが使用中の場合、サーバーは起動に失敗します
4. **SSE 接続維持**: クライアント側で再接続ロジックを実装することを推奨します

---

## トラブルシューティング

### 接続できない場合

1. 設定画面で API モードが有効になっているか確認
2. 指定ポートが他のアプリケーションで使用されていないか確認
3. ファイアウォール設定を確認

### SSE 接続が切断される場合

- ネットワーク不安定時に自動的に再接続するロジックを実装してください：

```javascript
function connectSSE() {
  const eventSource = new EventSource("http://localhost:8080/api/events");

  eventSource.onerror = () => {
    eventSource.close();
    // 3秒後に再接続
    setTimeout(connectSSE, 3000);
  };

  eventSource.addEventListener("switch", (e) => {
    // 処理
  });
}

connectSSE();
```
