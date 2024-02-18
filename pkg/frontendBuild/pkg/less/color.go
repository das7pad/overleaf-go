// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package less

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func evalColor(s tokens) (tokens, error) {
	s = trimSpace(s)
	out := make(tokens, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i].kind {
		case tokenTilde:
			if len(s) > i+2 && s[i+1].kind == tokenSingleQuote {
				idx := index(s[i+2:], tokenSingleQuote)
				if idx == -1 {
					return nil, fmt.Errorf("expected quote end")
				}
				idx += i + 2
				out = append(out, s[i:idx+1]...)
				i = idx
				continue
			}
			out = append(out, s[i])
			continue
		case tokenIdentifier:
			if len(s) == i+1 || s[i+1].kind != tokenParensOpen {
				out = append(out, s[i])
				continue
			}
			switch s[i].v {
			case
				"hue", "saturation", "lightness",
				"red", "green", "blue",
				"alpha":
			case "spin":
			case "saturate", "desaturate":
			case "lighten", "darken":
			case "fade", "fadein", "fadeout":
			case "mix", "tint", "shade":
			case "greyscale":
			default:
				out = append(out, s[i])
				continue
			}
		default:
			out = append(out, s[i])
			continue
		}
		j, err := expectSeq(s, i+1, false, tokenParensOpen)
		if err != nil {
			return nil, err
		}
		j--
		j, params, err := parseArgs(s, j)
		if err != nil {
			return nil, err
		}
		j--
		for idx, param := range params {
			params[idx], err = evalColor(param)
			if err != nil {
				return nil, err
			}
		}
		c, err := parseColor(params[0])
		if err != nil {
			return nil, err
		}
		var x int64
		switch s[i].v {
		case "spin":
			x, err = strconv.ParseInt(params[1].String(), 10, 64)
		case "mix":
			if len(params) > 2 {
				x, err = parseAsPercent(params[2])
			} else {
				x = 50
			}
		case
			"hue", "saturation", "lightness",
			"red", "green", "blue",
			"alpha":
		case "greyscale":
		default:
			x, err = parseAsPercent(params[1])
		}
		if err != nil {
			return nil, fmt.Errorf("parse 2nd param: %s", err)
		}
		switch s[i].v {
		case "hue", "saturation", "lightness":
			h := c.ToHSLA()
			switch s[i].v {
			case "hue":
				x = int64(math.Round(h.hue))
			case "saturation":
				x = int64(math.Round(h.saturation))
			case "lightness":
				x = int64(math.Round(h.lightness))
			}
			out = append(out, token{kind: tokenNum, v: strconv.FormatInt(x, 10)})
			switch s[i].v {
			case "saturation", "lightness":
				out = append(out, token{kind: tokenPercent, v: "%"})
			}
			i = j
			continue
		case "red", "green", "blue":
			rgb := c.ToRGBA()
			switch s[i].v {
			case "red":
				x = int64(math.Round(rgb.r))
			case "green":
				x = int64(math.Round(rgb.g))
			case "blue":
				x = int64(math.Round(rgb.b))
			}
			out = append(out, token{kind: tokenNum, v: strconv.FormatInt(x, 10)})
			i = j
			continue
		case "alpha":
			out = append(out, token{kind: tokenNum, v: strconv.FormatFloat(c.Alpha(), 'f', -1, 64)})
			i = j
			continue
		case "spin":
			h := c.ToHSLA()
			h.hue = math.Mod(h.hue+float64(x)+360, 360)
			c = h
		case "saturate":
			h := c.ToHSLA()
			h.saturation += float64(x)
			if h.saturation > 100 {
				h.saturation = 100
			}
			c = h
		case "desaturate":
			h := c.ToHSLA()
			h.saturation -= float64(x)
			if h.saturation < 0 {
				h.saturation = 0
			}
			c = h
		case "lighten":
			h := c.ToHSLA()
			h.lightness += float64(x)
			if h.lightness > 100 {
				h.lightness = 100
			}
			c = h
		case "darken":
			h := c.ToHSLA()
			h.lightness -= float64(x)
			if h.lightness < 0 {
				h.lightness = 0
			}
			c = h
		case "fade":
			c = c.SetAlpha(float64(x) / 100)
		case "fadein":
			c = c.SetAlpha(min(100, c.Alpha()*100+float64(x)) / 100)
		case "fadeout":
			c = c.SetAlpha(max(0, c.Alpha()*100-float64(x)) / 100)
		case "mix":
			var c2 color
			c2, err = parseColor(params[1])
			if err != nil {
				return nil, err
			}
			c = mixColor(c, c2, x)
		case "tint":
			c = mixColor(hslaColor{
				hue:        0,
				saturation: 0,
				lightness:  100,
				alpha:      1,
			}, c, x)
		case "shade":
			c = mixColor(hslaColor{
				hue:        0,
				saturation: 0,
				lightness:  0,
				alpha:      1,
			}, c, x)
		case "greyscale":
			h := c.ToHSLA()
			h.saturation = 0
			c = h
		}
		out = c.Render(out)
		i = j
	}
	return out, nil
}

type color interface {
	ToRGBA() rgbaColor
	ToHSLA() hslaColor
	Alpha() float64
	SetAlpha(a float64) color
	Render(out tokens) tokens
}

type hslaColor struct {
	hue        float64
	saturation float64
	lightness  float64
	alpha      float64
}

func (s hslaColor) Alpha() float64 {
	return s.alpha
}

func (s hslaColor) SetAlpha(a float64) color {
	s.alpha = a
	return s
}

func (s hslaColor) ToHSLA() hslaColor {
	return s
}

func (s hslaColor) ToRGBA() rgbaColor {
	l := float64(s.lightness) / 100
	c := (1 - math.Abs(2*l-1)) * s.saturation / 100
	h := float64(s.hue) / 60
	x := c * (1 - math.Abs(math.Mod(h, 2)-1))
	var r, g, b float64
	switch {
	case 0 <= h && h < 1:
		r, g, b = c, x, 0
	case 1 <= h && h < 2:
		r, g, b = x, c, 0
	case 2 <= h && h < 3:
		r, g, b = 0, c, x
	case 3 <= h && h < 4:
		r, g, b = 0, x, c
	case 4 <= h && h < 5:
		r, g, b = x, 0, c
	case 5 <= h && h < 6:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return rgbaColor{
		r: 255 * (r + m),
		g: 255 * (g + m),
		b: 255 * (b + m),
		a: s.alpha,
	}
}

func (s hslaColor) Render(out tokens) tokens {
	if s.alpha != 1 {
		out = append(out, token{kind: tokenIdentifier, v: "hsla"})
	} else {
		out = append(out, token{kind: tokenIdentifier, v: "hsl"})
	}
	out = append(out, token{kind: tokenParensOpen, v: "("})
	out = append(out, token{kind: tokenNum, v: formatRoundedInt(s.hue)})
	out = append(out, token{kind: tokenComma, v: ","})
	out = append(out, token{kind: tokenNum, v: formatRoundedInt(s.saturation)})
	out = append(out, token{kind: tokenPercent, v: "%"})
	out = append(out, token{kind: tokenComma, v: ","})
	out = append(out, token{kind: tokenNum, v: formatRoundedInt(s.lightness)})
	out = append(out, token{kind: tokenPercent, v: "%"})
	if s.alpha != 1 {
		out = append(out, token{kind: tokenComma, v: ","})
		out = append(out, token{kind: tokenNum, v: strconv.FormatFloat(s.alpha, 'f', -1, 64)})
	}
	out = append(out, token{kind: tokenParensClose, v: ")"})
	return out
}

type rgbaColor struct {
	r float64
	g float64
	b float64
	a float64
}

func (s rgbaColor) Alpha() float64 {
	return s.a
}

func (s rgbaColor) SetAlpha(a float64) color {
	s.a = a
	return s
}

func (s rgbaColor) ToRGBA() rgbaColor {
	return s
}

func (s rgbaColor) ToHSLA() hslaColor {
	r := float64(s.r) / 255
	g := float64(s.g) / 255
	b := float64(s.b) / 255
	lower := min(min(r, g), b)
	upper := max(max(r, g), b)
	c := upper - lower
	l := (lower + upper) / 2
	h := hslaColor{
		alpha: s.a,
	}
	if c != 0 {
		switch upper {
		case r:
			h.hue = 60*(g-b)/c + 0
		case g:
			h.hue = 60*(b-r)/c + 120
		case b:
			h.hue = 60*(r-g)/c + 240
		}
		h.hue = math.Mod(h.hue+360, 360)
		h.saturation = 100 * c / (1 - math.Abs(2*l-1))
	}
	h.lightness = 100 * l
	return h
}

func (s rgbaColor) Render(out tokens) tokens {
	if s.a == 1 {
		out = append(out, token{kind: tokenHash, v: "#"})
		v := strconv.FormatInt(
			0+
				int64(math.Round(s.r))<<16+
				int64(math.Round(s.g))<<8+
				int64(math.Round(s.b))<<0, 16)
		if len(v) != 6 {
			v = strings.Repeat("0", 6-len(v)) + v
		}
		out = append(out, token{kind: tokenNum, v: v})
		return out
	}
	out = append(out, token{kind: tokenIdentifier, v: "rgba"})
	out = append(out, token{kind: tokenParensOpen, v: "("})
	out = append(out, token{kind: tokenNum, v: formatRoundedInt(s.r)})
	out = append(out, token{kind: tokenComma, v: ","})
	out = append(out, token{kind: tokenNum, v: formatRoundedInt(s.g)})
	out = append(out, token{kind: tokenComma, v: ","})
	out = append(out, token{kind: tokenNum, v: formatRoundedInt(s.b)})
	out = append(out, token{kind: tokenComma, v: ","})
	out = append(out, token{kind: tokenNum, v: strconv.FormatFloat(s.a, 'f', -1, 64)})
	out = append(out, token{kind: tokenParensClose, v: ")"})
	return out
}

func parseAsPercent(s tokens) (int64, error) {
	if len(s) == 0 {
		return 0, errors.New("param is empty")
	}
	if s[len(s)-1].kind != tokenPercent {
		return 0, errors.New("param did not end in %")
	}
	return strconv.ParseInt(s[:len(s)-1].String(), 10, 64)
}

func parseColor(s tokens) (color, error) {
	switch s[0].kind {
	case tokenHash:
		if len(s) == 1 {
			return nil, fmt.Errorf("incomplete hex color: %q", s.String())
		}
		hex := s[1:].String()
		if !(len(hex) == 3 || len(hex) == 6) {
			return nil, fmt.Errorf("invalid hex color length: %q", s.String())
		}
		if len(hex) == 3 {
			hex = string(append(
				[]byte{}, hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]))
		}
		rgb, err := strconv.ParseInt(hex, 16, 64)
		if err != nil {
			return nil, fmt.Errorf("bad hex color: %s", err)
		}
		return rgbaColor{
			r: float64(rgb >> 16),
			g: float64((rgb >> 8) & 255),
			b: float64(rgb & 255),
			a: 1,
		}, nil
	case tokenIdentifier:
		param := s
		switch param[0].v {
		case "rgb", "rgba":
			_, args, err := parseArgs(param[1:], 0)
			if len(args) < 3 {
				return nil, fmt.Errorf("too few args for rgba: %q", args)
			}
			rc := rgbaColor{}
			rc.r, err = strconv.ParseFloat(args[0].String(), 64)
			if err != nil {
				return nil, fmt.Errorf("parse rgba r: %s", err)
			}
			rc.g, err = strconv.ParseFloat(args[1].String(), 64)
			if err != nil {
				return nil, fmt.Errorf("parse rgba g: %s", err)
			}
			rc.b, err = strconv.ParseFloat(args[2].String(), 64)
			if err != nil {
				return nil, fmt.Errorf("parse rgba b: %s", err)
			}
			if param[0].v == "rgba" {
				if len(args) < 4 {
					return nil, fmt.Errorf("too few args for rgba: %q", args)
				}
				rc.a, err = strconv.ParseFloat(args[3].String(), 64)
				if err != nil {
					return nil, fmt.Errorf("parse rgba alpha: %s", err)
				}
			} else {
				rc.a = 1
			}
			return rc, nil
		case "hsl", "hsla":
			_, args, err := parseArgs(param[1:], 0)
			if len(args) < 3 {
				return nil, fmt.Errorf("too few args for hsla: %q", args)
			}
			var c hslaColor
			c.hue, err = strconv.ParseFloat(args[0].String(), 64)
			if err != nil {
				return nil, fmt.Errorf("parse hsla hue: %s", err)
			}
			p, err := parseAsPercent(args[1])
			if err != nil {
				return nil, fmt.Errorf("parse hsla saturation: %s", err)
			}
			c.saturation = float64(p)
			p, err = parseAsPercent(args[2])
			if err != nil {
				return nil, fmt.Errorf("parse hsla lightness: %s", err)
			}
			c.lightness = float64(p)
			if param[0].v == "hsla" {
				if len(args) < 4 {
					return nil, fmt.Errorf("too few args for hsla: %q", args)
				}
				c.alpha, err = strconv.ParseFloat(args[3].String(), 64)
				if err != nil {
					return nil, fmt.Errorf("parse hsla alpha: %s", err)
				}
			} else {
				c.alpha = 1
			}
			return c, err
		case "transparent":
			return hslaColor{}, nil
		default:
			return nil, fmt.Errorf("unknown color: %s", param[0].v)
		}
	default:
		return nil, fmt.Errorf("unknown color: %s", s[0].v)
	}
}

func mixColor(c1, c2 color, x int64) color {
	p := float64(x) / 100
	w := p*2 - 1
	a := c1.Alpha() - c2.Alpha()
	w1 := w
	if w*a != -1 {
		w1 = (w + a) / (1 + w*a)
	}
	w1 = (w1 + 1) / 2.0
	w2 := 1 - w1
	r1 := c1.ToRGBA()
	r2 := c2.ToRGBA()
	return rgbaColor{
		r: r1.r*w1 + r2.r*w2,
		g: r1.g*w1 + r2.g*w2,
		b: r1.b*w1 + r2.b*w2,
		a: r1.a*p + r2.a*(1-p),
	}
}

func formatRoundedInt(x float64) string {
	return strconv.FormatInt(int64(math.Round(x)), 10)
}
