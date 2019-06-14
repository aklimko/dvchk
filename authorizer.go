package main

import (
	"bufio"
	"fmt"
	"github.com/howeyc/gopass"
	"os"
	"strings"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

const (
	marked   = "[X]"
	unmarked = "[ ]"
	tips     = "↑/↓ - move up/down | Space - (un)mark | Enter - authorize | Ctrl+c - finish " +
		"| Ctrl+a/Ctrl+l - mark/unmark all | Home/End - scroll to top/bottom"
)

type Status int

const (
	statusContinue Status = iota
	statusFinish
	statusIgnore
)

type ImageChoice struct {
	image    *ImageAuthUrl
	Marked   bool
	Position int
}

type Credentials struct {
	Username string
	Password string
}

type Widgets struct {
	list *widgets.List
	tips *widgets.Paragraph
}

type Authorizer struct {
	tagDownloader        TagDownloader
	ic                   []*ImageChoice
	widgets              *Widgets
	storage              *ImageStorage
	unauthorizedToRemove []int
}

func NewAuthorizer(tagDownloader TagDownloader, storage *ImageStorage) *Authorizer {
	return &Authorizer{tagDownloader: tagDownloader, storage: storage}
}

func (a *Authorizer) Authorize() {
	for a.authorizeContinuously() == statusContinue {
	}

	fmt.Println()
}

func (a *Authorizer) authorizeContinuously() Status {
	if len(a.storage.Unauthorized) == 0 {
		return statusFinish
	}

	a.unauthorizedToRemove = nil

	err := ui.Init()
	if err != nil {
		fmt.Printf("Failed to initialize termui: %v\n", err)
		return statusFinish
	}

	a.createImageChoices(a.storage.Unauthorized)

	a.createWidgets()
	a.renderWidgets()

	result := a.handleKeyInput()
	return result
}

func (a *Authorizer) createImageChoices(images []*ImageAuthUrl) {
	var ic []*ImageChoice
	for i, img := range images {
		ic = append(ic, &ImageChoice{image: img, Marked: false, Position: i})
	}
	a.ic = ic
}

func (a *Authorizer) createWidgets() {
	a.widgets = &Widgets{}

	a.createListWidget()
	a.createTipsWidget()
}

func (a *Authorizer) createListWidget() {
	a.widgets.list = widgets.NewList()
	a.widgets.list.Title = "Unauthorized images"
	a.widgets.list.TitleStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	a.widgets.list.TextStyle = ui.NewStyle(ui.ColorYellow)
	a.widgets.list.SelectedRowStyle = ui.NewStyle(ui.ColorBlue)
	a.widgets.list.WrapText = false
	a.fillListContent()

	w, h := ui.TerminalDimensions()
	a.widgets.list.SetRect(0, 0, w, h-5)
}

func (a *Authorizer) fillListContent() {
	var rows []string
	for _, i := range a.ic {
		rowValue := fmt.Sprintf("%s %s", unmarked, i.image.LocalFullName)
		rows = append(rows, rowValue)
	}

	a.widgets.list.Rows = rows
}

func (a *Authorizer) createTipsWidget() {
	a.widgets.tips = widgets.NewParagraph()
	a.widgets.tips.Text = tips
	a.widgets.tips.WrapText = true
	a.widgets.tips.Border = false

	w, h := ui.TerminalDimensions()
	a.widgets.tips.SetRect(0, h-5, w, h)
}

func (a *Authorizer) renderWidgets() {
	ui.Render(a.widgets.list, a.widgets.tips)
}

func (a *Authorizer) handleKeyInput() Status {
	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "<C-c>":
			ui.Close()
			return statusFinish
		case "<Up>":
			a.widgets.list.ScrollUp()
		case "<Down>":
			a.widgets.list.ScrollDown()
		case "<Enter>":
			status := a.authenticateMarkedImages()
			if status != statusIgnore {
				return status
			}
		case "<Space>":
			a.toggleSelectedRowMark()
		case "<Resize>":
			a.resizeWidgetsOnTerminalResize()
		case "<C-a>":
			a.markAllRows()
		case "<C-l>":
			a.unmarkAllRows()
		case "<Home>":
			a.widgets.list.ScrollTop()
		case "<End>":
			a.widgets.list.ScrollBottom()
		}

		a.renderWidgets()
	}
}

func (a *Authorizer) authenticateMarkedImages() Status {
	markedImages := a.getMarkedImages()

	if len(markedImages) == 0 {
		return statusIgnore
	}

	ui.Close()

	credentials, err := readCredentials()
	if err != nil {
		fmt.Println(err)
		return statusFinish
	}

	a.getMarkedImageTagsAuthenticated(markedImages, credentials)
	a.removeImagesFromUnauthorized()

	return statusContinue
}

func (a *Authorizer) getMarkedImages() []*ImageChoice {
	var markedImages []*ImageChoice
	for _, image := range a.ic {
		if image.Marked {
			markedImages = append(markedImages, image)
		}
	}
	return markedImages
}

func (a *Authorizer) getMarkedImageTagsAuthenticated(markedImages []*ImageChoice, credentials Credentials) {
	for _, image := range markedImages {
		tags, err := a.tagDownloader.DownloadWithAuth(image.image, credentials)
		if err != nil {
			fmt.Println(err)
			continue
		}

		a.storage.addSuccessful(&ImageContext{Image: image.image.Image, Tags: tags})
		a.unauthorizedToRemove = append(a.unauthorizedToRemove, image.Position)
	}
}

func (a *Authorizer) removeImagesFromUnauthorized() {
	for i := len(a.unauthorizedToRemove) - 1; i >= 0; i-- {
		position := a.unauthorizedToRemove[i]
		a.removeImageFromUnauthorized(position)
	}
}

func (a *Authorizer) removeImageFromUnauthorized(i int) {
	a.storage.Unauthorized = append(a.storage.Unauthorized[:i], a.storage.Unauthorized[i+1:]...)
}

func (a *Authorizer) toggleSelectedRowMark() {
	sr := a.widgets.list.SelectedRow

	a.ic[sr].Marked = !a.ic[sr].Marked

	var previous, current string
	if a.ic[sr].Marked {
		previous, current = unmarked, marked
	} else {
		previous, current = marked, unmarked
	}

	a.widgets.list.Rows[sr] = strings.Replace(a.widgets.list.Rows[sr], previous, current, 1)
}

func (a Authorizer) resizeWidgetsOnTerminalResize() {
	w, h := ui.TerminalDimensions()

	a.widgets.list.SetRect(0, 0, w, h-5)
	a.widgets.tips.SetRect(0, h-5, w, h)
}

func (a *Authorizer) markAllRows() {
	for i, image := range a.ic {
		if !image.Marked {
			a.markRow(i)
		}
	}
}

func (a *Authorizer) markRow(row int) {
	a.ic[row].Marked = true
	a.widgets.list.Rows[row] = strings.Replace(a.widgets.list.Rows[row], unmarked, marked, 1)
}

func (a *Authorizer) unmarkAllRows() {
	for i, image := range a.ic {
		if image.Marked {
			a.unmarkRow(i)
		}
	}
}

func (a *Authorizer) unmarkRow(row int) {
	a.ic[row].Marked = false
	a.widgets.list.Rows[row] = strings.Replace(a.widgets.list.Rows[row], marked, unmarked, 1)
}

func readCredentials() (Credentials, error) {
	fmt.Print("Enter username: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := scanner.Text()
	if username == "" {
		return Credentials{}, nil
	}

	fmt.Print("Password: ")
	password, err := gopass.GetPasswdMasked()

	return Credentials{Username: username, Password: string(password)}, err
}
