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

// EventNewsItem represents an event news item
type EventNewsItem struct {
	XMLName         xml.Name `xml:"item"`
	EventID         string   `xml:"eventId" db:"event_id"`
	Title           string   `xml:"title" db:"title"`
	Body            string   `xml:"body" db:"body"`
	DateStart       string   `xml:"dateStart" db:"date_start"`
	DateEnd         string   `xml:"dateEnd" db:"date_end"`
	
	Photo1          string   `xml:"photo1"`
	
	Photo1RemoteURL string   `db:"photo1_remote_url"`
	Photo1LocalPath string   `db:"photo1_local_path"`
	
	UpdateDate      string   `xml:"updateDate" db:"update_date"`
}

// ShopNewsItem represents a shop news item
type ShopNewsItem struct {
	XMLName         xml.Name `xml:"item"`
	ShopNewsID      string   `xml:"shopNewsId" db:"shop_news_id"`
	ShopID          string   `xml:"shopId" db:"shop_id"`
	Title           string   `xml:"title" db:"title"`
	Body            string   `xml:"body" db:"body"`
	
	Photo1          string   `xml:"photo1"`
	
	Photo1RemoteURL string   `db:"photo1_remote_url"`
	Photo1LocalPath string   `db:"photo1_local_path"`
	
	UpdateDate      string   `xml:"updateDate" db:"update_date"`
}

// SpecialTitleItem represents the parent item for specials in XML
type SpecialTitleItem struct {
	XMLName      xml.Name      `xml:"item"`
	Type         string        `xml:"type,attr"` // specialTitle or special
	SpecialTitle string        `xml:"specialTitle"`
	UpdateDate   string        `xml:"updateDate"`
	Items        []SpecialItem `xml:"item"` // Nested items
}

// SpecialItem represents a special item (sometimes nested or flattened)
type SpecialItem struct {
	XMLName         xml.Name `xml:"item"`
	Type            string   `xml:"type,attr"`
	SpecialID       string   `xml:"specialId" db:"special_id"`
	Title           string   `xml:"title" db:"title"`
	SpecialSubBody  string   `xml:"specialSubBody" db:"special_sub_body"`
	CategoryName    string   `xml:"categoryName" db:"category_name"`
	ShopID          string   `xml:"shopId" db:"shop_id"`
	ShopName        string   `xml:"shopName" db:"shop_name"`
	
	SpecialImage    string   `xml:"specialImage"`
	
	// Derived
	SpecialTitle          string `db:"special_title"`
	UpdateDate            string `db:"update_date"`
	SpecialImageRemoteURL string `db:"special_image_remote_url"` // Not in schema explicitly but good for logic
	SpecialImageLocalPath string `db:"special_image_local_path"`
}

// GenreItem represents a genre definition
type GenreItem struct {
	XMLName   xml.Name `xml:"item"`
	GenreID   string   `xml:"genreId" db:"genre_id"`
	GenreName string   `xml:"genreName" db:"genre_name"`
	GenreSlug string   `xml:"genreSlug" db:"genre_slug"`
}

// Root response wrapper for XML parsing
type ShopListResponse struct {
	XMLName xml.Name   `xml:"data"`
	Items   []ShopItem `xml:"item"`
}

type EventNewsResponse struct {
	XMLName xml.Name        `xml:"data"`
	Items   []EventNewsItem `xml:"item"`
}

type ShopNewsResponse struct {
	XMLName xml.Name       `xml:"data"`
	Items   []ShopNewsItem `xml:"item"`
}

type GenreListResponse struct {
	XMLName xml.Name    `xml:"data"`
	Items   []GenreItem `xml:"item"`
}

type SpecialListResponse struct {
	XMLName xml.Name           `xml:"data"`
	Items   []SpecialTitleItem `xml:"item"`
}
