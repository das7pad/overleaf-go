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
		k := j + 1
		var params []tokens
		l := 0
		for ; j < len(s); j++ {
			switch s[j].kind {
			case tokenParensOpen:
				l++
			case tokenParensClose:
				l--
			case tokenComma:
				if l == 1 {
					params = append(params, trimSpace(s[k:j]))
					k = j + 1
				}
			}
			if l == 0 {
				break
			}
		}
		if l > 0 {
			return nil, errors.New("expected closing ) after color fn")
		}
		params = append(params, trimSpace(s[k:j]))
		for idx, param := range params {
			params[idx], err = evalColor(param)
			if err != nil {
				return nil, err
			}
		}
		var c hslaColor
		switch params[0][0].kind {
		case tokenHash:
			if len(params[0]) == 1 {
				return nil, errors.New("incomplete hex color")
			}
			hex := params[0][1:].String()
			if !(len(hex) == 3 || len(hex) == 6) {
				return nil, errors.New("invalid hex color length")
			}
			if len(hex) == 3 {
				hex = string(append(
					[]byte{}, hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]))
			}
			var rgb int64
			rgb, err = strconv.ParseInt(hex, 16, 64)
			if err != nil {
				return nil, fmt.Errorf("bad hex color: %q", err)
			}
			c = rgbaColor{
				r: rgb >> 16,
				g: (rgb >> 8) & 255,
				b: rgb & 255,
				a: 1,
			}.ToHSLA()
		case tokenIdentifier:
			param := params[0]
			switch param[0].v {
			case "rgb", "rgba":
				args := getArgs(param[2 : len(param)-1])
				if len(args) < 3 {
					return nil, errors.New("too few args for rgba")
				}
				rc := rgbaColor{}
				rc.r, err = strconv.ParseInt(args[0].String(), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("parse rgba r: %q", err)
				}
				rc.g, err = strconv.ParseInt(args[1].String(), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("parse rgba g: %q", err)
				}
				rc.b, err = strconv.ParseInt(args[2].String(), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("parse rgba b: %q", err)
				}
				if param[0].v == "rgba" {
					if len(args) < 4 {
						return nil, errors.New("too few args for rgba")
					}
					rc.a, err = strconv.ParseFloat(args[3].String(), 64)
					if err != nil {
						return nil, fmt.Errorf("parse rgba alpha: %q", err)
					}
				} else {
					rc.a = 1
				}
				c = rc.ToHSLA()
			case "hsl", "hsla":
				args := getArgs(param[2 : len(param)-1])
				if len(args) < 3 {
					return nil, errors.New("too few args for hsla")
				}
				c.hue, err = strconv.ParseInt(args[0].String(), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("parse hsla hue: %q", err)
				}
				c.saturation, err = parseAsPercent(args[1])
				if err != nil {
					return nil, fmt.Errorf("parse hsla saturation: %q", err)
				}
				c.lightness, err = parseAsPercent(args[2])
				if err != nil {
					return nil, fmt.Errorf("parse hsla lightness: %q", err)
				}
				if param[0].v == "hsla" {
					if len(args) < 4 {
						return nil, errors.New("too few args for hsla")
					}
					c.alpha, err = strconv.ParseFloat(args[3].String(), 64)
					if err != nil {
						return nil, fmt.Errorf("parse hsla alpha: %q", err)
					}
				} else {
					c.alpha = 1
				}
			case "transparent":
			default:
				return s, nil
			}
		default:
			return s, nil
		}
		var x int64
		switch s[i].v {
		case "spin":
			x, err = strconv.ParseInt(params[1].String(), 10, 64)
		default:
			x, err = parseAsPercent(params[1])
		}
		if err != nil {
			return nil, fmt.Errorf("parse 2nd param: %q", err)
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

type rgbaColor struct {
	r int64
	g int64
	b int64
	a float64
}

func (c rgbaColor) ToHSLA() hslaColor {
	r := float64(c.r) / 255
	g := float64(c.g) / 255
	b := float64(c.b) / 255
	min := math.Min(math.Min(r, g), b)
	max := math.Max(math.Max(r, g), b)
	d := max - min
	l := (min + max) / 2
	h := hslaColor{
		alpha: c.a,
	}
	if d != 0 {
		switch max {
		case r:
			h.hue = int64(math.Round(60*(g-b)/d)) + 0
		case g:
			h.hue = int64(math.Round(60*(b-r)/d)) + 120
		case b:
			h.hue = int64(math.Round(60*(r-g)/d)) + 240
		}
		h.hue = (h.hue + 360) % 360
		h.saturation = int64(math.Round(100 * d / (1 - math.Abs(2*l-1))))
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
