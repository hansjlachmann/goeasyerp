// Package pdf writes PDF documents: pages, texts, lines, boxes and pictures at positions in
// millimetres from the top left corner of the page. It wraps the pure-Go go-pdf/fpdf with the
// brand fonts (UTF-8, so every language prints) — Inter for text, IBM Plex Mono for amounts
// (fonts/, SIL Open Font License) — and is what printed reports draw on.
package pdf

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/go-pdf/fpdf"
)

// The families the fonts are registered under: Inter for text, IBM Plex Mono for amounts
const (
	fontFamily = "inter"
	monoFamily = "plexmono"
)

var (
	//go:embed fonts/Inter-Regular.ttf
	interRegular []byte
	//go:embed fonts/Inter-Bold.ttf
	interBold []byte
	//go:embed fonts/IBMPlexMono-Regular.ttf
	plexMonoRegular []byte
	//go:embed fonts/IBMPlexMono-Bold.ttf
	plexMonoBold []byte
)

// PointToMM: a font size in points as millimetres
const PointToMM = 25.4 / 72

// Margins of a page in millimetres
type Margins struct {
	Top, Bottom, Left, Right float64
}

// Style is how a text is printed
type Style struct {
	Size  float64 // points (0: 9)
	Bold  bool
	Align string // "left" (default), "right", "center"
	Mono  bool   // an amount: IBM Plex Mono
	// Tag names what prints the text (the report element, e.g. "field:amount"); not printed,
	// kept with the placed text for tests
	Tag string
}

// size is the style's font size (9 points by default)
func (s Style) size() float64 {
	if s.Size <= 0 {
		return 9
	}
	return s.Size
}

// LineHeight is the height of a text line of the style in millimetres
func (s Style) LineHeight() float64 {
	return s.size() * PointToMM * 1.25
}

// PlacedText is a text printed on a page (Texts: what a test checks)
type PlacedText struct {
	Page int
	X, Y float64
	Text string
	Bold bool
	// Full is the whole text when it did not fit its width and was cut ("" when it fit);
	// Width is the box it was printed in and Tag the style's tag
	Full  string
	Width float64
	Tag   string
}

// Document is a PDF being written
type Document struct {
	f       *fpdf.Fpdf
	width   float64
	height  float64
	margins Margins
	texts   []PlacedText
	broken  []PlacedText // words SplitText had to break (wider than the width): Broken
	images  map[string]bool
}

// New starts a document: size "A4" or "Letter", landscape or portrait, the page margins
func New(size string, landscape bool, margins Margins) (*Document, error) {
	sizes := map[string]string{"a4": "A4", "letter": "Letter", "": "A4"}
	sizeStr, ok := sizes[strings.ToLower(size)]
	if !ok {
		return nil, fmt.Errorf("pdf: unknown page size %q", size)
	}
	orientation := "P"
	if landscape {
		orientation = "L"
	}
	f := fpdf.New(orientation, "mm", sizeStr, "")
	f.SetAutoPageBreak(false, 0)
	f.SetMargins(0, 0, 0)
	f.SetCreator("GoEasyERP", true)
	f.SetCreationDate(time.Now())
	f.AddUTF8FontFromBytes(fontFamily, "", interRegular)
	f.AddUTF8FontFromBytes(fontFamily, "B", interBold)
	f.AddUTF8FontFromBytes(monoFamily, "", plexMonoRegular)
	f.AddUTF8FontFromBytes(monoFamily, "B", plexMonoBold)
	f.SetFont(fontFamily, "", 9)
	f.SetDrawColor(0, 0, 0)
	f.SetTextColor(0, 0, 0)
	width, height := f.GetPageSize()
	if err := f.Error(); err != nil {
		return nil, err
	}
	return &Document{f: f, width: width, height: height, margins: margins, images: map[string]bool{}}, nil
}

// SetTitle sets the document's title (shown by PDF viewers)
func (d *Document) SetTitle(title string) { d.f.SetTitle(title, true) }

// AddPage starts a new page
func (d *Document) AddPage() { d.f.AddPage() }

// Pages is the number of pages
func (d *Document) Pages() int { return d.f.PageNo() }

// Width and Height of the page; Margins as set
func (d *Document) Width() float64   { return d.width }
func (d *Document) Height() float64  { return d.height }
func (d *Document) Margins() Margins { return d.margins }
func (d *Document) ContentWidth() float64 {
	return d.width - d.margins.Left - d.margins.Right
}

// setStyle selects the style's font
func (d *Document) setStyle(style Style) {
	fontStyle := ""
	if style.Bold {
		fontStyle = "B"
	}
	family := fontFamily
	if style.Mono {
		family = monoFamily
	}
	d.f.SetFont(family, fontStyle, style.size())
}

// StringWidth is the width of a text in the style, in millimetres
func (d *Document) StringWidth(text string, style Style) float64 {
	d.setStyle(style)
	return d.f.GetStringWidth(text)
}

// SplitText breaks a text into lines that fit the width (at spaces; a word longer than the
// width is broken)
func (d *Document) SplitText(text string, width float64, style Style) []string {
	if width <= 0 || text == "" {
		return []string{text}
	}
	d.setStyle(style)
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		for _, word := range strings.Fields(paragraph) {
			if d.f.GetStringWidth(word) > width+0.01 {
				d.broken = append(d.broken, PlacedText{Text: word, Full: paragraph, Width: width, Bold: style.Bold, Tag: style.Tag})
			}
		}
		lines = append(lines, d.f.SplitText(paragraph, width)...)
	}
	return lines
}

// Text prints a text in a box at x, y (millimetres, top left) of the width: aligned in it, one
// line (cut at the width when longer, so columns stay apart)
func (d *Document) Text(x, y, width float64, text string, style Style) {
	if text == "" {
		return
	}
	d.setStyle(style)
	full := ""
	if width > 0 && d.f.GetStringWidth(text) > width+0.01 {
		full = text
		for text != "" && d.f.GetStringWidth(text) > width+0.01 {
			runes := []rune(text)
			text = string(runes[:len(runes)-1])
		}
	}
	align := "L"
	switch style.Align {
	case "right":
		align = "R"
	case "center":
		align = "C"
	}
	d.f.SetXY(x, y)
	d.f.CellFormat(width, style.LineHeight(), text, "", 0, align+"M", false, 0, "")
	d.texts = append(d.texts, PlacedText{Page: d.f.PageNo(), X: x, Y: y, Text: text, Bold: style.Bold, Full: full, Width: width, Tag: style.Tag})
}

// Line draws a line from x1, y1 to x2, y2 (millimetres), width in millimetres (0: 0.2)
func (d *Document) Line(x1, y1, x2, y2, width float64) {
	if width <= 0 {
		width = 0.2
	}
	d.f.SetLineWidth(width)
	d.f.Line(x1, y1, x2, y2)
}

// Box draws a rectangle's border
func (d *Document) Box(x, y, w, h, lineWidth float64) {
	if lineWidth <= 0 {
		lineWidth = 0.2
	}
	d.f.SetLineWidth(lineWidth)
	d.f.Rect(x, y, w, h, "D")
}

// Image prints a picture (PNG or JPEG) fitted into the box at x, y of w × h millimetres,
// keeping its proportions, at the box's top left
func (d *Document) Image(x, y, w, h float64, data []byte) error {
	imageType := ""
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		imageType = "PNG"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		imageType = "JPG"
	default:
		return errors.New("pdf: picture is not PNG or JPEG")
	}
	sum := sha1.Sum(data)
	name := hex.EncodeToString(sum[:])
	options := fpdf.ImageOptions{ImageType: imageType}
	if !d.images[name] {
		d.f.RegisterImageOptionsReader(name, options, bytes.NewReader(data))
		if err := d.f.Error(); err != nil {
			return err
		}
		d.images[name] = true
	}
	info := d.f.GetImageInfo(name)
	if info == nil || info.Width() <= 0 || info.Height() <= 0 {
		return errors.New("pdf: picture has no size")
	}
	iw, ih := w, h
	ratio := info.Width() / info.Height()
	switch {
	case w <= 0 && h <= 0:
		return errors.New("pdf: picture needs a width or a height")
	case w <= 0:
		iw = h * ratio
	case h <= 0:
		ih = w / ratio
	case w/h > ratio:
		iw = h * ratio
	default:
		ih = w / ratio
	}
	d.f.ImageOptions(name, x, y, iw, ih, false, options, 0, "")
	return d.f.Error()
}

// Texts returns the texts printed so far, in order (for tests)
func (d *Document) Texts() []PlacedText { return d.texts }

// Broken returns the words wrapped texts had to break because they were wider than their
// width (Text: the word, Full: the text)
func (d *Document) Broken() []PlacedText { return d.broken }

// Bytes finishes the document and returns the PDF
func (d *Document) Bytes() ([]byte, error) {
	if d.f.PageNo() == 0 {
		d.f.AddPage()
	}
	var buf bytes.Buffer
	err := d.f.Output(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
