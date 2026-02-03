// Package serper provides a client for the Serper.dev search API.
package serper

import "fmt"

// SearchRequest represents a search request to Serper.dev.
type SearchRequest struct {
	Q        string `json:"q"`
	Num      int    `json:"num,omitempty"`
	GL       string `json:"gl,omitempty"`
	HL       string `json:"hl,omitempty"`
	Location string `json:"location,omitempty"`
	Page     int    `json:"page,omitempty"`
}

// SearchResponse represents the response from Serper.dev search endpoint.
type SearchResponse struct {
	SearchParameters SearchParameters `json:"searchParameters"`
	KnowledgeGraph   *KnowledgeGraph  `json:"knowledgeGraph,omitempty"`
	Organic          []OrganicResult  `json:"organic"`
	PeopleAlsoAsk    []PeopleAlsoAsk  `json:"peopleAlsoAsk,omitempty"`
	RelatedSearches  []RelatedSearch  `json:"relatedSearches,omitempty"`
}

// SearchParameters contains the echoed search parameters.
type SearchParameters struct {
	Q      string `json:"q"`
	GL     string `json:"gl"`
	HL     string `json:"hl"`
	Num    int    `json:"num"`
	Type   string `json:"type"`
	Engine string `json:"engine"`
}

// KnowledgeGraph contains knowledge graph data.
type KnowledgeGraph struct {
	Title       string            `json:"title"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Website     string            `json:"website,omitempty"`
	ImageURL    string            `json:"imageUrl,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// OrganicResult represents a single organic search result.
type OrganicResult struct {
	Title     string     `json:"title"`
	Link      string     `json:"link"`
	Snippet   string     `json:"snippet"`
	Position  int        `json:"position"`
	Date      string     `json:"date,omitempty"`
	Sitelinks []Sitelink `json:"sitelinks,omitempty"`
}

// Sitelink represents a sitelink within an organic result.
type Sitelink struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

// PeopleAlsoAsk represents a "People Also Ask" entry.
type PeopleAlsoAsk struct {
	Question string `json:"question"`
	Snippet  string `json:"snippet"`
	Title    string `json:"title"`
	Link     string `json:"link"`
}

// RelatedSearch represents a related search suggestion.
type RelatedSearch struct {
	Query string `json:"query"`
}

// ImagesResponse represents the response from Serper.dev images endpoint.
type ImagesResponse struct {
	SearchParameters SearchParameters `json:"searchParameters"`
	Images           []ImageResult    `json:"images"`
}

// ImageResult represents a single image search result.
type ImageResult struct {
	Title        string `json:"title"`
	ImageURL     string `json:"imageUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Source       string `json:"source"`
	Link         string `json:"link"`
	Position     int    `json:"position"`
}

// NewsResponse represents the response from Serper.dev news endpoint.
type NewsResponse struct {
	SearchParameters SearchParameters `json:"searchParameters"`
	News             []NewsResult     `json:"news"`
}

// NewsResult represents a single news search result.
type NewsResult struct {
	Title    string `json:"title"`
	Link     string `json:"link"`
	Snippet  string `json:"snippet"`
	Source   string `json:"source"`
	Date     string `json:"date"`
	ImageURL string `json:"imageUrl,omitempty"`
	Position int    `json:"position"`
}

// PlacesResponse represents the response from Serper.dev places endpoint.
type PlacesResponse struct {
	SearchParameters SearchParameters `json:"searchParameters"`
	Places           []PlaceResult    `json:"places"`
}

// PlaceResult represents a single place search result.
type PlaceResult struct {
	Title       string   `json:"title"`
	Address     string   `json:"address"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	Rating      float64  `json:"rating"`
	RatingCount int      `json:"ratingCount"`
	Category    string   `json:"category"`
	Phone       string   `json:"phoneNumber,omitempty"`
	Website     string   `json:"website,omitempty"`
	Hours       []string `json:"hours,omitempty"`
	Position    int      `json:"position"`
}

// ScholarResponse represents the response from Serper.dev scholar endpoint.
type ScholarResponse struct {
	SearchParameters SearchParameters `json:"searchParameters"`
	Organic          []ScholarResult  `json:"organic"`
}

// ScholarResult represents a single scholar search result.
type ScholarResult struct {
	Title           string   `json:"title"`
	Link            string   `json:"link"`
	Snippet         string   `json:"snippet"`
	PublicationInfo string   `json:"publicationInfo"`
	CitedBy         int      `json:"citedBy"`
	Authors         []string `json:"authors,omitempty"`
	Year            int      `json:"year,omitempty"`
	Position        int      `json:"position"`
}

// SetDefaults applies default values to a SearchRequest.
func (r *SearchRequest) SetDefaults() {
	if r.Num == 0 {
		r.Num = 10
	}
	if r.GL == "" {
		r.GL = "us"
	}
	if r.HL == "" {
		r.HL = "en"
	}
	if r.Page == 0 {
		r.Page = 1
	}
}

// Validate checks that the SearchRequest is valid.
func (r *SearchRequest) Validate() error {
	if r.Q == "" {
		return fmt.Errorf("query (q) is required")
	}
	if r.Num < 0 || r.Num > 100 {
		return fmt.Errorf("num must be between 1 and 100")
	}
	if r.Page < 0 {
		return fmt.Errorf("page must be non-negative")
	}
	return nil
}
