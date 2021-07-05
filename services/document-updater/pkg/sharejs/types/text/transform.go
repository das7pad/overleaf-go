// Golang port of the Overleaf document-updater service
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

package text

import (
	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/types"
)

func Transform(op, otherOp types.Op) (types.Op, error) {
	if len(otherOp) == 0 {
		return op, nil
	}
	if len(otherOp) == 1 && len(op) == 1 {
		transformed := make(types.Op, 0, 1)
		return transformComponent(transformed, op[0], otherOp[0], leftSide)
	}
	return transformX(op, otherOp)
}

type transformSide int

const (
	leftSide transformSide = iota
	rightSide
)

var deleteOpsDeleteDifferentText = &errors.JavaScriptError{
	Message: "Delete ops delete different text in the same region of the document",
}

func transformPosition(p int64, c types.Component, insertAfter bool) int64 {
	if c.IsInsertion() {
		if c.Position < p || (c.Position == p && insertAfter) {
			return p + int64(len(c.Insertion))
		}
		return p
	}
	if c.IsDeletion() {
		if p <= c.Position {
			return p
		}
		delSize := int64(len(c.Deletion))
		if p <= c.Position+delSize {
			return c.Position
		}
		return p - delSize
	}
	// else: comment type
	return p
}

func transformComponent(op types.Op, c, otherC types.Component, side transformSide) (types.Op, error) {
	if c.IsInsertion() {
		c.Position = transformPosition(c.Position, otherC, side == rightSide)
		return appendOp(op, c), nil
	}
	if c.IsDeletion() {
		if otherC.IsInsertion() {
			p := c.Position
			d := c.Deletion
			if c.Position < otherC.Position {
				edge := otherC.Position - c.Position
				if edge > int64(len(d)) {
					edge = int64(len(d))
				}
				c.Deletion = d[:edge]
				op = appendOp(op, c)
				d = d[edge:]
			}
			if len(d) == 0 {
				return op, nil
			}
			return appendOp(op, types.Component{
				Deletion: d,
				Position: p + int64(len(otherC.Insertion)),
			}), nil
		}
		if otherC.IsDeletion() {
			cEndBeforeOp := c.Position + int64(len(c.Deletion))
			otherCEndBeforeOp := otherC.Position + int64(len(otherC.Deletion))
			if c.Position >= otherCEndBeforeOp {
				c.Position -= int64(len(otherC.Deletion))
				return appendOp(op, c), nil
			}
			if cEndBeforeOp <= otherC.Position {
				return appendOp(op, c), nil
			}

			intersectStart := c.Position
			intersectEnd := cEndBeforeOp

			d := ""
			if c.Position < otherC.Position {
				intersectStart = otherC.Position
				d = c.Deletion[:otherC.Position-c.Position]
			}
			if cEndBeforeOp > otherCEndBeforeOp {
				intersectEnd = otherCEndBeforeOp
				d += c.Deletion[otherCEndBeforeOp-c.Position:]
			}
			cIntersect := c.Deletion[intersectStart-c.Position : intersectEnd-c.Position]
			otherCIntersect := otherC.Deletion[intersectStart-otherC.Position : intersectEnd-otherC.Position]

			if cIntersect != otherCIntersect {
				return nil, deleteOpsDeleteDifferentText
			}

			if len(d) == 0 {
				return op, nil
			}

			c.Deletion = d
			c.Position = transformPosition(c.Position, otherC, false)
			return appendOp(op, c), nil
		}

		// else: comment type
		return appendOp(op, c), nil
	}

	// else: comment type
	if otherC.IsInsertion() {
		cLen := int64(len(c.Comment))
		if c.Position < otherC.Position && otherC.Position < c.Position+cLen {
			offset := otherC.Position - c.Position
			c.Comment = inject(c.Comment, offset, otherC.Insertion)
			return appendOp(op, c), nil
		}
		c.Position = transformPosition(c.Position, otherC, true)
		return appendOp(op, c), nil
	}
	if otherC.IsDeletion() {
		cEnd := c.Position + int64(len(c.Comment))
		otherCEndBeforeOp := otherC.Position + int64(len(otherC.Deletion))

		if c.Position >= otherCEndBeforeOp {
			c.Position -= int64(len(otherC.Deletion))
			return appendOp(op, c), nil
		}
		if cEnd <= otherC.Position {
			return appendOp(op, c), nil
		}

		intersectStart := c.Position
		intersectEnd := cEnd

		cc := ""
		if c.Position < otherC.Position {
			intersectStart = otherC.Position
			cc = c.Comment[:otherC.Position-c.Position]
		}
		if cEnd > otherCEndBeforeOp {
			intersectEnd = otherCEndBeforeOp
			cc += c.Comment[otherCEndBeforeOp-c.Position:]
		}
		cIntersect := c.Comment[intersectStart-c.Position : intersectEnd-c.Position]
		otherCIntersect := otherC.Deletion[intersectStart-otherC.Position : intersectEnd-otherC.Position]

		if cIntersect != otherCIntersect {
			return nil, deleteOpsDeleteDifferentText
		}

		c.Comment = cc
		c.Position = transformPosition(c.Position, otherC, false)
		return appendOp(op, c), nil
	}

	// else: comment type
	return appendOp(op, c), nil
}

func transformComponentX(left, right types.Component, destLeft, destRight types.Op) (types.Op, types.Op, error) {
	var err error
	destLeft, err = transformComponent(destLeft, left, right, leftSide)
	if err != nil {
		return nil, nil, err
	}
	destRight, err = transformComponent(destRight, right, left, rightSide)
	if err != nil {
		return nil, nil, err
	}
	return destLeft, destRight, nil
}

func transformX(left, right types.Op) (types.Op, error) {
	var err error
	for _, component := range right {
		transformedLeft := make(types.Op, 0, len(left))

		k := 0
	inner:
		for k < len(left) {
			nextC := make(types.Op, 0)

			transformedLeft, nextC, err = transformComponentX(
				left[k], component, transformedLeft, nextC,
			)
			if err != nil {
				return nil, err
			}
			k++

			switch len(nextC) {
			case 1:
				component = nextC[0]
			case 0:
				for _, c := range left[k:] {
					transformedLeft = appendOp(transformedLeft, c)
				}
				break inner
			default:
				left2, err2 := transformX(left[k:], nextC)
				if err2 != nil {
					return nil, err2
				}
				for _, c := range left2 {
					transformedLeft = appendOp(transformedLeft, c)
				}
				break inner
			}
		}
		left = transformedLeft
	}
	return left, nil
}
