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
	"errors"
	"fmt"
	"strconv"
)

func parseNum(s tokens, i int) (int, float64, string, error) {
	i += consumeSpace(s[i:])
	if len(s) < i+1 {
		return i, 0, "", errors.New("num too short")
	}
	sign := float64(1)
	if s[i].kind == tokenMinus {
		sign = -1
		i += 1
	}
	if len(s) < i+1 {
		return i, 0, "", errors.New("num ended unexpectedly with minus")
	}
	switch s[i].kind {
	case tokenParensOpen:
		l := 0
		j := i
		for ; j < len(s); j++ {
			switch s[j].kind {
			case tokenParensOpen:
				l++
			case tokenParensClose:
				l--
			}
			if l == 0 {
				break
			}
		}
		if l > 0 {
			return i, 0, "", errors.New("imbalanced parens")
		}
		j, x, u, err := evalMathExpr(s[:j], i+1, 0)
		return j + 1, x * sign, u, err
	case tokenNum:
	default:
		return i, 0, "", errors.New("invalid num")
	}
	x, err := strconv.ParseFloat(s[i].v, 64)
	if err != nil {
		return i, 0, "", err
	}
	i++
	if len(s) > i && s[i].kind == tokenIdentifier {
		switch s[i].v {
		case
			"px",
			"em", "rem",
			"vh", "vw":
			return i + 1, x * sign, s[i].v, nil
		default:
			return i, x * sign, "", nil
		}
	}
	return i, x * sign, "", nil
}

func evalMathExpr(s tokens, i int, l kind) (int, float64, string, error) {
	if len(s) <= i {
		return i, 0, "", errors.New("empty math expression")
	}
	i, a, aUnit, err := parseNum(s, i)
	if err != nil {
		return i, 0, "", err
	}
	var b float64
	var bUnit string
	for len(s) > i+1 {
		unitOK := false
		o := i + consumeSpace(s[i:])
		j := o + 1 + consumeSpace(s[o+1:])
		switch s[o].kind {
		case tokenPlus:
			if l == tokenMinus {
				return i, a, aUnit, nil
			}
			j, b, bUnit, err = evalMathExpr(s, j, tokenPlus)
			if err != nil {
				return j, 0, "", err
			}
			unitOK = aUnit == bUnit
			a += b
		case tokenMinus:
			if l == tokenMinus {
				return i, a, aUnit, nil
			}
			j, b, bUnit, err = evalMathExpr(s, j, tokenMinus)
			if err != nil {
				return j, 0, "", err
			}
			unitOK = aUnit == bUnit
			a -= b
		case tokenSlash:
			j, b, bUnit, err = parseNum(s, j)
			if err != nil {
				return j, 0, "", err
			}
			unitOK = aUnit == bUnit
			if unitOK {
				aUnit = ""
			} else if len(bUnit) == 0 {
				unitOK = true
			}
			a /= b
		case tokenStar:
			j, b, bUnit, err = parseNum(s, j)
			if err != nil {
				return j, 0, "", err
			}
			unitOK = !(len(aUnit) > 0 && len(bUnit) > 0)
			if unitOK && len(bUnit) > 0 {
				aUnit = bUnit
			}
			a *= b
		default:
			return i, a, aUnit, nil
		}
		if !unitOK {
			return i, 0, "", fmt.Errorf("unexpected units %q %s %q", aUnit, s[i].v, bUnit)
		}
		i = j
	}
	return i, a, aUnit, nil
}

func evalMath(s tokens) tokens {
	i, x, unit, err := evalMathExpr(s, 0, 0)
	if err != nil {
		// TODO: propagate error
		return s
	}
	n := 1 + len(s) - i
	if x < 0 {
		n += 1
	}
	if len(unit) > 0 {
		n += 1
	}
	out := make(tokens, 0, n)
	if x < 0 {
		x *= -1
		out = append(out, token{
			kind: tokenMinus,
			v:    "-",
		})
	}
	out = append(out, token{
		kind: tokenNum,
		v:    strconv.FormatFloat(x, 'f', -1, 64),
	})
	if len(unit) > 0 {
		out = append(out, token{
			kind: tokenIdentifier,
			v:    unit,
		})
	}
	out = append(out, s[i:]...)
	return out
}
