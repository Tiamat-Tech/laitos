package handler

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/browserp"
	"github.com/HouzuoGuo/laitos/browsers"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"net/http"
	"strconv"
)

// Render web page in a server-side javascript-capable browser, and respond with rendered page image.
type HandleBrowserSlimerJS struct {
	ImageEndpoint string             `json:"-"`
	Browsers      browsers.Instances `json:"Browsers"`
}

func (remoteBrowser *HandleBrowserSlimerJS) Initialise(misc.Logger, *common.CommandProcessor) error {
	return remoteBrowser.Browsers.Initialise()
}

func (remoteBrowser *HandleBrowserSlimerJS) RenderPage(title string,
	instanceIndex int, instanceTag string,
	lastErr error, debugOut string,
	viewWidth, viewHeight int,
	userAgent, pageUrl string,
	pointerX, pointerY int,
	typeText string) []byte {
	var errStr string
	if lastErr == nil {
		errStr = ""
	} else {
		errStr = lastErr.Error()
	}
	return []byte(fmt.Sprintf(HandleBrowserPage,
		title,
		instanceIndex, instanceTag,
		errStr, debugOut,
		strconv.Itoa(viewWidth), strconv.Itoa(viewHeight),
		userAgent, pageUrl,
		strconv.Itoa(pointerX), strconv.Itoa(pointerY),
		typeText,
		remoteBrowser.ImageEndpoint, instanceIndex, instanceTag))
}

func (remoteBrowser *HandleBrowserSlimerJS) parseSubmission(r *http.Request) (instanceIndex int, instanceTag string, viewWidth, viewHeight int, userAgent, pageUrl string, pointerX, pointerY int, typeText string) {
	instanceIndex, _ = strconv.Atoi(r.FormValue("instance_index"))
	instanceTag = r.FormValue("instance_tag")
	viewWidth, _ = strconv.Atoi(r.FormValue("view_width"))
	viewHeight, _ = strconv.Atoi(r.FormValue("view_height"))
	userAgent = r.FormValue("user_agent")
	pageUrl = r.FormValue("page_url")
	pointerX, _ = strconv.Atoi(r.FormValue("pointer_x"))
	pointerY, _ = strconv.Atoi(r.FormValue("pointer_y"))
	typeText = r.FormValue("type_text")
	return
}

func (remoteBrowser *HandleBrowserSlimerJS) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	NoCache(w)
	if !WarnIfNoHTTPS(r, w) {
		return
	}
	if r.Method == http.MethodGet {
		// Start a new browser instance
		index, instance, err := remoteBrowser.Browsers.Acquire()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to acquire browser instance: %v", err), http.StatusInternalServerError)
			return
		}
		w.Write(remoteBrowser.RenderPage(
			"Empty Browser",
			index, instance.Tag,
			nil, instance.GetDebugOutput(),
			800, 800, browserp.GoodUserAgent,
			"https://www.google.com",
			0, 0,
			""))
	} else if r.Method == http.MethodPost {
		index, tag, viewWidth, viewHeight, userAgent, pageUrl, pointerX, pointerY, typeText := remoteBrowser.parseSubmission(r)
		instance := remoteBrowser.Browsers.Retrieve(index, tag)
		if instance == nil {
			// Old instance is no longer there, so start a new browser instance
			index, instance, err := remoteBrowser.Browsers.Acquire()
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to acquire browser instance: %v", err), http.StatusInternalServerError)
				return
			}
			w.Write(remoteBrowser.RenderPage(
				"Empty Browser",
				index, instance.Tag,
				nil, instance.GetDebugOutput(),
				800, 800, browserp.GoodUserAgent,
				"https://www.google.com",
				0, 0,
				""))
			return
		}
		var actionErr error
		switch r.FormValue("action") {
		case "Redraw":
			// There is no browser interaction involved, every page refresh automatically renders the latest screen.
		case "Kill All":
			remoteBrowser.Browsers.KillAll()
			actionErr = errors.New(fmt.Sprint("All browser sessions are gone. Please nagivate back to this browser page by re-entering the URL, do not refresh the page."))
		case "Back":
			actionErr = instance.GoBack()
		case "Forward":
			actionErr = instance.GoForward()
		case "Reload":
			actionErr = instance.Reload()
		case "Go To":
			actionErr = instance.GoTo(userAgent, pageUrl, viewWidth, viewHeight)
		case "Left Click":
			actionErr = instance.Pointer(browserp.PointerTypeClick, browserp.PointerButtonLeft, pointerX, pointerY)
		case "Right Click":
			actionErr = instance.Pointer(browserp.PointerTypeClick, browserp.PointerButtonRight, pointerX, pointerY)
		case "Move To":
			actionErr = instance.Pointer(browserp.PointerTypeMove, browserp.PointerButtonLeft, pointerX, pointerY)
		case "Backspace":
			actionErr = instance.SendKey("", browsers.KeyCodeBackspace)
		case "Enter":
			actionErr = instance.SendKey("", browsers.KeyCodeEnter)
		case "Type":
			actionErr = instance.SendKey(typeText, 0)
		}
		// Display action error, or page info error if there is any.
		pageInfo, pageInfoErr := instance.GetPageInfo()
		if actionErr == nil {
			actionErr = pageInfoErr
		}
		w.Write(remoteBrowser.RenderPage(
			pageInfo.Title,
			index, instance.Tag,
			actionErr, instance.GetDebugOutput(),
			viewWidth, viewHeight,
			userAgent, pageInfo.URL,
			pointerX, pointerY,
			typeText))
	}
}

func (_ *HandleBrowserSlimerJS) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleBrowserSlimerJS) SelfTest() error {
	return nil
}

type HandleBrowserSlimerJSImage struct {
	Browsers *browsers.Instances `json:"-"` // Reference to browser instances constructed in HandleBrowser handler
}

func (_ *HandleBrowserSlimerJSImage) Initialise(misc.Logger, *common.CommandProcessor) error {
	return nil
}

func (remoteBrowserImage *HandleBrowserSlimerJSImage) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	/*
		There is no need to call WarnIfNoHTTPS function in this API, because this API is not reachable unless
		HandleBrowser has already warned the user.
	*/
	index, err := strconv.Atoi(r.FormValue("instance_index"))
	if err != nil {
		http.Error(w, "Bad instance_index", http.StatusBadRequest)
		return
	}
	instance := remoteBrowserImage.Browsers.Retrieve(index, r.FormValue("instance_tag"))
	if instance == nil {
		http.Error(w, "That browser session expired", http.StatusBadRequest)
		return
	}
	if err := instance.RenderPage(); err != nil {
		http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pngFile, err := ioutil.ReadFile(instance.GetRenderPageFilePath())
	if err != nil {
		http.Error(w, "File IO error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(pngFile)))
	w.Write(pngFile)
}

func (_ *HandleBrowserSlimerJSImage) GetRateLimitFactor() int {
	return 2
}

func (_ *HandleBrowserSlimerJSImage) SelfTest() error {
	return nil
}