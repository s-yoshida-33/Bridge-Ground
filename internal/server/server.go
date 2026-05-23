package server

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/logging"
	"bridge-ground/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	Config         *config.Config
	DB             *db.Manager
	Broker         *EventBroker
	Apps           *AppRegistry
	SaveConfigFunc func(cfg config.Config) error
	StartSyncFunc  func() (bool, string)
	RestartFunc    func() error
	AppVersion     string
}

func NewServer(cfg *config.Config, db *db.Manager) *Server {
	return &Server{
		Config: cfg,
		DB:     db,
		Broker: NewEventBroker(),
		Apps:   newAppRegistry(db),
	}
}

func (s *Server) Start() {
	mux := http.NewServeMux()

	// Management API
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/sync/start", s.handleSyncStart)
	mux.HandleFunc("/api/counts", s.handleCounts)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/restart", s.handleRestart)

	// External Apps API
	mux.HandleFunc("/api/apps", s.handleAppsList)
	mux.HandleFunc("/api/apps/ws", s.handleAppWS)
	mux.HandleFunc("/api/apps/", s.handleAppsDetail)

	// Portal CMS API
	mux.HandleFunc("/api/portal/clear-device", s.handlePortalClearDevice)

	// Data API Routes
	mux.HandleFunc("/api/shops", s.handleShopList)
	mux.HandleFunc("/api/shop-news", s.handleShopNewsList)
	mux.HandleFunc("/api/event-news", s.handleEventNewsList)
	mux.HandleFunc("/api/specials", s.handleSpecialList)
	mux.HandleFunc("/api/sales", s.handleSaleList)
	mux.HandleFunc("/api/genres", s.handleGenreList)
	mux.HandleFunc("/api/floors", s.createListEndpoint("floors", "SELECT floor_id, floor_name, sort_order FROM floors"))

	// SSE Endpoint
	mux.Handle("/api/events", s.Broker)

	// Dashboard
	mux.HandleFunc("/", s.handleDashboard)

	// Bridge JS for UI integration
	mux.HandleFunc("/bridge.js", s.handleBridgeJS)

	// Favicon
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, "src/assets/icon.ico")
	})
	mux.HandleFunc("/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		http.ServeFile(w, r, "src/assets/icon.svg")
	})

	port := s.Config.ServerSettings.Port
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	fmt.Printf("HTTP Server running at http://localhost:%d\n", port)
	if err := http.ListenAndServe(addr, s.withCORS(mux)); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func (s *Server) withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) BroadcastEvent(eventType string, data interface{}) {
	s.Broker.Broadcast(eventType, data)
}

func (s *Server) FetchShopList() ([]models.ShopItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT
		shop_id, shop_name, shop_name_kana, shop_name_english,
		COALESCE(shop_name_china_cn,''), COALESCE(shop_name_china_tw,''), COALESCE(shop_name_korea,''), COALESCE(shop_name_france,''), COALESCE(shop_name_vietnam,''), COALESCE(shop_name_thai,''),
		COALESCE(abbr,''), COALESCE(web_status,''), searches,
		genre, genre_sub, genre_sub_english,
		genre_memo, genre_memo_english,
		COALESCE(genre_memo_china_cn,''), COALESCE(genre_memo_china_tw,''), COALESCE(genre_memo_korea,''), COALESCE(genre_memo_france,''), COALESCE(genre_memo_vietnam,''), COALESCE(genre_memo_thai,''),
		group_id, COALESCE(tenant_code,''), tel, COALESCE(user_url,''),
		COALESCE(floor,''), floors, area, area_sub, number,
		COALESCE(open_year,''), COALESCE(open_month,''), COALESCE(open_day,''), close_flg, COALESCE(pub_start,''), COALESCE(pub_end,''), open_time, description,
		COALESCE(qr,''), COALESCE(food_class,''), COALESCE(seats,''), COALESCE(smoking,''), COALESCE(reservation,''),
		COALESCE(lunch_menu,''), COALESCE(dinner_menu,''), COALESCE(take_out,''), COALESCE(childrens_menu,''), COALESCE(baby_seat,''), COALESCE(alcohol,''), COALESCE(options,''),
		photo1, photo1_local_path,
		photo2, photo2_local_path,
		shop_logo, shop_logo_local_path,
		COALESCE(photo1_thumb,''), COALESCE(photo1_thumb_150x150,''), COALESCE(photo1_thumb_640x640,''), COALESCE(photo1_thumb_w320,''),
		photo1_thumb_w640, photo1_thumb_w640_local_path,
		COALESCE(photo2_thumb,''), COALESCE(photo2_thumb_150x150,''), COALESCE(photo2_thumb_640x640,''), COALESCE(photo2_thumb_w320,''),
		photo2_thumb_w640, photo2_thumb_w640_local_path,
		COALESCE(shop_logo_thumb,''), COALESCE(shop_logo_thumb_150x150,''),
		shop_logo_thumb_640x640, shop_logo_thumb_640x640_local_path,
		COALESCE(shop_logo_thumb_w320,''),
		shop_logo_thumb_w640, shop_logo_thumb_w640_local_path,
		update_date
		FROM shops`

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var result []models.ShopItem

	for rows.Next() {
		var item models.ShopItem
		var (
			shopId, shopName, shopNameKana, shopNameEnglish,
			shopNameChinaCN, shopNameChinaTW, shopNameKorea, shopNameFrance, shopNameVietnam, shopNameThai,
			abbr, webStatus, searches,
			genre, genreSub, genreSubEnglish,
			genreMemo, genreMemoEnglish,
			genreMemoChinaCN, genreMemoChinaTW, genreMemoKorea, genreMemoFrance, genreMemoVietnam, genreMemoThai,
			groupId, tenantCode, tel, userUrl,
			floor, floors, area, areaSub, number,
			openYear, openMonth, openDay, closeFlg, pubStart, pubEnd, openTime, description,
			qr, foodClass, seats, smoking, reservation,
			lunchMenu, dinnerMenu, takeOut, childrensMenu, babySeat, alcohol, options,
			photo1, photo1LocalPath,
			photo2, photo2LocalPath,
			shopLogo, shopLogoLocalPath,
			photo1Thumb, photo1Thumb150x150, photo1Thumb640x640, photo1ThumbW320,
			photo1ThumbW640, photo1ThumbW640LocalPath,
			photo2Thumb, photo2Thumb150x150, photo2Thumb640x640, photo2ThumbW320,
			photo2ThumbW640, photo2ThumbW640LocalPath,
			shopLogoThumb, shopLogoThumb150x150,
			shopLogoThumb640x640, shopLogoThumb640x640LocalPath,
			shopLogoThumbW320,
			shopLogoThumbW640, shopLogoThumbW640LocalPath,
			updateDate *string
		)

		if err := rows.Scan(
			&shopId, &shopName, &shopNameKana, &shopNameEnglish,
			&shopNameChinaCN, &shopNameChinaTW, &shopNameKorea, &shopNameFrance, &shopNameVietnam, &shopNameThai,
			&abbr, &webStatus, &searches,
			&genre, &genreSub, &genreSubEnglish,
			&genreMemo, &genreMemoEnglish,
			&genreMemoChinaCN, &genreMemoChinaTW, &genreMemoKorea, &genreMemoFrance, &genreMemoVietnam, &genreMemoThai,
			&groupId, &tenantCode, &tel, &userUrl,
			&floor, &floors, &area, &areaSub, &number,
			&openYear, &openMonth, &openDay, &closeFlg, &pubStart, &pubEnd, &openTime, &description,
			&qr, &foodClass, &seats, &smoking, &reservation,
			&lunchMenu, &dinnerMenu, &takeOut, &childrensMenu, &babySeat, &alcohol, &options,
			&photo1, &photo1LocalPath,
			&photo2, &photo2LocalPath,
			&shopLogo, &shopLogoLocalPath,
			&photo1Thumb, &photo1Thumb150x150, &photo1Thumb640x640, &photo1ThumbW320,
			&photo1ThumbW640, &photo1ThumbW640LocalPath,
			&photo2Thumb, &photo2Thumb150x150, &photo2Thumb640x640, &photo2ThumbW320,
			&photo2ThumbW640, &photo2ThumbW640LocalPath,
			&shopLogoThumb, &shopLogoThumb150x150,
			&shopLogoThumb640x640, &shopLogoThumb640x640LocalPath,
			&shopLogoThumbW320,
			&shopLogoThumbW640, &shopLogoThumbW640LocalPath,
			&updateDate,
		); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		sv := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.ShopID = sv(shopId)
		item.ShopName = sv(shopName)
		item.ShopNameKana = sv(shopNameKana)
		item.ShopNameEnglish = sv(shopNameEnglish)
		item.ShopNameChinaCN = sv(shopNameChinaCN)
		item.ShopNameChinaTW = sv(shopNameChinaTW)
		item.ShopNameKorea = sv(shopNameKorea)
		item.ShopNameFrance = sv(shopNameFrance)
		item.ShopNameVietnam = sv(shopNameVietnam)
		item.ShopNameThai = sv(shopNameThai)
		item.Abbr = sv(abbr)
		item.WebStatus = sv(webStatus)
		item.Searches = sv(searches)
		item.Genre = sv(genre)
		item.GenreSub = sv(genreSub)
		item.GenreSubEnglish = sv(genreSubEnglish)
		item.GenreMemo = sv(genreMemo)
		item.GenreMemoEnglish = sv(genreMemoEnglish)
		item.GenreMemoChinaCN = sv(genreMemoChinaCN)
		item.GenreMemoChinaTW = sv(genreMemoChinaTW)
		item.GenreMemoKorea = sv(genreMemoKorea)
		item.GenreMemoFrance = sv(genreMemoFrance)
		item.GenreMemoVietnam = sv(genreMemoVietnam)
		item.GenreMemoThai = sv(genreMemoThai)
		item.GroupID = sv(groupId)
		item.TenantCode = sv(tenantCode)
		item.Tel = sv(tel)
		item.UserUrl = sv(userUrl)
		item.Floor = sv(floor)
		item.Floors = sv(floors)
		item.Area = sv(area)
		item.AreaSub = sv(areaSub)
		item.Number = sv(number)
		item.OpenYear = sv(openYear)
		item.OpenMonth = sv(openMonth)
		item.OpenDay = sv(openDay)
		item.CloseFlg = sv(closeFlg)
		item.PubStart = sv(pubStart)
		item.PubEnd = sv(pubEnd)
		item.OpenTime = sv(openTime)
		item.Description = sv(description)
		item.Qr = sv(qr)
		item.FoodClass = sv(foodClass)
		item.Seats = sv(seats)
		item.Smoking = sv(smoking)
		item.Reservation = sv(reservation)
		item.LunchMenu = sv(lunchMenu)
		item.DinnerMenu = sv(dinnerMenu)
		item.TakeOut = sv(takeOut)
		item.ChildrensMenu = sv(childrensMenu)
		item.BabySeat = sv(babySeat)
		item.Alcohol = sv(alcohol)
		item.Options = sv(options)
		item.Photo1 = sv(photo1)
		item.Photo1LocalPath = sv(photo1LocalPath)
		item.Photo2 = sv(photo2)
		item.Photo2LocalPath = sv(photo2LocalPath)
		item.ShopLogo = sv(shopLogo)
		item.ShopLogoLocalPath = sv(shopLogoLocalPath)
		item.Photo1Thumb = sv(photo1Thumb)
		item.Photo1Thumb150x150 = sv(photo1Thumb150x150)
		item.Photo1Thumb640x640 = sv(photo1Thumb640x640)
		item.Photo1ThumbW320 = sv(photo1ThumbW320)
		item.Photo1ThumbW640 = sv(photo1ThumbW640)
		item.Photo1ThumbW640LocalPath = sv(photo1ThumbW640LocalPath)
		item.Photo2Thumb = sv(photo2Thumb)
		item.Photo2Thumb150x150 = sv(photo2Thumb150x150)
		item.Photo2Thumb640x640 = sv(photo2Thumb640x640)
		item.Photo2ThumbW320 = sv(photo2ThumbW320)
		item.Photo2ThumbW640 = sv(photo2ThumbW640)
		item.Photo2ThumbW640LocalPath = sv(photo2ThumbW640LocalPath)
		item.ShopLogoThumb = sv(shopLogoThumb)
		item.ShopLogoThumb150x150 = sv(shopLogoThumb150x150)
		item.ShopLogoThumb640x640 = sv(shopLogoThumb640x640)
		item.ShopLogoThumb640x640LocalPath = sv(shopLogoThumb640x640LocalPath)
		item.ShopLogoThumbW320 = sv(shopLogoThumbW320)
		item.ShopLogoThumbW640 = sv(shopLogoThumbW640)
		item.ShopLogoThumbW640LocalPath = sv(shopLogoThumbW640LocalPath)
		item.UpdateDate = sv(updateDate)

		result = append(result, item)
	}
	return result, nil
}

func (s *Server) FetchGenericList(query string) ([]map[string]interface{}, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	var result []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			rowMap[col] = v
		}
		result = append(result, rowMap)
	}
	return result, nil
}

func (s *Server) createListEndpoint(tableName string, query string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := s.FetchGenericList(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "")
		encoder.Encode(result)
	}
}

func (s *Server) handleShopList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchShopList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	encoder.Encode(result)
}

func (s *Server) FetchEventNews() ([]models.EventNewsItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	query := `SELECT 
		event_id, title, body, categories, date_start, date_end, display_end, venues,
		photo1, photo1_remote_url, photo1_local_path, update_date
		FROM event_news`
	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()
	var result []models.EventNewsItem
	for rows.Next() {
		var item models.EventNewsItem
		var (eventId, title, body, categories, dateStart, dateEnd, displayEnd, venues, photo1, photo1RemoteUrl, photo1LocalPath, updateDate *string)
		if err := rows.Scan(&eventId, &title, &body, &categories, &dateStart, &dateEnd, &displayEnd, &venues, &photo1, &photo1RemoteUrl, &photo1LocalPath, &updateDate); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		s := func(ptr *string) string {
			if ptr == nil { return "" }
			return *ptr
		}
		item.EventID = s(eventId); item.Title = s(title); item.Body = s(body)
		item.Categories = s(categories); item.DateStart = s(dateStart); item.DateEnd = s(dateEnd)
		item.DisplayEnd = s(displayEnd); item.Venues = s(venues); item.Photo1 = s(photo1)
		item.Photo1RemoteURL = s(photo1RemoteUrl); item.Photo1LocalPath = s(photo1LocalPath)
		item.UpdateDate = s(updateDate)
		result = append(result, item)
	}
	return result, nil
}

func (s *Server) FetchShopNews() ([]models.ShopNewsItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	query := `SELECT 
		shop_news_id, shop_id, shop_name, shop_logo, shop_floors_name, title, body, categories,
		date_start, date_end, photo1, photo1_remote_url, photo1_local_path,
		shop_logo_remote_url, shop_logo_local_path, update_date
		FROM shop_news`
	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()
	var result []models.ShopNewsItem
	for rows.Next() {
		var item models.ShopNewsItem
		var (shopNewsId, shopId, shopName, shopLogo, shopFloorsName, title, body, categories, dateStart, dateEnd, photo1, photo1RemoteUrl, photo1LocalPath, shopLogoRemoteUrl, shopLogoLocalPath, updateDate *string)
		if err := rows.Scan(&shopNewsId, &shopId, &shopName, &shopLogo, &shopFloorsName, &title, &body, &categories, &dateStart, &dateEnd, &photo1, &photo1RemoteUrl, &photo1LocalPath, &shopLogoRemoteUrl, &shopLogoLocalPath, &updateDate); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		s := func(ptr *string) string {
			if ptr == nil { return "" }
			return *ptr
		}
		item.ShopNewsID = s(shopNewsId); item.ShopID = s(shopId); item.ShopName = s(shopName)
		item.ShopLogo = s(shopLogo); item.ShopFloorsName = s(shopFloorsName)
		item.Title = s(title); item.Body = s(body); item.Categories = s(categories)
		item.DateStart = s(dateStart); item.DateEnd = s(dateEnd); item.Photo1 = s(photo1)
		item.Photo1RemoteURL = s(photo1RemoteUrl); item.Photo1LocalPath = s(photo1LocalPath)
		item.ShopLogoRemoteURL = s(shopLogoRemoteUrl); item.ShopLogoLocalPath = s(shopLogoLocalPath)
		item.UpdateDate = s(updateDate)
		result = append(result, item)
	}
	return result, nil
}

func (s *Server) FetchSpecials() ([]models.SpecialItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	query := `SELECT
		special_id, COALESCE(special_title_id,''), special_title,
		title, COALESCE(sub_title,''), COALESCE(category_id,''), category_name, special_sub_body,
		shop_id, shop_name, COALESCE(genre_memo,''), COALESCE(shop_logo,''), COALESCE(shop_logo_local_path,''),
		COALESCE(shop_floor_name,''), COALESCE(shop_floors_name,''), COALESCE(venue,''),
		COALESCE(pub_start,''), COALESCE(pub_end,''), update_date,
		COALESCE(special_image_local_path,''), COALESCE(special_image2_local_path,'')
		FROM specials`
	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()
	var result []models.SpecialItem
	for rows.Next() {
		var item models.SpecialItem
		var (specialId, specialTitleId, specialTitle, title, subTitle, categoryId, categoryName, specialSubBody, shopId, shopName, genreMemo, shopLogo, shopLogoLocalPath, shopFloorName, shopFloorsName, venue, pubStart, pubEnd, updateDate, specialImageLocalPath, specialImage2LocalPath *string)
		if err := rows.Scan(&specialId, &specialTitleId, &specialTitle, &title, &subTitle, &categoryId, &categoryName, &specialSubBody, &shopId, &shopName, &genreMemo, &shopLogo, &shopLogoLocalPath, &shopFloorName, &shopFloorsName, &venue, &pubStart, &pubEnd, &updateDate, &specialImageLocalPath, &specialImage2LocalPath); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		sv := func(ptr *string) string {
			if ptr == nil { return "" }
			return *ptr
		}
		item.SpecialID = sv(specialId); item.SpecialTitleID = sv(specialTitleId); item.SpecialTitle = sv(specialTitle)
		item.Title = sv(title); item.SubTitle = sv(subTitle); item.CategoryID = sv(categoryId)
		item.CategoryName = sv(categoryName); item.SpecialSubBody = sv(specialSubBody)
		item.ShopID = sv(shopId); item.ShopName = sv(shopName); item.GenreMemo = sv(genreMemo)
		item.ShopLogo = sv(shopLogo); item.ShopLogoLocalPath = sv(shopLogoLocalPath)
		item.ShopFloorName = sv(shopFloorName); item.ShopFloorsName = sv(shopFloorsName)
		item.Venue = sv(venue); item.PubStart = sv(pubStart); item.PubEnd = sv(pubEnd)
		item.UpdateDate = sv(updateDate); item.SpecialImageLocalPath = sv(specialImageLocalPath)
		result = append(result, item)
	}
	return result, nil
}

func (s *Server) FetchSales() ([]models.SaleItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	query := `SELECT
		sale_id, COALESCE(sale_title_id,''), COALESCE(sale_title,''),
		COALESCE(sale_body,''), COALESCE(shop_id,''), COALESCE(shop_name,''),
		COALESCE(genre,''), COALESCE(genre_memo,''),
		COALESCE(shop_logo,''), COALESCE(shop_logo_local_path,''),
		COALESCE(shop_floor_name,''), COALESCE(shop_floors_name,''),
		COALESCE(area,''), COALESCE(area_sub,''),
		COALESCE(sale_title_image,''), COALESCE(sale_title_image_local_path,''),
		COALESCE(pub_start,''), COALESCE(pub_end,''), update_date
		FROM sales`
	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()
	var result []models.SaleItem
	for rows.Next() {
		var item models.SaleItem
		var (saleId, saleTitleId, saleTitle, saleBody, shopId, shopName, genre, genreMemo, shopLogo, shopLogoLocalPath, shopFloorName, shopFloorsName, area, areaSub, saleTitleImage, saleTitleImageLocalPath, pubStart, pubEnd, updateDate *string)
		if err := rows.Scan(&saleId, &saleTitleId, &saleTitle, &saleBody, &shopId, &shopName, &genre, &genreMemo, &shopLogo, &shopLogoLocalPath, &shopFloorName, &shopFloorsName, &area, &areaSub, &saleTitleImage, &saleTitleImageLocalPath, &pubStart, &pubEnd, &updateDate); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		sv := func(ptr *string) string {
			if ptr == nil { return "" }
			return *ptr
		}
		item.SaleID = sv(saleId); item.SaleTitleID = sv(saleTitleId); item.SaleTitle = sv(saleTitle)
		item.SaleBody = sv(saleBody); item.ShopID = sv(shopId); item.ShopName = sv(shopName)
		item.Genre = sv(genre); item.GenreMemo = sv(genreMemo)
		item.ShopLogo = sv(shopLogo); item.ShopLogoLocalPath = sv(shopLogoLocalPath)
		item.ShopFloorName = sv(shopFloorName); item.ShopFloorsName = sv(shopFloorsName)
		item.Area = sv(area); item.AreaSub = sv(areaSub)
		item.SaleTitleImage = sv(saleTitleImage); item.SaleTitleImageLocalPath = sv(saleTitleImageLocalPath)
		item.PubStart = sv(pubStart); item.PubEnd = sv(pubEnd); item.UpdateDate = sv(updateDate)
		result = append(result, item)
	}
	return result, nil
}

func (s *Server) FetchGenres() ([]models.GenreItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	query := `SELECT genre_id, genre_name, genre_slug FROM genres`
	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()
	var result []models.GenreItem
	for rows.Next() {
		var item models.GenreItem
		var genreId, genreName, genreSlug *string
		if err := rows.Scan(&genreId, &genreName, &genreSlug); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		s := func(ptr *string) string {
			if ptr == nil { return "" }
			return *ptr
		}
		item.GenreID = s(genreId); item.GenreName = s(genreName); item.GenreSlug = s(genreSlug)
		result = append(result, item)
	}
	return result, nil
}

func (s *Server) handleEventNewsList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchEventNews()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	s.respondJSON(w, result)
}
func (s *Server) handleShopNewsList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchShopNews()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	s.respondJSON(w, result)
}
func (s *Server) handleSpecialList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchSpecials()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	s.respondJSON(w, result)
}
func (s *Server) handleSaleList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchSales()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	s.respondJSON(w, result)
}
func (s *Server) handleGenreList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchGenres()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	s.respondJSON(w, result)
}

func (s *Server) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		http.ServeFile(w, r, "src/ui/index.html")
		return
	}
	fs := http.FileServer(http.Dir("./src/ui"))
	fs.ServeHTTP(w, r)
}

// configResponse is Config with password and device tokens replaced by sentinel flags.
type configResponse struct {
	config.Config
	APISettings    apiSettingsResponse    `json:"apiSettings"`
	PortalSettings portalSettingsResponse `json:"portalSettings"`
}
type apiSettingsResponse struct {
	BaseURL     string `json:"baseUrl"`
	APIKey      string `json:"apiKey"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	PasswordSet bool   `json:"passwordSet"`
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		resp := configResponse{Config: *s.Config}
		resp.APISettings = apiSettingsResponse{
			BaseURL:     s.Config.APISettings.BaseURL,
			APIKey:      s.Config.APISettings.APIKey,
			Username:    s.Config.APISettings.Username,
			Password:    "",
			PasswordSet: s.Config.APISettings.Password != "",
		}
		resp.PortalSettings = maskedPortalSettings(s.Config.PortalSettings)
		json.NewEncoder(w).Encode(resp)

	case http.MethodPost:
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if newCfg.APISettings.Password == "" {
			newCfg.APISettings.Password = s.Config.APISettings.Password
		}
		newCfg.PortalSettings.Devices = mergePortalDevices(
			newCfg.PortalSettings.Devices,
			s.Config.PortalSettings.Devices,
		)
		if s.SaveConfigFunc != nil {
			if err := s.SaveConfigFunc(newCfg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			*s.Config = newCfg
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSyncStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.StartSyncFunc == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "sync not available"})
		return
	}
	ok, msg := s.StartSyncFunc()
	json.NewEncoder(w).Encode(map[string]interface{}{"success": ok, "message": msg})
}

func (s *Server) handleCounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	counts, err := s.DB.GetDataCounts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(counts)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	today := time.Now().Format("2006-01-02")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" { from = today }
	if to == "" { to = today }
	if to > today { to = today }
	entries := logging.GetEntriesForRange(from, to)
	if entries == nil { entries = []logging.LogEntry{} }
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"version": s.AppVersion})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.RestartFunc == nil {
		json.NewEncoder(w).Encode(map[string]bool{"success": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
	go func() {
		time.Sleep(200 * time.Millisecond)
		s.RestartFunc()
	}()
}

func (s *Server) handleBridgeJS(w http.ResponseWriter, r *http.Request) {
	js := `
		(function() {
			const _isLorca = typeof window.go_getConfig === 'function';
			const _get = (url) => fetch(url).then(r => r.json());
			const _post = (url, body) => fetch(url, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			}).then(r => r.ok ? r.json() : r.text().then(t => Promise.reject(new Error(t || r.statusText))));

			window.bridgeApi = {
				getConfig: () => _isLorca
					? window.go_getConfig()
					: _get('/api/config').then(cfg => {
						cfg.apiSettings = cfg.apiSettings || {};
						return cfg;
					}),
				getAppVersion: () => _isLorca
					? window.go_getAppVersion()
					: _get('/api/version').then(d => d.version),
				saveConfig: (cfg) => _isLorca
					? window.go_saveConfig(cfg)
					: _post('/api/config', cfg),
				startManualSync: () => _isLorca
					? window.go_startManualSync()
					: _post('/api/sync/start', {}),
				getDataCounts: () => _isLorca
					? window.go_getDataCounts()
					: _get('/api/counts'),
				getLogs: (from, to) => {
					const params = [];
					if (from) params.push('from=' + from);
					if (to)   params.push('to='   + to);
					return _get('/api/logs' + (params.length ? '?' + params.join('&') : ''));
				},
				getApps: () => _get('/api/apps'),
				getApp:  (id) => _get('/api/apps/' + id),
				clearPortalDevice: (appName, hostname) => _isLorca
					? window.go_clearPortalDevice(appName, hostname)
					: _post('/api/portal/clear-device', {appName, hostname}),
				restartApp: () => _isLorca
					? window.go_restartApp()
					: _post('/api/restart', {}),
				onSyncProgress: function(callback) {
					window._syncProgressCallback = callback;
					if (!_isLorca && !window._syncProgressSSE) {
						window._syncProgressSSE = new EventSource('/api/events');
						window._syncProgressSSE.addEventListener('sync_progress', function(e) {
							var data = JSON.parse(e.data);
							if (window._syncProgressCallback) window._syncProgressCallback(data);
						});
					}
				}
			};

			window.dispatchSyncProgress = function(data) {
				if (window._syncProgressCallback) window._syncProgressCallback(data);
			};
		})();
	`
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(js))
}
