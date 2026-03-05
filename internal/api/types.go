package api

import "encoding/json"

// Presentation is a Google Slides presentation.
type Presentation struct {
	PresentationId string  `json:"presentationId"`
	Title          string  `json:"title"`
	Locale         string  `json:"locale"`
	Slides         []*Page `json:"slides"`
	Masters        []*Page `json:"masters"`
	Layouts        []*Page `json:"layouts"`
}

// Page represents a single slide (or master/layout) within a presentation.
type Page struct {
	ObjectId        string         `json:"objectId"`
	PageType        string         `json:"pageType"`
	PageElements    []*PageElement `json:"pageElements"`
	SlideProperties *SlideProps    `json:"slideProperties,omitempty"`
}

// SlideProps holds slide-specific properties.
type SlideProps struct {
	LayoutObjectId string `json:"layoutObjectId"`
}

// PageElement is a shape, image, table, video, line, chart, or group on a slide.
type PageElement struct {
	ObjectId     string    `json:"objectId"`
	Description  string    `json:"description"`
	Shape        *struct{} `json:"shape,omitempty"`
	Image        *struct{} `json:"image,omitempty"`
	Table        *struct{} `json:"table,omitempty"`
	Video        *struct{} `json:"video,omitempty"`
	Line         *struct{} `json:"line,omitempty"`
	SheetsChart  *struct{} `json:"sheetsChart,omitempty"`
	ElementGroup *struct{} `json:"elementGroup,omitempty"`
}

// Thumbnail is the response from the pages.getThumbnail endpoint.
type Thumbnail struct {
	Width      int64  `json:"width"`
	Height     int64  `json:"height"`
	ContentUrl string `json:"contentUrl"`
}

// BatchUpdateResponse is the response from the presentations.batchUpdate endpoint.
type BatchUpdateResponse struct {
	PresentationId string            `json:"presentationId"`
	Replies        []json.RawMessage `json:"replies"`
}

// SlidesError is returned when the API responds with an HTTP error.
type SlidesError struct {
	StatusCode int
	Message    string
}

func (e *SlidesError) Error() string {
	return e.Message
}
