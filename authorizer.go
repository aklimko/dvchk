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
	Checked  bool
	Position int
}

type Credentials struct {
	Username string
	Password string
}

type Authorizer struct {
	apiClient            *ApiClient
	ic                   []*ImageChoice
	list                 *widgets.List
	tips                 *widgets.Paragraph
	storage              *ImageStorage
	unauthorizedToRemove []int
}

func NewAuthorizer(apiClient *ApiClient, storage *ImageStorage) *Authorizer {
	return &Authorizer{apiClient: apiClient, storage: storage}
}

func (a *Authorizer) Authorize() {
	for a.authorizeContinuously() == statusContinue {
	}
}

func (a *Authorizer) authorizeContinuously() Status {
	if len(a.storage.unauthorized) == 0 {
		return statusFinish
	}

	a.unauthorizedToRemove = nil

	if err := ui.Init(); err != nil {
		fmt.Printf("Failed to initialize termui: %v\n", err)
		return statusFinish
	}

	a.createImageChoices(a.storage.unauthorized)
	a.createWidgets()

	a.renderWidgets()
	result := a.handleKeyInput()
	return result
}

func (a *Authorizer) createImageChoices(images []*ImageAuthUrl) {
	var ic []*ImageChoice
	for i, img := range images {
		ic = append(ic, &ImageChoice{image: img, Checked: false, Position: i})
	}
	a.ic = ic
}

func (a *Authorizer) createWidgets() {
	a.createListWidget()
	a.createTipsWidget()
}

func (a *Authorizer) createListWidget() {
	a.list = widgets.NewList()
	a.list.Title = "Unauthorized images"
	a.list.TitleStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	a.list.TextStyle = ui.NewStyle(ui.ColorYellow)
	a.list.SelectedRowStyle = ui.NewStyle(ui.ColorBlue)
	a.list.WrapText = false
	a.fillListContent()

	w, h := ui.TerminalDimensions()
	a.list.SetRect(0, 0, w, h-5)
}

func (a *Authorizer) fillListContent() {
	var rows []string
	for _, i := range a.ic {
		rows = append(rows, unmarked+" "+i.image.FullName)
	}
	a.list.Rows = rows
}

func (a *Authorizer) createTipsWidget() {
	a.tips = widgets.NewParagraph()
	a.tips.Text = tips
	a.tips.WrapText = true
	a.tips.Border = false

	w, h := ui.TerminalDimensions()
	a.tips.SetRect(0, h-5, w, h)
}

func (a *Authorizer) renderWidgets() {
	ui.Render(a.list, a.tips)
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
			a.list.ScrollUp()
		case "<Down>":
			a.list.ScrollDown()
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
			a.list.ScrollTop()
		case "<End>":
			a.list.ScrollBottom()
		}

		a.renderWidgets()
	}
}

func (a *Authorizer) authenticateMarkedImages() Status {
	var mi []*ImageChoice
	for _, img := range a.ic {
		if img.Checked {
			mi = append(mi, img)
		}
	}
	if len(mi) == 0 {
		return statusIgnore
	}
	ui.Close()

	credentials, err := readCredentials()
	if err != nil {
		fmt.Println(err)
		return statusFinish
	}
	a.getMarkedImageTagsAuthenticated(mi, credentials)
	a.removeImagesFromUnauthorized()
	return statusContinue
}

func (a *Authorizer) getMarkedImageTagsAuthenticated(mi []*ImageChoice, credentials Credentials) {
	for _, img := range mi {
		a.getImageTagsAuthenticated(img, credentials)
	}
}

func (a *Authorizer) getImageTagsAuthenticated(ic *ImageChoice, credentials Credentials) {
	it := &ImageTags{}

	withCredentials, err := a.apiClient.GetTokenWithCredentials(ic.image.AuthUrl, credentials)
	if err != nil {
		fmt.Printf("Token request failed for %s, error:%v\n", ic.image.FullName, err)
		return
	}
	token, err := unmarshalToken(withCredentials)
	if err != nil {
		fmt.Printf("Failed to unmarshal token for %s, error:%v\n", ic.image.FullName, err)
		return
	}
	resp, err := a.apiClient.GetTagListAuthenticated(&ic.image.Image, getAuthHeader(token.Token))
	if err != nil {
		fmt.Printf("Response failed for %s, error:%v\n", ic.image.FullName, err)
	}
	if resp.StatusCode == 200 {
		it, err = unmarshalTags(resp)
		if err != nil {
			fmt.Printf("Failed to unmarshal tags for %s, error:%v\n", ic.image.FullName, err)
			return
		}
		a.storage.successful = append(a.storage.successful, &ImageContext{image: &ic.image.Image, imageTags: it})
		a.unauthorizedToRemove = append(a.unauthorizedToRemove, ic.Position)
		fmt.Printf("Tags for %s downloaded successfully\n", ic.image.FullName)
	} else {
		fmt.Printf("Failed authentication for %s\n", ic.image.FullName)
	}
}

func (a *Authorizer) removeImagesFromUnauthorized() {
	for i := len(a.unauthorizedToRemove) - 1; i >= 0; i-- {
		position := a.unauthorizedToRemove[i]
		a.removeImageFromUnauthorized(position)
	}
}

func (a *Authorizer) removeImageFromUnauthorized(i int) {
	a.storage.unauthorized = append(a.storage.unauthorized[:i], a.storage.unauthorized[i+1:]...)
}

func (a *Authorizer) toggleSelectedRowMark() {
	sr := a.list.SelectedRow
	a.ic[sr].Checked = !a.ic[sr].Checked

	var previous, current string
	if a.ic[sr].Checked {
		previous, current = unmarked, marked
	} else {
		previous, current = marked, unmarked
	}
	a.list.Rows[sr] = strings.Replace(a.list.Rows[sr], previous, current, 1)
}

func (a Authorizer) resizeWidgetsOnTerminalResize() {
	w, h := ui.TerminalDimensions()
	a.list.SetRect(0, 0, w, h-5)
	a.tips.SetRect(0, h-5, w, h)
}

func (a *Authorizer) markAllRows() {
	for i, image := range a.ic {
		if !image.Checked {
			a.markRow(i)
		}
	}
}

func (a *Authorizer) markRow(row int) {
	a.ic[row].Checked = true
	a.list.Rows[row] = strings.Replace(a.list.Rows[row], unmarked, marked, 1)
}

func (a *Authorizer) unmarkAllRows() {
	for i, image := range a.ic {
		if image.Checked {
			a.unmarkRow(i)
		}
	}
}

func (a *Authorizer) unmarkRow(row int) {
	a.ic[row].Checked = false
	a.list.Rows[row] = strings.Replace(a.list.Rows[row], marked, unmarked, 1)
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
