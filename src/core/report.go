package core

// SkippedAlreadyOptimal marks a file returned unchanged because re-encoding
// would not make it smaller.
const SkippedAlreadyOptimal = "already_optimal"

// Report describes one Compress call. The CLI fills Input/Output.
type Report struct {
	Input     string   `json:"input,omitempty"`
	Output    string   `json:"output,omitempty"`
	FormatIn  string   `json:"format_in"`
	FormatOut string   `json:"format_out"`
	SizeIn    int      `json:"size_in"`
	SizeOut   int      `json:"size_out"`
	SavedPct  float64  `json:"saved_pct"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Colors    int      `json:"colors,omitempty"`
	Quality   int      `json:"quality,omitempty"`
	SSIM      float64  `json:"ssim"`
	PSNR      float64  `json:"psnr"`
	Skipped   string   `json:"skipped,omitempty"`
	Warnings  []string `json:"warnings"`
}
