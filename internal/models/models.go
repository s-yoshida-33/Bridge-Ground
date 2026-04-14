package models

import "encoding/xml"

// ShopItem represents a shop from the XML feed
type ShopItem struct {
	XMLName         xml.Name `xml:"item" json:"-"`
	ShopID          string   `xml:"shopId" db:"shop_id" json:"shopId"`
	ShopName        string   `xml:"shopName" db:"shop_name" json:"shopName"`
	ShopNameKana    string   `xml:"shopNameKana" db:"shop_name_kana" json:"shopNameKana"`
	ShopNameEnglish string   `xml:"shopNameEnglish" db:"shop_name_english" json:"shopNameEnglish"`
	ShopNameChinaCN string   `xml:"shopNameChinaCN" db:"shop_name_china_cn" json:"shopNameChinaCN"`
	ShopNameChinaTW string   `xml:"shopNameChinaTW" db:"shop_name_china_tw" json:"shopNameChinaTW"`
	ShopNameKorea   string   `xml:"shopNameKorea" db:"shop_name_korea" json:"shopNameKorea"`
	ShopNameFrance  string   `xml:"shopNameFrance" db:"shop_name_france" json:"shopNameFrance"`
	ShopNameVietnam string   `xml:"shopNameVietnam" db:"shop_name_vietnam" json:"shopNameVietnam"`
	ShopNameThai    string   `xml:"shopNameThai" db:"shop_name_thai" json:"shopNameThai"`
	Abbr            string   `xml:"abbr" db:"abbr" json:"abbr"`
	WebStatus       string   `xml:"webStatus" db:"web_status" json:"webStatus"`
	Searches        string   `xml:"searches" db:"searches" json:"searches"`
	Genre           string   `xml:"genre" db:"genre" json:"genre"`
	GenreSub        string   `xml:"genreSub" db:"genre_sub" json:"genreSub"`
	GenreSubEnglish string   `xml:"genreSubEnglish" db:"genre_sub_english" json:"genreSubEnglish"`
	GenreMemo        string  `xml:"genreMemo" db:"genre_memo" json:"genreMemo"`
	GenreMemoEnglish string  `xml:"genreMemoEnglish" db:"genre_memo_english" json:"genreMemoEnglish"`
	GenreMemoChinaCN string  `xml:"genreMemoChinaCN" db:"genre_memo_china_cn" json:"genreMemoChinaCN"`
	GenreMemoChinaTW string  `xml:"genreMemoChinaTw" db:"genre_memo_china_tw" json:"genreMemoChinaTW"`
	GenreMemoKorea   string  `xml:"genreMemoKorea" db:"genre_memo_korea" json:"genreMemoKorea"`
	GenreMemoFrance  string  `xml:"genreMemoFrance" db:"genre_memo_france" json:"genreMemoFrance"`
	GenreMemoVietnam string  `xml:"genreMemoVietnam" db:"genre_memo_vietnam" json:"genreMemoVietnam"`
	GenreMemoThai    string  `xml:"genreMemoThai" db:"genre_memo_thai" json:"genreMemoThai"`
	GroupID          string  `xml:"groupId" db:"group_id" json:"groupId"`
	TenantCode       string  `xml:"tenantCode" db:"tenant_code" json:"tenantCode"`
	Tel              string  `xml:"tel" db:"tel" json:"tel"`
	UserUrl          string  `xml:"userUrl" db:"user_url" json:"userUrl"`
	Floor            string  `xml:"floor" db:"floor" json:"floor"`
	Floors           string  `xml:"floors" db:"floors" json:"floors"`
	Area             string  `xml:"area" db:"area" json:"area"`
	AreaSub          string  `xml:"areaSub" db:"area_sub" json:"areaSub"`
	Number           string  `xml:"number" db:"number" json:"number"`
	OpenYear         string  `xml:"openYear" db:"open_year" json:"openYear"`
	OpenMonth        string  `xml:"openMonth" db:"open_month" json:"openMonth"`
	OpenDay          string  `xml:"openDay" db:"open_day" json:"openDay"`
	CloseFlg         string  `xml:"closeFlg" db:"close_flg" json:"closeFlg"`
	PubStart         string  `xml:"pubStart" db:"pub_start" json:"pubStart"`
	PubEnd           string  `xml:"pubEnd" db:"pub_end" json:"pubEnd"`
	OpenTime         string  `xml:"openTime" db:"open_time" json:"openTime"`
	Description      string  `xml:"description" db:"description" json:"description"`
	Qr               string  `xml:"qr" db:"qr" json:"qr"`
	FoodClass        string  `xml:"foodClass" db:"food_class" json:"foodClass"`
	Seats            string  `xml:"seats" db:"seats" json:"seats"`
	Smoking          string  `xml:"smoking" db:"smoking" json:"smoking"`
	Reservation      string  `xml:"reservation" db:"reservation" json:"reservation"`
	LunchMenu        string  `xml:"lunchMenu" db:"lunch_menu" json:"lunchMenu"`
	DinnerMenu       string  `xml:"dinnerMenu" db:"dinner_menu" json:"dinnerMenu"`
	TakeOut          string  `xml:"takeOut" db:"take_out" json:"takeOut"`
	ChildrensMenu    string  `xml:"childrensMenu" db:"childrens_menu" json:"childrensMenu"`
	BabySeat         string  `xml:"babySeat" db:"baby_seat" json:"babySeat"`
	Alcohol          string  `xml:"alcohol" db:"alcohol" json:"alcohol"`
	Options          string  `xml:"options" db:"options" json:"options"`

	// Images (Original values from XML)
	Photo1          string `xml:"photo1" db:"photo1" json:"photo1"`
	Photo1LocalPath string `db:"photo1_local_path" json:"photo1LocalPath"`

	Photo2          string `xml:"photo2" db:"photo2" json:"photo2"`
	Photo2LocalPath string `db:"photo2_local_path" json:"photo2LocalPath"`

	ShopLogo          string `xml:"shopLogo" db:"shop_logo" json:"shopLogo"`
	ShopLogoLocalPath string `db:"shop_logo_local_path" json:"shopLogoLocalPath"`

	// Thumbnail images (W640 variants are downloaded locally; others are URL-only)
	Photo1Thumb          string `xml:"photo1Thumb" db:"photo1_thumb" json:"photo1Thumb"`
	Photo1Thumb150x150   string `xml:"photo1Thumb150x150" db:"photo1_thumb_150x150" json:"photo1Thumb150x150"`
	Photo1Thumb640x640   string `xml:"photo1Thumb640x640" db:"photo1_thumb_640x640" json:"photo1Thumb640x640"`
	Photo1ThumbW320      string `xml:"photo1ThumbW320" db:"photo1_thumb_w320" json:"photo1ThumbW320"`
	Photo1ThumbW640      string `xml:"photo1ThumbW640" db:"photo1_thumb_w640" json:"photo1ThumbW640"`
	Photo1ThumbW640LocalPath string `db:"photo1_thumb_w640_local_path" json:"photo1ThumbW640LocalPath"`

	Photo2Thumb          string `xml:"photo2Thumb" db:"photo2_thumb" json:"photo2Thumb"`
	Photo2Thumb150x150   string `xml:"photo2Thumb150x150" db:"photo2_thumb_150x150" json:"photo2Thumb150x150"`
	Photo2Thumb640x640   string `xml:"photo2Thumb640x640" db:"photo2_thumb_640x640" json:"photo2Thumb640x640"`
	Photo2ThumbW320      string `xml:"photo2ThumbW320" db:"photo2_thumb_w320" json:"photo2ThumbW320"`
	Photo2ThumbW640      string `xml:"photo2ThumbW640" db:"photo2_thumb_w640" json:"photo2ThumbW640"`
	Photo2ThumbW640LocalPath string `db:"photo2_thumb_w640_local_path" json:"photo2ThumbW640LocalPath"`

	ShopLogoThumb          string `xml:"shopLogoThumb" db:"shop_logo_thumb" json:"shopLogoThumb"`
	ShopLogoThumb150x150   string `xml:"shopLogoThumb150x150" db:"shop_logo_thumb_150x150" json:"shopLogoThumb150x150"`
	ShopLogoThumb640x640   string `xml:"shopLogoThumb640x640" db:"shop_logo_thumb_640x640" json:"shopLogoThumb640x640"`
	ShopLogoThumb640x640LocalPath string `db:"shop_logo_thumb_640x640_local_path" json:"shopLogoThumb640x640LocalPath"`
	ShopLogoThumbW320      string `xml:"shopLogoThumbW320" db:"shop_logo_thumb_w320" json:"shopLogoThumbW320"`
	ShopLogoThumbW640      string `xml:"shopLogoThumbW640" db:"shop_logo_thumb_w640" json:"shopLogoThumbW640"`
	ShopLogoThumbW640LocalPath string `db:"shop_logo_thumb_w640_local_path" json:"shopLogoThumbW640LocalPath"`

	UpdateDate string `xml:"updateDate" db:"update_date" json:"updateDate"`

	// Internal fields for remote URL tracking (not exposed in JSON response)
	Photo1RemoteURL              string `db:"photo1_remote_url" json:"-"`
	Photo2RemoteURL              string `db:"photo2_remote_url" json:"-"`
	Photo1ThumbW640RemoteURL     string `db:"photo1_thumb_w640_remote_url" json:"-"`
	Photo2ThumbW640RemoteURL     string `db:"photo2_thumb_w640_remote_url" json:"-"`
	ShopLogoRemoteURL            string `db:"shop_logo_remote_url" json:"-"`
	ShopLogoThumb640x640RemoteURL string `db:"shop_logo_thumb_640x640_remote_url" json:"-"`
	ShopLogoThumbW640RemoteURL   string `db:"shop_logo_thumb_w640_remote_url" json:"-"`
}

// CDATAString handles cases where data might be wrapped in CDATA or have whitespace
type CDATAString struct {
	Value string `xml:",cdata"`
}

// EventNewsItem represents an event news item
type EventNewsItem struct {
	XMLName         xml.Name `xml:"item" json:"-"`
	EventID         string   `xml:"eventId" db:"event_id" json:"eventId"`
	Title           string   `xml:"title" db:"title" json:"title"`
	Body            string   `xml:"body" db:"body" json:"body"`
	Categories      string   `xml:"categories" db:"categories" json:"categories"`
	DateStart       string   `xml:"dateStart" db:"date_start" json:"dateStart"`
	DateEnd         string   `xml:"dateEnd" db:"date_end" json:"dateEnd"`
	DisplayEnd      string   `xml:"displayEnd" db:"display_end" json:"displayEnd"`
	Venues          string   `xml:"venues" db:"venues" json:"venues"`
	
	Photo1          string   `xml:"photo1" db:"photo1" json:"photo1"`
	
	Photo1RemoteURL string   `db:"photo1_remote_url" json:"-"`
	Photo1LocalPath string   `db:"photo1_local_path" json:"photo1LocalPath"`
	
	UpdateDate      string   `xml:"updateDate" db:"update_date" json:"updateDate"`
}

// ShopNewsItem represents a shop news item
type ShopNewsItem struct {
	XMLName         xml.Name `xml:"item" json:"-"`
	ShopNewsID      string   `xml:"shopNewsId" db:"shop_news_id" json:"shopNewsId"`
	ShopID          string   `xml:"shopId" db:"shop_id" json:"shopId"`
	ShopName        string   `xml:"shopName" db:"shop_name" json:"shopName"`
	ShopLogo        string   `xml:"shopLogo" db:"shop_logo" json:"shopLogo"`
	ShopFloorsName  string   `xml:"shopFloorsName" db:"shop_floors_name" json:"shopFloorsName"`
	Title           string   `xml:"title" db:"title" json:"title"`
	Body            string   `xml:"body" db:"body" json:"body"`
	Categories      string   `xml:"categories" db:"categories" json:"categories"`
	DateStart       string   `xml:"dateStart" db:"date_start" json:"dateStart"`
	DateEnd         string   `xml:"dateEnd" db:"date_end" json:"dateEnd"`
	
	Photo1          string   `xml:"photo1" db:"photo1" json:"photo1"`
	
	Photo1RemoteURL string   `db:"photo1_remote_url" json:"-"`
	Photo1LocalPath string   `db:"photo1_local_path" json:"photo1LocalPath"`
	ShopLogoRemoteURL string `db:"shop_logo_remote_url" json:"-"`
	ShopLogoLocalPath string `db:"shop_logo_local_path" json:"shopLogoLocalPath"`
	
	UpdateDate      string   `xml:"updateDate" db:"update_date" json:"updateDate"`
}

// SpecialTitleItem represents the parent item for specials in XML
type SpecialTitleItem struct {
	XMLName          xml.Name      `xml:"item" json:"-"`
	Type             string        `xml:"type,attr" json:"type"`
	SpecialTitleID   string        `xml:"specialTitleId" json:"specialTitleId"`
	SpecialTitle     string        `xml:"specialTitle" json:"specialTitle"`
	SpecialTitleStr  string        `xml:"specialTitleStr" json:"specialTitleStr"`
	SpecialTitleBody string        `xml:"specialTitleBody" json:"specialTitleBody"`
	SpecialTitleImage string       `xml:"specialTitleImage" json:"specialTitleImage"`
	StatusWeb        string        `xml:"statusWeb" json:"statusWeb"`
	StatusSignage    string        `xml:"statusSignage" json:"statusSignage"`
	PubStart         string        `xml:"pubStart" json:"pubStart"`
	PubEnd           string        `xml:"pubEnd" json:"pubEnd"`
	UpdateDate       string        `xml:"updateDate" json:"updateDate"`
	Items            []SpecialItem `xml:"item" json:"items"`
}

// SpecialItem represents a special item nested inside a specialTitle
type SpecialItem struct {
	XMLName                xml.Name `xml:"item" json:"-"`
	Type                   string   `xml:"type,attr" json:"type"`
	SpecialID              string   `xml:"specialId" db:"special_id" json:"specialId"`
	Title                  string   `xml:"title" db:"title" json:"title"`
	SubTitle               string   `xml:"subTitle" db:"sub_title" json:"subTitle"`
	CategoryID             string   `xml:"categoryId" db:"category_id" json:"categoryId"`
	CategoryName           string   `xml:"categoryName" db:"category_name" json:"categoryName"`
	SpecialSubBody         string   `xml:"specialSubBody" db:"special_sub_body" json:"specialSubBody"`
	ShopID                 string   `xml:"shopId" db:"shop_id" json:"shopId"`
	ShopName               string   `xml:"shopName" db:"shop_name" json:"shopName"`
	GenreMemo              string   `xml:"genreMemo" db:"genre_memo" json:"genreMemo"`
	ShopFloorName          string   `xml:"shopFloorName" db:"shop_floor_name" json:"shopFloorName"`
	ShopFloorsName         string   `xml:"shopFloorsName" db:"shop_floors_name" json:"shopFloorsName"`
	Venue                  string   `xml:"venue" db:"venue" json:"venue"`

	SpecialImage           string   `xml:"specialImage" json:"specialImage"`
	SpecialImage2          string   `xml:"specialImage2" json:"specialImage2"`
	SpecialImageThumbW1080 string   `xml:"specialImageThumbW1080" json:"specialImageThumbW1080"`

	ShopLogo               string   `xml:"shopLogo" db:"shop_logo" json:"shopLogo"`
	ShopLogoLocalPath      string   `db:"shop_logo_local_path" json:"shopLogoLocalPath"`

	// Promoted from parent SpecialTitleItem
	SpecialTitleID        string `db:"special_title_id" json:"specialTitleId"`
	SpecialTitle          string `db:"special_title" json:"specialTitle"`
	PubStart              string `db:"pub_start" json:"pubStart"`
	PubEnd                string `db:"pub_end" json:"pubEnd"`
	UpdateDate            string `db:"update_date" json:"updateDate"`

	SpecialImageRemoteURL string `db:"special_image_remote_url" json:"-"`
	SpecialImageLocalPath string `db:"special_image_local_path" json:"specialImageLocalPath"`
}

// SaleTitleItem represents the parent item for sales in XML
type SaleTitleItem struct {
	XMLName       xml.Name   `xml:"item" json:"-"`
	Type          string     `xml:"type,attr" json:"type"`
	SaleTitleID   string     `xml:"saleTitleId" json:"saleTitleId"`
	SaleTitle     string     `xml:"saleTitle" json:"saleTitle"`
	SaleTitleStr  string     `xml:"saleTitleStr" json:"saleTitleStr"`
	SaleTitleImage string    `xml:"saleTitleImage" json:"saleTitleImage"`
	StatusWeb     string     `xml:"statusWeb" json:"statusWeb"`
	StatusSignage string     `xml:"statusSignage" json:"statusSignage"`
	PubStart      string     `xml:"pubStart" json:"pubStart"`
	PubEnd        string     `xml:"pubEnd" json:"pubEnd"`
	UpdateDate    string     `xml:"updateDate" json:"updateDate"`
	Items         []SaleItem `xml:"item" json:"items"`
}

// SaleItem represents an individual sale entry nested inside a saleTitle
type SaleItem struct {
	XMLName        xml.Name `xml:"item" json:"-"`
	Type           string   `xml:"type,attr" json:"type"`
	SaleID         string   `xml:"saleId" db:"sale_id" json:"saleId"`
	SaleBody       string   `xml:"saleBody" db:"sale_body" json:"saleBody"`
	ShopID         string   `xml:"shopId" db:"shop_id" json:"shopId"`
	ShopName       string   `xml:"shopName" db:"shop_name" json:"shopName"`
	Genre          string   `xml:"genre" db:"genre" json:"genre"`
	GenreMemo      string   `xml:"genreMemo" db:"genre_memo" json:"genreMemo"`
	ShopFloorName  string   `xml:"shopFloorName" db:"shop_floor_name" json:"shopFloorName"`
	ShopFloorsName string   `xml:"shopFloorsName" db:"shop_floors_name" json:"shopFloorsName"`
	Area           string   `xml:"area" db:"area" json:"area"`
	AreaSub        string   `xml:"areaSub" db:"area_sub" json:"areaSub"`

	ShopLogo          string `xml:"shopLogo" db:"shop_logo" json:"shopLogo"`
	ShopLogoLocalPath string `db:"shop_logo_local_path" json:"shopLogoLocalPath"`

	// Promoted from parent SaleTitleItem
	SaleTitleID   string `db:"sale_title_id" json:"saleTitleId"`
	SaleTitle     string `db:"sale_title" json:"saleTitle"`
	PubStart      string `db:"pub_start" json:"pubStart"`
	PubEnd        string `db:"pub_end" json:"pubEnd"`
	UpdateDate    string `db:"update_date" json:"updateDate"`

	ShopLogoRemoteURL string `db:"shop_logo_remote_url" json:"-"`
}

// SaleListResponse is the root wrapper for salelist XML
type SaleListResponse struct {
	XMLName       xml.Name       `xml:"data"`
	UpdateDateAll string         `xml:"updateDateAll"`
	Items         []SaleTitleItem `xml:"item"`
}

// OptionItem represents a single option/amenity tag on a shop (from shoplist_app)
type OptionItem struct {
	OptionID      string `xml:"optionId" json:"optionId"`
	OptionName    string `xml:"optionName" json:"optionName"`
	OptionNameStr string `xml:"optionNameStr" json:"optionNameStr"`
	OptionSort    string `xml:"optionSort" json:"optionSort"`
	OptionLogo    string `xml:"optionLogo" json:"optionLogo"`
	OptionStatus  string `xml:"optionStatus" json:"optionStatus"`
	OptionUpdate  string `xml:"optionUpdate" json:"optionUpdate"`
}

type optionList struct {
	Items []OptionItem `xml:"item"`
}

// ShopAppItem holds the shop-id and options list parsed from shoplist_app
type ShopAppItem struct {
	XMLName    xml.Name   `xml:"item"`
	ShopID     string     `xml:"shopId"`
	Options    optionList `xml:"options"`
	UpdateDate string     `xml:"updateDate"`
}

// ShopAppListResponse is the root wrapper for shoplist_app XML
type ShopAppListResponse struct {
	XMLName       xml.Name      `xml:"data"`
	UpdateDateAll string        `xml:"updateDateAll"`
	Items         []ShopAppItem `xml:"item"`
}

// GenreItem represents a genre definition
type GenreItem struct {
	XMLName   xml.Name `xml:"item" json:"-"`
	GenreID   string   `xml:"genreId" db:"genre_id" json:"genreId"`
	GenreName string   `xml:"genreName" db:"genre_name" json:"genreName"`
	GenreSlug string   `xml:"genreSlug" db:"genre_slug" json:"genreSlug"`
}

// Root response wrapper for XML parsing
type ShopListResponse struct {
	XMLName       xml.Name   `xml:"data"`
	UpdateDateAll string     `xml:"updateDateAll"`
	Items         []ShopItem `xml:"item"`
}

type EventNewsResponse struct {
	XMLName       xml.Name        `xml:"data"`
	UpdateDateAll string          `xml:"updateDateAll"`
	Items         []EventNewsItem `xml:"item"`
}

type ShopNewsResponse struct {
	XMLName       xml.Name       `xml:"data"`
	UpdateDateAll string         `xml:"updateDateAll"`
	Items         []ShopNewsItem `xml:"item"`
}

type GenreListResponse struct {
	XMLName       xml.Name    `xml:"data"`
	UpdateDateAll string      `xml:"updateDateAll"`
	Items         []GenreItem `xml:"item"`
}

type SpecialListResponse struct {
	XMLName       xml.Name           `xml:"data"`
	UpdateDateAll string             `xml:"updateDateAll"`
	Items         []SpecialTitleItem `xml:"item"`
}

// FloorItem represents a floor definition
type FloorItem struct {
	XMLName   xml.Name `xml:"item" json:"-"`
	FloorID   string   `xml:"floorId" db:"floor_id" json:"floorId"`
	FloorName string   `xml:"floorName" db:"floor_name" json:"floorName"`
	SortOrder string   `xml:"sortOrder" db:"sort_order" json:"sortOrder"`
}

type FloorListResponse struct {
	XMLName       xml.Name    `xml:"data"`
	UpdateDateAll string      `xml:"updateDateAll"`
	Items         []FloorItem `xml:"item"`
}
