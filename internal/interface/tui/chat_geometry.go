package tui

// Chat block geometry. One home for the widths the transcript renders
// against, so the bubbles, the nested rows and the right-aligned tool
// duration all agree on where the transcript's right edge is.

const (
	// Columns a response bubble spends before its content starts: its
	// left border and its left padding.
	bubbleChromeCols = 2

	// A turn block indents every nested row one column inside the
	// bubble, so the block's own chrome is the bubble's plus that indent.
	turnBlockChromeCols = bubbleChromeCols + 1

	// The columns a tool row's own renderer prepends: the expand caret
	// and the space after it.
	toolRowCaretCols = 2
)

// bubbleWidth is the width handed to the user/assistant bubble styles.
// The transcript spans the frame's inner width: the bubble styles draw a
// left border only and keep their padding inside the budget, so the
// value here is the block's visible width minus that border. Sizing
// every bubble from one place is what keeps the transcript's right edge
// on the chrome's right edge (the composer rule's edge) — and it is the
// edge a right-aligned tool duration lands against.
func (m ChatModel) bubbleWidth() int {
	if m.width < 2 {
		return 1
	}
	return m.width - 1
}

// toolRowWidth is the budget for a tool row, given how many columns its
// container spends before the row itself begins (0 for a row drawn
// straight into the pane).
//
// The row pads itself out to exactly this budget, which is what puts its
// right-aligned duration on the transcript's right edge — and what makes
// an over-generous budget a wrap instead of a silently short row.
func (m ChatModel) toolRowWidth(containerCols int) int {
	w := m.width - containerCols - toolRowCaretCols
	if w < 1 {
		return 1
	}
	return w
}
