package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type treeWrapper struct {
	widget.BaseWidget
	tree         *widget.Tree
	onRightClick func(id widget.TreeNodeID, pos fyne.Position)
	getNodeID    func() widget.TreeNodeID
}

func newTreeWrapper(tree *widget.Tree, getNodeID func() widget.TreeNodeID, onRightClick func(widget.TreeNodeID, fyne.Position)) *treeWrapper {
	w := &treeWrapper{tree: tree, getNodeID: getNodeID, onRightClick: onRightClick}
	w.ExtendBaseWidget(w)
	return w
}

func (w *treeWrapper) TappedSecondary(e *fyne.PointEvent) {
	if w.onRightClick == nil {
		return
	}
	id := widget.TreeNodeID("")
	if w.getNodeID != nil {
		id = w.getNodeID()
	}
	if id == "" {
		return
	}
	w.onRightClick(id, e.AbsolutePosition)
}

func (w *treeWrapper) CreateRenderer() fyne.WidgetRenderer {
	return w.tree.CreateRenderer()
}

func (w *treeWrapper) MinSize() fyne.Size {
	return w.tree.MinSize()
}