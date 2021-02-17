package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	// HTMLCurrentDateTime is the string anchor to be replaced by current system time in rendered HTML output.
	HTMLCurrentDateTime = "#LAITOS_3339TIME"

	// HTMLClientAddress it the string anchor to be replaced by HTTP client IP address in rendered HTML output.
	HTMLClientAddress = "#LAITOS_CLIENTADDR"
)

// HandleHTMLDocument renders an HTML page with client IP and current system time injected inside.
type HandleHTMLDocument struct {
	HTMLFilePath string `json:"HTMLFilePath"`

	contentBytes  []byte // contentBytes is the HTML document file's content in bytes
	contentString string // contentString is the HTML document file's content in string
}

func (doc *HandleHTMLDocument) Initialise(lalog.Logger, *toolbox.CommandProcessor, string) error {
	var err error
	if doc.contentBytes, err = os.ReadFile(doc.HTMLFilePath); err != nil {
		return fmt.Errorf("HandleHTMLDocument.Initialise: failed to open HTML file at %s - %v", doc.HTMLFilePath, err)
	}
	doc.contentString = string(doc.contentBytes)
	return nil
}

func (doc *HandleHTMLDocument) Handle(w http.ResponseWriter, r *http.Request) {
	// Inject browser client IP and current time into index document and return.
	w.Header().Set("Content-Type", "text/html")
	NoCache(w)
	page := strings.Replace(doc.contentString, HTMLCurrentDateTime, time.Now().Format(time.RFC3339), -1)
	page = strings.Replace(page, HTMLClientAddress, middleware.GetRealClientIP(r), -1)
	_, _ = w.Write([]byte(page))
}

func (_ *HandleHTMLDocument) GetRateLimitFactor() int {
	/*
		Usually nobody visits the index page (or plain HTML document) this often, but on Elastic Beanstalk the nginx
		proxy in front of the HTTP 80 server visits the index page a lot! If HTTP server fails to serve this page,
		Elastic Beanstalk will consider the instance unhealthy. Therefore, the factor here allows 8x as many requests
		to be processed.
	*/
	return 8
}

func (_ *HandleHTMLDocument) SelfTest() error {
	return nil
}
