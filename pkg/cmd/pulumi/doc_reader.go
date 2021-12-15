// Copyright 2016-2021, Pulumi Corporation.
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

package main

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/markdown-kit/renderer"
	mdk "github.com/pgavlin/markdown-kit/tview"
	"github.com/rivo/tview"
	"github.com/skratchdot/open-golang/open"
)

const helpText = `Ctrl+o: open the selected URL in the default browser

]: select the next URL

[: select the previous URL

}: select the next heading

{: select the previous heading

<: go back to the previous selection
`

func textDimensions(text string) (int, int) {
	s, w, h := "", 0, 0
	for len(text) != 0 {
		i := strings.Index(text, "\n")
		if i == -1 {
			s, text = text, ""
		} else {
			s, text = text[:i], text[i+1:]
		}
		if sw := tview.TaggedStringWidth(s); sw > w {
			w = sw
		}
		h++
	}
	return w, h
}

type textDialog struct {
	x, y, w, h int
	text       string
	textWidth  int
	textHeight int
	textView   *tview.TextView
	persistent bool
}

func newTextDialog(text, title string) *textDialog {
	textView := tview.NewTextView()
	textView.SetBorder(true).SetTitle(title)
	textView.SetWrap(true).SetWordWrap(true)
	textView.SetText(text)

	tw, th := textDimensions(text)

	return &textDialog{
		text:       text,
		textWidth:  tw,
		textHeight: th,
		textView:   textView,
	}
}

func (td *textDialog) SetText(text string) {
	td.textView.SetText(text)
	td.textWidth, td.textHeight = textDimensions(text)
}

func (td *textDialog) Draw(screen tcell.Screen) {
	screenWidth, screenHeight := screen.Size()

	w, h := screenWidth/2, screenHeight/2

	// We add 2 below to account for the text view's border.
	textWidth, textHeight := td.textWidth+2, td.textHeight+2

	if w > textWidth {
		w = textWidth
	} else {
		// We're going to be wrapping the text. Recalculate the text height with word wrap.
		textHeight = len(tview.WordWrap(td.text, w)) + 2
	}
	if h > textHeight {
		h = textHeight
	}

	x, y := (screenWidth-w)/2, (screenHeight-h)/2
	td.textView.SetRect(x, y, w, h)
	td.textView.Draw(screen)
}

func (td *textDialog) GetRect() (int, int, int, int) {
	return td.x, td.y, td.w, td.h
}

func (td *textDialog) SetRect(x, y, w, h int) {
	td.x, td.y, td.w, td.h = x, y, w, h
}

func (td *textDialog) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return td.textView.InputHandler()
}

type MouseHandlerFunc = func(action tview.MouseAction, event *tcell.EventMouse,
	setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive)

func (td *textDialog) MouseHandler() MouseHandlerFunc {
	return td.textView.MouseHandler()
}

func (td *textDialog) Focus(delegate func(p tview.Primitive)) {
	td.textView.Focus(delegate)
}

func (td *textDialog) Blur() {
	td.textView.Blur()
}

func (td *textDialog) HasFocus() bool {
	return td.textView.HasFocus()
}

func getDocumentAnchor(url string) (string, bool) {
	if !strings.HasPrefix(url, "#") {
		return "", false
	}
	return url[1:], true
}

func openInBrowser(url string) error {
	if url == "" {
		return fmt.Errorf("missing URL")
	}
	return open.Run(url)
}

type markdownReader struct {
	view *mdk.MarkdownView

	app *tview.Application

	hasFocus      bool
	focused       tview.Primitive
	lastFocused   tview.Primitive
	inputHandler  func(event *tcell.EventKey, setFocus func(p tview.Primitive))
	visibleDialog *textDialog

	helpDialog *textDialog
	rootPages  *tview.Pages

	backstack []*renderer.NodeSpan

	externalURLResolver func(url string, reader *markdownReader) (bool, error)
}

func newMarkdownReader(name, source string, theme *chroma.Style, app *tview.Application) *markdownReader {
	r := &markdownReader{
		view:       mdk.NewMarkdownView(theme),
		app:        app,
		helpDialog: newTextDialog(helpText, "Help"),
	}

	r.view.SetText(name, source)
	r.view.SetGutter(true)

	rootPages := tview.NewPages()
	rootPages.AddAndSwitchToPage("markdown", r.view, true)
	rootPages.AddPage("help", r.helpDialog, true, false)
	r.rootPages = rootPages

	r.focused = r.view

	return r
}

func (r *markdownReader) SetSource(name, source string) {
	view := mdk.NewMarkdownView(nil)
	view.SetText(name, source)
	r.view.SetGutter(true)

	r.rootPages.AddAndSwitchToPage("markdown", view, true)
}

func (r *markdownReader) Draw(screen tcell.Screen) {
	r.rootPages.Draw(screen)
}

func (r *markdownReader) GetRect() (int, int, int, int) {
	return r.rootPages.GetRect()
}

func (r *markdownReader) SetRect(xc, yc, width, height int) {
	r.rootPages.SetRect(xc, yc, width, height)
}

func (r *markdownReader) focusedLink() string {
	if span := r.view.Selection(); span != nil {
		switch node := span.Node.(type) {
		case *ast.AutoLink:
			return string(node.URL(r.view.GetMarkdown()))
		case *ast.Link:
			return string(node.Destination)
		}
	}
	return ""
}

func (r *markdownReader) OpenLink() {
	link := r.focusedLink()
	if anchor, ok := getDocumentAnchor(link); ok {
		selection := r.view.Selection()
		if r.view.SelectAnchor(anchor) && selection != nil {
			r.backstack = append(r.backstack, selection)
		}
		return
	}
	if r.externalURLResolver != nil {
		ok, err := r.externalURLResolver(link, r)
		if err != nil {
			r.showErrorDialog("failed to resolve url", err)
		}
		if ok {
			return
		}
	}

	if err := openInBrowser(link); err != nil {
		r.showErrorDialog("opening issue", err)
	}

}

func (r *markdownReader) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		event = func() *tcell.EventKey {
			if r.visibleDialog != nil {
				if event.Key() == tcell.KeyEscape || event.Rune() == 'h' || event.Rune() == '?' && r.visibleDialog == r.helpDialog {
					r.hideDialog()
					return nil
				}
				return event
			}

			switch event.Key() {
			case tcell.KeyCtrlO:
				r.OpenLink()
			case tcell.KeyRune:
				switch event.Rune() {
				case '<':
					if len(r.backstack) != 0 {
						last := r.backstack[len(r.backstack)-1]
						r.backstack = r.backstack[:len(r.backstack)-1]
						r.view.SelectSpan(last, true)
					}
				case 'h', '?':
					// Show the help
					r.showDialog(r.helpDialog)
					return nil
				case 'q':
					// quit the app
					r.app.Stop()
				}
			}
			return event
		}()
		if event != nil && r.focused != nil {
			if handler := r.focused.InputHandler(); handler != nil {
				handler(event, r.setFocus)
			}
		}
	}
}

func (r *markdownReader) MouseHandler() MouseHandlerFunc {
	return r.rootPages.MouseHandler()
}

func (r *markdownReader) Focus(delegate func(p tview.Primitive)) {
	r.hasFocus = true
	if r.focused != nil {
		r.focused.Focus(r.setFocus)
	}
}

func (r *markdownReader) Blur() {
	r.hasFocus = false
	if r.focused != nil {
		r.focused.Blur()
	}
}

func (r *markdownReader) HasFocus() bool {
	return r.hasFocus
}

func (r *markdownReader) setFocus(p tview.Primitive) {
	r.lastFocused = r.focused

	var doSetFocus func(p tview.Primitive)
	doSetFocus = func(p tview.Primitive) {
		if r.focused != nil {
			r.focused.Blur()
		}

		r.focused = p
		r.inputHandler = p.InputHandler()
		r.focused.Focus(doSetFocus)
	}

	doSetFocus(p)
}

func (r *markdownReader) showDialog(d *textDialog) {
	if r.visibleDialog != nil && r.visibleDialog.persistent {
		return
	}

	r.rootPages.AddPage("dialog", d, true, true)
	r.setFocus(d)
	r.visibleDialog = d
}

func (r *markdownReader) hideDialog() {
	if r.visibleDialog != nil && r.visibleDialog.persistent {
		return
	}

	r.rootPages.HidePage("dialog")
	r.setFocus(r.lastFocused)
	r.visibleDialog = nil
}

func (r *markdownReader) showErrorDialog(action string, err error) {
	r.showDialog(newTextDialog(fmt.Sprintf("Error %v: %v", action, err.Error()), "Error"))
}
