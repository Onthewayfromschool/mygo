package terminal

import (
	"testing"

	"github.com/egoist/mygo/internal/text"
	"github.com/egoist/mygo/ui"
)

func TestContrasted(t *testing.T) {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	for _, c := range []struct {
		fg, bg ui.Color
		ratio  float32
	}{
		{ui.RGB(240, 240, 240), white, 3},  // a dark theme's gray on a light one
		{ui.RGB(30, 30, 30), black, 3},     // a light theme's on a dark one
		{ui.RGB(229, 229, 16), white, 4.5}, // bright yellow
	} {
		got := contrasted(c.fg, c.bg, c.ratio)
		r := contrast(luminance(got), luminance(c.bg))
		if r < c.ratio || r > c.ratio*1.1 {
			t.Errorf("contrasted(%v, %v, %v) = %v, a ratio of %v", c.fg, c.bg, c.ratio, got, r)
		}
	}
	if got := contrasted(black, white, 4.5); got != black {
		t.Errorf("black on white changed to %v", got)
	}
	// Short of the ratio even at black or white, the text takes the one
	// that contrasts more.
	if got := contrasted(ui.RGB(100, 100, 100), ui.RGB(128, 128, 128), 21); got != black {
		t.Errorf("gray on gray at 21 = %v, want black", got)
	}
}

// Shapes keep their colors under the minimum contrast, as in Ghostty, and
// text takes it: Copilot CLI's input box, a light gray edged with half
// blocks of that gray, showed dark bars on a white background.
func TestMinimumContrastShapes(t *testing.T) {
	loadLib(t)
	light := &Theme{Foreground: ui.RGB(0x20, 0x20, 0x20), Background: ui.RGB(255, 255, 255)}
	term, err := New(Options{Conn: newPipe(), Theme: light, MinimumContrast: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 400, 200)
	gray := ui.RGB(232, 233, 235)
	term.Feed([]byte("\x1b[38;2;232;233;235m▄▀█\U0001fb82A"))
	tt.Frame()
	term.mu.Lock()
	line := term.v.lines[0]
	term.mu.Unlock()
	for _, b := range line.boxes {
		if b.c != gray {
			t.Errorf("%q drawn in %v, want %v", b.r, b.c, gray)
		}
	}
	if len(line.boxes) != 4 {
		t.Errorf("drew %d shapes, want 4", len(line.boxes))
	}
	var text []ui.Color
	for _, r := range line.runs {
		text = append(text, r.c)
	}
	if len(text) == 0 || text[len(text)-1] == gray {
		t.Errorf("text in %v: not contrasted from %v", text, gray)
	}
}

// TestRowsShapedAnewAfterFonts shapes the rows anew once the text system
// lets go of its fonts: the glyphs of the rows refer to them, and drawing
// them crashed in Core Text after a long session.
func TestRowsShapedAnewAfterFonts(t *testing.T) {
	loadLib(t)
	term, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 400, 200)
	term.Feed([]byte("hello"))
	tt.Frame()
	glyph := func() *ui.Glyph {
		term.mu.Lock()
		defer term.mu.Unlock()
		if len(term.v.lines) == 0 || len(term.v.lines[0].runs) == 0 {
			t.Fatal("no text in the first row")
		}
		return &term.v.lines[0].runs[0].glyphs[0]
	}
	before := glyph()
	// Fonts of many sizes, as an animation makes, which the next frame
	// lets go of.
	s := text.Shared()
	gen := s.Generation()
	for i := 0; s.Generation() == gen && i < 10000; i++ {
		s.Layout(text.Params{Text: "Hi", Style: text.Style{Size: 10 + float32(i)/10}})
		if i%64 == 63 {
			s.EndFrame()
		}
	}
	if s.Generation() == gen {
		t.Fatal("the text system kept its fonts")
	}
	tt.Frame()
	if glyph() == before {
		t.Error("the row kept the glyphs of the fonts let go of")
	}
}
