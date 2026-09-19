package sync

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestSyncShopNewsDoesNotCollideImagesAcrossNewsForSameShop reproduces the
// reported bug: a shop that posts multiple news items whose photo filenames
// happen to match (a common upstream naming pattern) must not have all of
// them resolve to the same local file. The image path must be keyed by the
// news item's own ShopNewsID, the same way syncEventNews keys by EventID —
// not by the shared ShopID.
func TestSyncShopNewsDoesNotCollideImagesAcrossNewsForSameShop(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AppData", tmpDir) // getBaseFileDir() resolves files under %AppData%

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shopnewslist" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<data>
  <updateDateAll>2026-09-19T00:00:00</updateDateAll>
  <item>
    <shopNewsId>SN001</shopNewsId>
    <shopId>SHOP1</shopId>
    <shopName>Shop One</shopName>
    <shopLogo></shopLogo>
    <shopFloorsName>1F</shopFloorsName>
    <title>News A</title>
    <body>Body A</body>
    <categories></categories>
    <dateStart>2026-09-01</dateStart>
    <dateEnd>2026-09-30</dateEnd>
    <photo1>img/news.jpg</photo1>
    <updateDate>2026-09-01T00:00:00</updateDate>
  </item>
  <item>
    <shopNewsId>SN002</shopNewsId>
    <shopId>SHOP1</shopId>
    <shopName>Shop One</shopName>
    <shopLogo></shopLogo>
    <shopFloorsName>1F</shopFloorsName>
    <title>News B</title>
    <body>Body B</body>
    <categories></categories>
    <dateStart>2026-09-01</dateStart>
    <dateEnd>2026-09-30</dateEnd>
    <photo1>img/news.jpg</photo1>
    <updateDate>2026-09-01T00:00:00</updateDate>
  </item>
</data>`)
			return
		}
		// Any other path is an image download request.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-bytes"))
	}))
	defer server.Close()

	dbMgr := &db.Manager{Path: filepath.Join(tmpDir, "test.db")}
	if err := dbMgr.Connect(); err != nil {
		t.Fatalf("db connect failed: %v", err)
	}
	defer dbMgr.Close()
	if err := dbMgr.InitializeSchema(); err != nil {
		t.Fatalf("schema init failed: %v", err)
	}

	cfg := &config.Config{APISettings: config.APISettings{BaseURL: server.URL + "/"}}
	mgr := NewManager(cfg, dbMgr)

	if _, err := mgr.syncShopNews(); err != nil {
		t.Fatalf("syncShopNews failed: %v", err)
	}

	paths := map[string]string{}
	rows, err := dbMgr.Conn.Query("SELECT shop_news_id, photo1_local_path FROM shop_news ORDER BY shop_news_id")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		paths[id] = path
	}

	pathA, okA := paths["SN001"]
	pathB, okB := paths["SN002"]
	if !okA || !okB {
		t.Fatalf("expected both shop news rows, got: %v", paths)
	}
	if pathA == "" || pathB == "" {
		t.Fatalf("expected non-empty photo1_local_path, got A=%q B=%q", pathA, pathB)
	}
	if pathA == pathB {
		t.Fatalf("photo1_local_path collided across two news items of the same shop with same filename: %q", pathA)
	}

	wantA := filepath.Join(tmpDir, "TTI", "BridgeGround", "files", "news", "SN001", "news.jpg")
	wantB := filepath.Join(tmpDir, "TTI", "BridgeGround", "files", "news", "SN002", "news.jpg")
	if pathA != wantA {
		t.Errorf("SN001 photo1_local_path = %q, want %q", pathA, wantA)
	}
	if pathB != wantB {
		t.Errorf("SN002 photo1_local_path = %q, want %q", pathB, wantB)
	}
}
