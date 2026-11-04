package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestDocument(t *testing.T) {
	d, err := New("A4", false, Margins{Top: 10, Bottom: 10, Left: 12, Right: 12})
	if err != nil {
		t.Fatal(err)
	}
	if d.Width() < 209 || d.Width() > 211 || d.Height() < 296 || d.Height() > 298 {
		t.Fatalf("A4 is %v × %v", d.Width(), d.Height())
	}
	d.AddPage()
	d.Text(12, 10, 100, "Nordkapp Kontor AS – Bærum, Ølstuen", Style{Bold: true, Size: 12})
	d.Text(12, 20, 40, "1 234,50", Style{Align: "right"})
	d.Line(12, 30, 198, 30, 0)
	d.Box(12, 35, 50, 20, 0)

	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		img.Set(x, 10, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := d.Image(150, 10, 40, 40, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := d.Image(150, 10, 40, 40, []byte("<svg/>")); err == nil {
		t.Error("an SVG was taken as a picture")
	}
	d.AddPage()
	d.Text(12, 10, 0, "Page 2", Style{})

	out, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", out[:10])
	}
	if d.Pages() != 2 {
		t.Errorf("pages %d, want 2", d.Pages())
	}
	texts := d.Texts()
	if len(texts) != 3 || texts[0].Text != "Nordkapp Kontor AS – Bærum, Ølstuen" || !texts[0].Bold || texts[2].Page != 2 {
		t.Errorf("texts %+v", texts)
	}
}

func TestTextCutAtWidth(t *testing.T) {
	d, _ := New("Letter", false, Margins{})
	d.AddPage()
	d.Text(0, 0, 10, "A very long description that does not fit", Style{})
	got := d.Texts()[0].Text
	if got == "A very long description that does not fit" || d.StringWidth(got, Style{}) > 10.01 {
		t.Errorf("not cut: %q", got)
	}
	lines := d.SplitText("A very long description that does not fit", 30, Style{})
	if len(lines) < 2 {
		t.Errorf("not split: %q", lines)
	}
}

func TestEmptyDocumentHasAPage(t *testing.T) {
	d, _ := New("", true, Margins{})
	out, err := d.Bytes()
	if err != nil || !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("%v %q", err, out)
	}
	if _, err := New("A5", false, Margins{}); err == nil {
		t.Error("A5 taken")
	}
}
