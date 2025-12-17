package models

import "encoding/xml"

// ShopItem represents a shop from the XML feed
type ShopItem struct {
	XMLName          xml.Name `xml:"item" json:"-"`
	ShopID           string   `xml:"shopId" db:"shop_id" json:"shopId"`
	ShopName         string   `xml:"shopName" db:"shop_name" json:"shopName"`
	ShopNameKana     string   `xml:"shopNameKana" db:"shop_name_kana" json:"shopNameKana"`
	ShopNameEnglish  string   `xml:"shopNameEnglish" db:"shop_name_english" json:"shopNameEnglish"`
	Searches         string   `xml:"searches" db:"searches" json:"searches"`
	Genre            string   `xml:"genre" db:"genre" json:"genre"`
	GenreSub         string   `xml:"genreSub" db:"genre_sub" json:"genreSub"`
	GenreSubEnglish  string   `xml:"genreSubEnglish" db:"genre_sub_english" json:"genreSubEnglish"`
	GenreMemo        string   `xml:"genreMemo" db:"genre_memo" json:"genreMemo"`
	GenreMemoEnglish string   `xml:"genreMemoEnglish" db:"genre_memo_english" json:"genreMemoEnglish"`
	GroupID          string   `xml:"groupId" db:"group_id" json:"groupId"`
	Tel              string   `xml:"tel" db:"tel" json:"tel"`
	Floors           string   `xml:"floors" db:"floors" json:"floors"`
	Area             string   `xml:"area" db:"area" json:"area"`
	AreaSub          string   `xml:"areaSub" db:"area_sub" json:"areaSub"`
	Number           string   `xml:"number" db:"number" json:"number"`
	CloseFlg         string   `xml:"closeFlg" db:"close_flg" json:"closeFlg"`
	OpenTime         string   `xml:"openTime" db:"open_time" json:"openTime"`
	Description      string   `xml:"description" db:"description" json:"description"`

	// Images (Original values from XML)
	Photo1   string `xml:"photo1" db:"photo1" json:"photo1"`
	Photo1LocalPath   string `db:"photo1_local_path" json:"photo1LocalPath"`

	Photo2   string `xml:"photo2" db:"photo2" json:"photo2"`
	Photo2LocalPath   string `db:"photo2_local_path" json:"photo2LocalPath"`

	ShopLogo string `xml:"shopLogo" db:"shop_logo" json:"shopLogo"`
	ShopLogoLocalPath string `db:"shop_logo_local_path" json:"shopLogoLocalPath"`

	UpdateDate string `xml:"updateDate" db:"update_date" json:"updateDate"`

	// Internal fields for downloaded files (not in XML, but needed for DB/App logic)
	Photo1RemoteURL   string `db:"photo1_remote_url" json:"-"`
	Photo2RemoteURL   string `db:"photo2_remote_url" json:"-"`
	ShopLogoRemoteURL string `db:"shop_logo_remote_url" json:"-"`
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
	XMLName      xml.Name      `xml:"item" json:"-"`
	Type         string        `xml:"type,attr" json:"type"` // specialTitle or special
	SpecialTitle string        `xml:"specialTitle" json:"specialTitle"`
	UpdateDate   string        `xml:"updateDate" json:"updateDate"`
	Items        []SpecialItem `xml:"item" json:"items"` // Nested items
}

// SpecialItem represents a special item (sometimes nested or flattened)
type SpecialItem struct {
	XMLName         xml.Name `xml:"item" json:"-"`
	Type            string   `xml:"type,attr" json:"type"`
	SpecialID       string   `xml:"specialId" db:"special_id" json:"specialId"`
	Title           string   `xml:"title" db:"title" json:"title"`
	SpecialSubBody  string   `xml:"specialSubBody" db:"special_sub_body" json:"specialSubBody"`
	CategoryName    string   `xml:"categoryName" db:"category_name" json:"categoryName"`
	ShopID          string   `xml:"shopId" db:"shop_id" json:"shopId"`
	ShopName        string   `xml:"shopName" db:"shop_name" json:"shopName"`
	
	SpecialImage    string   `xml:"specialImage" json:"specialImage"`
	
	// Derived
	SpecialTitle          string `db:"special_title" json:"specialTitle"`
	UpdateDate            string `db:"update_date" json:"updateDate"`
	SpecialImageRemoteURL string `db:"special_image_remote_url" json:"-"` // Not in schema explicitly but good for logic
	SpecialImageLocalPath string `db:"special_image_local_path" json:"specialImageLocalPath"`
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
