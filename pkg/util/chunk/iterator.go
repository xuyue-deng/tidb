// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chunk

import "golang.org/x/sys/cpu"

var (
	_ Iterator = (*Iterator4Chunk)(nil)
	_ Iterator = (*iterator4RowPtr)(nil)
	_ Iterator = (*iterator4List)(nil)
	_ Iterator = (*Iterator4Slice)(nil)
	_ Iterator = (*iterator4RowContainer)(nil)
	_ Iterator = (*multiIterator)(nil)
)

// Iterator is used to iterate a number of rows.
//
//	for row := it.Begin(); row != it.End(); row = it.Next() {
//	    ...
//	}
type Iterator interface {
	// Begin resets the cursor of the iterator and returns the first Row.
	Begin() Row

	// Next returns the next Row.
	Next() Row

	// End returns the invalid end Row.
	End() Row

	// Len returns the length.
	Len() int

	// Current returns the current Row.
	Current() Row

	// ReachEnd reaches the end of iterator.
	ReachEnd()

	// Error returns none-nil error if anything wrong happens during the iteration.
	Error() error
}

// NewIterator4Slice returns a *Iterator4Slice for Row slice.
func NewIterator4Slice(rows []Row) *Iterator4Slice {
	return &Iterator4Slice{
		rows:   rows,
		cursor: 0,
	}
}

// Iterator4Slice is used to iterate rows inside a slice.
type Iterator4Slice struct {
	_      cpu.CacheLinePad
	rows   []Row
	cursor int
	_      cpu.CacheLinePad
}

// Begin implements the Iterator interface.
func (it *Iterator4Slice) Begin() Row {
	if it.Len() == 0 {
		return it.End()
	}
	it.cursor = 1
	return it.rows[0]
}

// Next implements the Iterator interface.
func (it *Iterator4Slice) Next() Row {
	if l := it.Len(); it.cursor >= l {
		it.cursor = l + 1
		return it.End()
	}
	row := it.rows[it.cursor]
	it.cursor++
	return row
}

// Current implements the Iterator interface.
func (it *Iterator4Slice) Current() Row {
	if it.cursor == 0 || it.cursor > it.Len() {
		return it.End()
	}
	return it.rows[it.cursor-1]
}

// End implements the Iterator interface.
func (*Iterator4Slice) End() Row {
	return Row{}
}

// ReachEnd implements the Iterator interface.
func (it *Iterator4Slice) ReachEnd() {
	it.cursor = it.Len() + 1
}

// Len implements the Iterator interface.
func (it *Iterator4Slice) Len() int {
	return len(it.rows)
}

// Reset iterator.rows and cursor.
func (it *Iterator4Slice) Reset(rows []Row) {
	it.rows = rows
	it.cursor = 0
}

// Error returns none-nil error if anything wrong happens during the iteration.
func (*Iterator4Slice) Error() error {
	return nil
}

// NewIterator4Chunk returns a iterator for Chunk.
func NewIterator4Chunk(chk *Chunk) *Iterator4Chunk {
	return &Iterator4Chunk{chk: chk}
}

// Iterator4Chunk is used to iterate rows inside a chunk.
type Iterator4Chunk struct {
	chk     *Chunk
	cursor  int32
	numRows int32
}

// Begin implements the Iterator interface.
func (it *Iterator4Chunk) Begin() Row {
	it.numRows = int32(it.chk.NumRows())
	if it.numRows == 0 {
		return it.End()
	}
	it.cursor = 1
	return it.chk.GetRow(0)
}

// Next implements the Iterator interface.
func (it *Iterator4Chunk) Next() Row {
	if it.cursor >= it.numRows {
		it.cursor = it.numRows + 1
		return it.End()
	}
	row := it.chk.GetRow(int(it.cursor))
	it.cursor++
	return row
}

// Current implements the Iterator interface.
func (it *Iterator4Chunk) Current() Row {
	if it.cursor == 0 || int(it.cursor) > it.Len() {
		return it.End()
	}
	return it.chk.GetRow(int(it.cursor) - 1)
}

// End implements the Iterator interface.
func (*Iterator4Chunk) End() Row {
	return Row{}
}

// ReachEnd implements the Iterator interface.
func (it *Iterator4Chunk) ReachEnd() {
	it.cursor = int32(it.Len() + 1)
}

// Len implements the Iterator interface
func (it *Iterator4Chunk) Len() int {
	return it.chk.NumRows()
}

// GetChunk returns the chunk stored in the Iterator4Chunk
func (it *Iterator4Chunk) GetChunk() *Chunk {
	return it.chk
}

// ResetChunk reset the chunk this iter holds
func (it *Iterator4Chunk) ResetChunk(chk *Chunk) {
	it.chk = chk
}

// Error returns none-nil error if anything wrong happens during the iteration.
func (*Iterator4Chunk) Error() error {
	return nil
}

// NewIterator4List returns a Iterator for List.
func NewIterator4List(li *List) Iterator {
	return &iterator4List{li: li}
}

type iterator4List struct {
	li        *List
	chkCursor int
	rowCursor int
}

// Begin implements the Iterator interface.
func (it *iterator4List) Begin() Row {
	if it.li.NumChunks() == 0 {
		return it.End()
	}
	chk := it.li.GetChunk(0)
	row := chk.GetRow(0)
	if chk.NumRows() == 1 {
		it.chkCursor = 1
		it.rowCursor = 0
	} else {
		it.chkCursor = 0
		it.rowCursor = 1
	}
	return row
}

// Next implements the Iterator interface.
func (it *iterator4List) Next() Row {
	if it.chkCursor >= it.li.NumChunks() {
		it.chkCursor = it.li.NumChunks() + 1
		return it.End()
	}
	chk := it.li.GetChunk(it.chkCursor)
	row := chk.GetRow(it.rowCursor)
	it.rowCursor++
	if it.rowCursor == chk.NumRows() {
		it.rowCursor = 0
		it.chkCursor++
	}
	return row
}

// Current implements the Iterator interface.
func (it *iterator4List) Current() Row {
	if (it.chkCursor == 0 && it.rowCursor == 0) || it.chkCursor > it.li.NumChunks() {
		return it.End()
	}
	if it.rowCursor == 0 {
		curChk := it.li.GetChunk(it.chkCursor - 1)
		return curChk.GetRow(curChk.NumRows() - 1)
	}
	curChk := it.li.GetChunk(it.chkCursor)
	return curChk.GetRow(it.rowCursor - 1)
}

// End implements the Iterator interface.
func (*iterator4List) End() Row {
	return Row{}
}

// ReachEnd implements the Iterator interface.
func (it *iterator4List) ReachEnd() {
	it.chkCursor = it.li.NumChunks() + 1
}

// Len implements the Iterator interface.
func (it *iterator4List) Len() int {
	return it.li.Len()
}

// Error returns none-nil error if anything wrong happens during the iteration.
func (*iterator4List) Error() error {
	return nil
}

// NewIterator4RowPtr returns a Iterator for RowPtrs.
func NewIterator4RowPtr(li *List, ptrs []RowPtr) Iterator {
	return &iterator4RowPtr{li: li, ptrs: ptrs}
}

type iterator4RowPtr struct {
	li     *List
	ptrs   []RowPtr
	cursor int
}

// Begin implements the Iterator interface.
func (it *iterator4RowPtr) Begin() Row {
	if it.Len() == 0 {
		return it.End()
	}
	it.cursor = 1
	return it.li.GetRow(it.ptrs[0])
}

// Next implements the Iterator interface.
func (it *iterator4RowPtr) Next() Row {
	if l := it.Len(); it.cursor >= l {
		it.cursor = l + 1
		return it.End()
	}
	row := it.li.GetRow(it.ptrs[it.cursor])
	it.cursor++
	return row
}

// Current implements the Iterator interface.
func (it *iterator4RowPtr) Current() Row {
	if it.cursor == 0 || it.cursor > it.Len() {
		return it.End()
	}
	return it.li.GetRow(it.ptrs[it.cursor-1])
}

// End implements the Iterator interface.
func (*iterator4RowPtr) End() Row {
	return Row{}
}

// ReachEnd implements the Iterator interface.
func (it *iterator4RowPtr) ReachEnd() {
	it.cursor = it.Len() + 1
}

// Len implements the Iterator interface.
func (it *iterator4RowPtr) Len() int {
	return len(it.ptrs)
}

// Error returns none-nil error if anything wrong happens during the iteration.
func (*iterator4RowPtr) Error() error {
	return nil
}

// NewIterator4RowContainer create a new iterator for RowContainer
func NewIterator4RowContainer(c *RowContainer) *iterator4RowContainer {
	return &iterator4RowContainer{c: c}
}

type iterator4RowContainer struct {
	c      *RowContainer
	chkIdx int
	rowIdx int
	err    error
}

// Len implements the Iterator interface.
func (it *iterator4RowContainer) Len() int {
	return it.c.NumRow()
}

func (it *iterator4RowContainer) setNextPtr() {
	it.rowIdx++
	if it.rowIdx == it.c.NumRowsOfChunk(it.chkIdx) {
		it.rowIdx = 0
		it.chkIdx++
	}
}

// Begin implements the Iterator interface.
func (it *iterator4RowContainer) Begin() Row {
	it.chkIdx, it.rowIdx = 0, -1
	return it.Next()
}

// Next implements the Iterator interface.
func (it *iterator4RowContainer) Next() Row {
	if it.chkIdx >= it.c.NumChunks() {
		it.ReachEnd()
		return it.End()
	}
	it.setNextPtr()
	return it.Current()
}

// Current implements the Iterator interface.
func (it *iterator4RowContainer) Current() Row {
	if it.rowIdx < 0 || it.chkIdx >= it.c.NumChunks() {
		return it.End()
	}
	row, err := it.c.GetRow(RowPtr{uint32(it.chkIdx), uint32(it.rowIdx)})
	if err != nil {
		it.err = err
		it.ReachEnd()
		return it.End()
	}
	return row
}

// End implements the Iterator interface.
func (*iterator4RowContainer) End() Row {
	return Row{}
}

// ReachEnd implements the Iterator interface.
func (it *iterator4RowContainer) ReachEnd() {
	it.chkIdx, it.rowIdx = it.c.NumChunks(), 0
}

// Error returns none-nil error if anything wrong happens during the iteration.
func (it *iterator4RowContainer) Error() error {
	return it.err
}

// multiIterator joins several iterators together to form a new iterator
type multiIterator struct {
	iters   []Iterator
	numIter int
	length  int
	curPtr  int
	curIter Iterator
	err     error
}

// NewMultiIterator creates a new multiIterator
func NewMultiIterator(iters ...Iterator) Iterator {
	iter := &multiIterator{}
	for i := range iters {
		if iters[i].Len() > 0 {
			iter.iters = append(iter.iters, iters[i])
			iter.length += iters[i].Len()
		}
	}
	iter.numIter = len(iter.iters)
	return iter
}

// Len implements the Iterator interface.
func (it *multiIterator) Len() int {
	return it.length
}

// Begin implements the Iterator interface.
func (it *multiIterator) Begin() Row {
	it.curPtr = 0
	if it.numIter > 0 {
		it.curIter = it.iters[0]
		it.curIter.Begin()
	}
	return it.Current()
}

// Next implements the Iterator interface.
func (it *multiIterator) Next() Row {
	if it.curPtr == it.numIter {
		return it.End()
	}
	next := it.curIter.Next()
	if next == it.curIter.End() {
		it.err = it.curIter.Error()
		if it.err != nil {
			it.ReachEnd()
			return it.End()
		}
		it.curPtr++
		if it.curPtr == it.numIter {
			return it.End()
		}
		it.curIter = it.iters[it.curPtr]
		next = it.curIter.Begin()
	}
	return next
}

// Current implements the Iterator interface.
func (it *multiIterator) Current() Row {
	if it.curPtr == it.numIter {
		return it.End()
	}
	row := it.curIter.Current()
	if row == it.curIter.End() {
		it.err = it.curIter.Error()
	}
	return row
}

// End implements the Iterator interface.
func (*multiIterator) End() Row {
	return Row{}
}

// ReachEnd implements the Iterator interface.
func (it *multiIterator) ReachEnd() {
	it.curPtr = it.numIter
}

// Error returns none-nil error if anything wrong happens during the iteration.
func (it *multiIterator) Error() error {
	return it.err
}
