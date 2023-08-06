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
	"math"
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
	case tokenIdentifier:
		if _, err := expectSeq(s, i+1, false, tokenParensOpen); err != nil {
			return i + 1, 0, "", err
		}
		switch s[i].v {
		case "unit":
			j, args, err := parseArgs(s, i+1)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s args: %s", s[i].v, err)
			}
			if len(args) == 0 || len(args) > 2 {
				return i, 0, "", fmt.Errorf("%s expects one/two args", s[i].v)
			}
			_, a, _, err := parseNum(args[0], 0)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s first arg: %s", s[i].v, err)
			}
			if len(args) == 1 {
				return j, a, "", nil
			}
			unit := trimSpace(args[1])
			if len(unit) == 0 || len(unit[0].v) == 0 {
				return i, 0, "", fmt.Errorf("%s 2nd arg is empty", s[i].v)
			}
			return j, a, unit[0].v, nil
		case
			"ceil", "floor", "round",
			"percentage",
			"sqrt",
			"abs",
			"sin", "cos", "tan", "asin", "acos", "atan":
		case "pow", "mod":
			j, args, err := parseArgs(s, i+1)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s args: %s", s[i].v, err)
			}
			if len(args) != 2 {
				return i, 0, "", fmt.Errorf("%s expects two args", s[i].v)
			}
			_, a, aUnit, err := parseNum(args[0], 0)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s first arg: %s", s[i].v, err)
			}
			_, b, _, err := parseNum(args[1], 0)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s 2nd arg: %s", s[i].v, err)
			}
			switch s[i].v {
			case "pow":
				return j, math.Pow(a, b), aUnit, nil
			case "mod":
				return j, math.Mod(a, b), aUnit, nil
			}
		case "min", "max":
			j, args, err := parseArgs(s, i+1)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s args: %s", s[i].v, err)
			}
			if len(args) < 2 {
				return i, 0, "", fmt.Errorf("%s expects at least two args", s[i].v)
			}
			_, a, aUnit, err := parseNum(args[0], 0)
			if err != nil {
				return i, 0, "", fmt.Errorf("%s first arg: %s", s[i].v, err)
			}
			for idx, param := range args[1:] {
				var b float64
				_, b, _, err = parseNum(param, 0)
				if err != nil {
					return i, 0, "", fmt.Errorf("%s arg[%d]: %s", s[i].v, idx, err)
				}
				switch s[i].v {
				case "min":
					a = math.Min(a, b)
				case "max":
					a = math.Max(a, b)
				}
			}
			return j, a, aUnit, nil
		case "pi":
			if j, err := expectSeq(s, i+1, true, tokenParensOpen, tokenParensClose); err != nil {
				return i + 1, 0, "", err
			} else {
				return j, math.Pi, "", err
			}
		default:
			return i, 0, "", fmt.Errorf("unexpected name %q", s[i].v)
		}
		j, x, u, err := parseNum(s, i+1)
		if err != nil {
			return i + 1, 0, "", err
		}
		switch s[i].v {
		case "floor":
			return j, math.Floor(x), u, nil
		case "ceil":
			return j, math.Ceil(x), u, nil
		case "round":
			return j, math.Round(x), u, nil
		case "percentage":
			return j, x * 100, "%", nil
		case "sqrt":
			return j, math.Sqrt(x), u, nil
		case "abs":
			return j, math.Abs(x), u, nil
		case "sin":
			return j, math.Sin(x), u, nil
		case "cos":
			return j, math.Cos(x), u, nil
		case "tan":
			return j, math.Tan(x), u, nil
		case "asin":
			return j, math.Asin(x), u, nil
		case "acos":
			return j, math.Acos(x), u, nil
		case "atan":
			return j, math.Atan(x), u, nil
		}
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
		k, x, u, err := evalMathExpr(s[:j], i+1, 0)
		if err != nil {
			return k, 0, "", err
		}
		if k+consumeSpace(s[k:]) != j {
			return i, 0, "", fmt.Errorf("unexpected content before parens close: full=%q remaining=%q", s[i:j+1], s[k:j])
		}
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
			"vh", "vw",
			"%":
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
		evalSimple := o == i || (o > i && j > o+1)
		switch s[o].kind {
		case tokenPlus:
			if !evalSimple {
				return i, a, aUnit, nil
			}
			if l == tokenMinus {
				return i, a, aUnit, nil
			}
			j, b, bUnit, err = evalMathExpr(s, j, tokenPlus)
			if err != nil {
				return j, 0, "", err
			}
			unitOK = aUnit == bUnit
			if !unitOK &&
				(aUnit == "px" && bUnit == "" || aUnit == "" && bUnit == "px") {
				aUnit = "px"
				unitOK = true
			}
			a += b
		case tokenMinus:
			if !evalSimple {
				return i, a, aUnit, nil
			}
			if l == tokenMinus {
				return i, a, aUnit, nil
			}
			j, b, bUnit, err = evalMathExpr(s, j, tokenMinus)
			if err != nil {
				return j, 0, "", err
			}
			unitOK = aUnit == bUnit
			if !unitOK &&
				(aUnit == "px" && bUnit == "" || aUnit == "" && bUnit == "px") {
				aUnit = "px"
				unitOK = true
			}
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
	if isConstant(s) {
		return s
	}
	i, x, unit, err := evalMathExpr(s, 0, 0)
	if err != nil {
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
	if len(s) == i {
		return out
	}
	if s[i].IsSpace() {
		out = append(out, token{
			kind: space,
			v:    " ",
		})
		i++
	}
	out = append(out, evalMath(s[i:])...)
	return out
}
