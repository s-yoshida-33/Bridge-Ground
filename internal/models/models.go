package models

import "encoding/xml"

// ShopItem represents a shop from the XML feed
type ShopItem struct {
	XMLName            xml.Name `xml:"item"`
	ShopID             string   `xml:"shopId" db:"shop_id"`
	ShopName           string   `xml:"shopName" db:"shop_name"`
	ShopNameKana       string   `xml:"shopNameKana" db:"shop_name_kana"`
	ShopNameEnglish    string   `xml:"shopNameEnglish" db:"shop_name_english"`
	Genre              string   `xml:"genre" db:"genre"`
	GenreSub           string   `xml:"genreSub" db:"genre_sub"`
	Tel                string   `xml:"tel" db:"tel"`
	OpenTime           string   `xml:"openTime" db:"open_time"`
	Floor              string   `xml:"floor" db:"floor"`
	
	// Images (Relative paths in XML)
	Photo1             string   `xml:"photo1"`
	Photo1Thumb        string   `xml:"photo1Thumb"`
	ShopLogo           string   `xml:"shopLogo"`
	
	// These fields are populated during processing, not from XML directly
	Photo1RemoteURL    string   `db:"photo1_remote_url"`
	Photo1LocalPath    string   `db:"photo1_local_path"`
	ShopLogoRemoteURL  string   `db:"shop_logo_remote_url"`
	ShopLogoLocalPath  string   `db:"shop_logo_local_path"`
	
	UpdateDate         string   `xml:"updateDate" db:"update_date"`
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
