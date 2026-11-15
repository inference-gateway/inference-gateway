package videos

import (
	"strconv"
	"strings"
)

// sizeSeparator is the separator of an OpenAI `size` value ("WIDTHxHEIGHT").
const sizeSeparator = "x"

// Size is an OpenAI `size` value: a positive width and height in pixels.
type Size struct {
	Width  int
	Height int
}

// ParseSize reads a WIDTHxHEIGHT size, reporting false when it is not one.
func ParseSize(raw string) (Size, bool) {
	w, h, ok := strings.Cut(raw, sizeSeparator)
	width, werr := strconv.Atoi(w)
	height, herr := strconv.Atoi(h)
	if !ok || werr != nil || herr != nil || width <= 0 || height <= 0 {
		return Size{}, false
	}
	return Size{Width: width, Height: height}, true
}

// AspectRatio returns the width and height reduced to lowest terms.
func (s Size) AspectRatio() (width, height int) {
	g := gcd(s.Width, s.Height)
	return s.Width / g, s.Height / g
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
