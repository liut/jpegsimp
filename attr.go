package jpegsimp

// Dimension ...
type Dimension uint32

// Quality ...
type Quality uint8

// Attr ...
type Attr struct {
	Width   Dimension `json:"width"`
	Height  Dimension `json:"height"`
	Quality Quality   `json:"quality,omitempty"`
	Ext     string    `json:"ext,omitempty"`
	Name    string    `json:"name,omitempty"`
}

// NewAttr ...
func NewAttr(w, h uint, q uint8) *Attr {
	return &Attr{
		Width:   Dimension(w),
		Height:  Dimension(h),
		Quality: Quality(q),
	}
}

// WriteOption ...
type WriteOption struct {
	StripAll bool
	Quality  Quality
}
