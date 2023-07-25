// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func evalColor(s tokens) (tokens, error) {
	s = trimSpace(s)
	out := make(tokens, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i].kind {
		case tokenIdentifier:
			switch s[i].v {
			case "spin":
			case "saturate", "desaturate":
			case "lighten", "darken":
			case "fadein", "fadeout":
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
		case "greyscale":
		default:
			x, err = parseAsPercent(params[1])
		}
		if err != nil {
			return nil, fmt.Errorf("parse 2nd param: %s", err)
		}
		switch s[i].v {
		case "spin":
			c.hue = (c.hue + x + 360) % 360
		case "saturate":
			c.saturation += x
			if c.saturation > 100 {
				c.saturation = 100
			}
		case "desaturate":
			c.saturation -= x
			if c.saturation < 0 {
				c.saturation = 0
			}
		case "lighten":
			c.lightness += x
			if c.lightness > 100 {
				c.lightness = 100
			}
		case "darken":
			c.lightness -= x
			if c.lightness < 0 {
				c.lightness = 0
			}
		case "fadein":
			c.alpha = math.Min(100, c.alpha*100+float64(x)) / 100
		case "fadeout":
			c.alpha = math.Max(0, c.alpha*100-float64(x)) / 100
		case "mix":
			var c2 hslaColor
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
			c.saturation = 0
		}
		if c.alpha != 1 {
			out = append(out, token{kind: tokenIdentifier, v: "hsla"})
		} else {
			out = append(out, token{kind: tokenIdentifier, v: "hsl"})
		}
		out = append(out, token{kind: tokenParensOpen, v: "("})
		out = append(out, token{kind: tokenNum, v: strconv.FormatInt(c.hue, 10)})
		out = append(out, token{kind: tokenComma, v: ","})
		out = append(out, token{kind: tokenNum, v: strconv.FormatInt(c.saturation, 10)})
		out = append(out, token{kind: tokenPercent, v: "%"})
		out = append(out, token{kind: tokenComma, v: ","})
		out = append(out, token{kind: tokenNum, v: strconv.FormatInt(c.lightness, 10)})
		out = append(out, token{kind: tokenPercent, v: "%"})
		if c.alpha != 1 {
			out = append(out, token{kind: tokenComma, v: ","})
			out = append(out, token{kind: tokenNum, v: strconv.FormatFloat(c.alpha, 'f', -1, 64)})
		}
		out = append(out, token{kind: tokenParensClose, v: ")"})
		i = j
	}
	return out, nil
}

type hslaColor struct {
	hue        int64
	saturation int64
	lightness  int64
	alpha      float64
}

func (s hslaColor) ToRGBA() rgbaColor {
	l := float64(s.lightness) / 100
	c := (1 - math.Abs(2*l-1)) * float64(s.saturation) / 100
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
		r: int64(math.Round(255 * (r + m))),
		g: int64(math.Round(255 * (g + m))),
		b: int64(math.Round(255 * (b + m))),
		a: s.alpha,
	}
}

type rgbaColor struct {
	r int64
	g int64
	b int64
	a float64
}

func (s rgbaColor) ToHSLA() hslaColor {
	r := float64(s.r) / 255
	g := float64(s.g) / 255
	b := float64(s.b) / 255
	min := math.Min(math.Min(r, g), b)
	max := math.Max(math.Max(r, g), b)
	c := max - min
	l := (min + max) / 2
	h := hslaColor{
		alpha: s.a,
	}
	if c != 0 {
		switch max {
		case r:
			h.hue = int64(math.Round(60*(g-b)/c)) + 0
		case g:
			h.hue = int64(math.Round(60*(b-r)/c)) + 120
		case b:
			h.hue = int64(math.Round(60*(r-g)/c)) + 240
		}
		h.hue = (h.hue + 360) % 360
		h.saturation = int64(math.Round(100 * c / (1 - math.Abs(2*l-1))))
	}
	h.lightness = int64(math.Round(100 * l))
	return h
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

func parseColor(s tokens) (hslaColor, error) {
	var c hslaColor
	switch s[0].kind {
	case tokenHash:
		if len(s) == 1 {
			return c, fmt.Errorf("incomplete hex color: %q", s.String())
		}
		hex := s[1:].String()
		if !(len(hex) == 3 || len(hex) == 6) {
			return c, fmt.Errorf("invalid hex color length: %q", s.String())
		}
		if len(hex) == 3 {
			hex = string(append(
				[]byte{}, hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]))
		}
		rgb, err := strconv.ParseInt(hex, 16, 64)
		if err != nil {
			return c, fmt.Errorf("bad hex color: %s", err)
		}
		c = rgbaColor{
			r: rgb >> 16,
			g: (rgb >> 8) & 255,
			b: rgb & 255,
			a: 1,
		}.ToHSLA()
		return c, nil
	case tokenIdentifier:
		param := s
		switch param[0].v {
		case "rgb", "rgba":
			_, args, err := parseArgs(param[1:], 0)
			if len(args) < 3 {
				return c, fmt.Errorf("too few args for rgba: %q", args)
			}
			rc := rgbaColor{}
			rc.r, err = strconv.ParseInt(args[0].String(), 10, 64)
			if err != nil {
				return c, fmt.Errorf("parse rgba r: %s", err)
			}
			rc.g, err = strconv.ParseInt(args[1].String(), 10, 64)
			if err != nil {
				return c, fmt.Errorf("parse rgba g: %s", err)
			}
			rc.b, err = strconv.ParseInt(args[2].String(), 10, 64)
			if err != nil {
				return c, fmt.Errorf("parse rgba b: %s", err)
			}
			if param[0].v == "rgba" {
				if len(args) < 4 {
					return c, fmt.Errorf("too few args for rgba: %q", args)
				}
				rc.a, err = strconv.ParseFloat(args[3].String(), 64)
				if err != nil {
					return c, fmt.Errorf("parse rgba alpha: %s", err)
				}
			} else {
				rc.a = 1
			}
			c = rc.ToHSLA()
		case "hsl", "hsla":
			_, args, err := parseArgs(param[1:], 0)
			if len(args) < 3 {
				return c, fmt.Errorf("too few args for hsla: %q", args)
			}
			c.hue, err = strconv.ParseInt(args[0].String(), 10, 64)
			if err != nil {
				return c, fmt.Errorf("parse hsla hue: %s", err)
			}
			c.saturation, err = parseAsPercent(args[1])
			if err != nil {
				return c, fmt.Errorf("parse hsla saturation: %s", err)
			}
			c.lightness, err = parseAsPercent(args[2])
			if err != nil {
				return c, fmt.Errorf("parse hsla lightness: %s", err)
			}
			if param[0].v == "hsla" {
				if len(args) < 4 {
					return c, fmt.Errorf("too few args for hsla: %q", args)
				}
				c.alpha, err = strconv.ParseFloat(args[3].String(), 64)
				if err != nil {
					return c, fmt.Errorf("parse hsla alpha: %s", err)
				}
			} else {
				c.alpha = 1
			}
		case "transparent":
		default:
			return c, fmt.Errorf("unknown color: %s", param[0].String())
		}
		return c, nil
	default:
		return c, fmt.Errorf("unknown color: %s", s[0].String())
	}
}

func mixColor(c1, c2 hslaColor, x int64) hslaColor {
	p := float64(x) / 100
	w := p*2 - 1
	a := c1.alpha - c2.alpha
	w1 := w
	if w*a != -1 {
		w1 = (w + a) / (1 + w*a)
	}
	w1 = (w1 + 1) / 2.0
	w2 := 1 - w1
	r1 := c1.ToRGBA()
	r2 := c2.ToRGBA()
	return rgbaColor{
		r: int64(math.Round(float64(r1.r)*w1 + float64(r2.r)*w2)),
		g: int64(math.Round(float64(r1.g)*w1 + float64(r2.g)*w2)),
		b: int64(math.Round(float64(r1.b)*w1 + float64(r2.b)*w2)),
		a: r1.a*p + r2.a*(1-p),
	}.ToHSLA()
}
