package core

import (
	"fmt"
	"github.com/go-resty/resty"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	jsoniter "github.com/json-iterator/go"
)

// Overview Dehashed code
type Overview struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	CheckSum      string `json:"checksum"`
	ContentFile   string `json:"content_file"`
	Status        string `json:"status_code"`
	ContentLength string `json:"length"`
	Redirect      string `json:"redirect"`
}

// PrintOverview
func PrintOverview(options Options, overview Overview) string {
	if options.JsonOutput {
		if data, err := jsoniter.MarshalToString(overview); err == nil {
			return data
		}
	}
	// more detail when no output file
	if options.NoOutput {
		return fmt.Sprintf("%v ;; %v ;; %v ;; %v ;; %v ;; %v", overview.URL, overview.Title, overview.CheckSum, overview.Status, overview.ContentLength, overview.Redirect)
	}

	if options.SaveRedirectURL {
		return fmt.Sprintf("%v ;; %v ;; %v ;; %v ;; %v", overview.URL, overview.Title, overview.CheckSum, overview.ContentFile, overview.Redirect)
	}
	return fmt.Sprintf("%v ;; %v ;; %v ;; %v", overview.URL, overview.Title, overview.CheckSum, overview.ContentFile)
}

// CalcCheckSum calculate checksum
func CalcCheckSum(options Options, url string, client *resty.Client) string {
	var result string
	title := "No-Title"
	hash := "No-CheckSum"
	contentFile := "No-Content"
	overview := Overview{
		URL:         url,
		Title:       title,
		CheckSum:    "",
		ContentFile: "",
		Redirect:    "No-Redirect",
	}
	res, err := JustSend(options, url, client)
	if err != nil {
		DebugF("Headers: \n%v", res.BeautifyHeader)
		DebugF("Body: \n%v", res.Beautify)
		ErrorF("Error sending: %v", url)
		//return fmt.Sprintf("%v ;; %v ;; %v ;; %v", url, title, hash, contentFile)
		return ""
	}

	overview.Status = res.Status
	overview.ContentLength = fmt.Sprintf("%v", res.Length)
	if res.Location != "" {
		overview.Redirect = res.Location
	}

	// store response
	content := res.BeautifyHeader
	if options.SaveReponse {
		content += "\n\n" + res.Body
	}
	if !options.NoOutput && strings.TrimSpace(content) != "" {
		contentFile = fmt.Sprintf("%v.txt", strings.Replace(url, "://", "___", -1))
		contentFile = strings.Replace(contentFile, "?", "_", -1)
		contentFile = strings.Replace(contentFile, "/", "_", -1)
		content = fmt.Sprintf("> GET %v\n%v", url, content)
		contentFile = path.Join(options.ContentOutput, contentFile)
		DebugF("contentFile: %v", contentFile)
		_, err = WriteToFile(contentFile, content)
		if err != nil {
			ErrorF("WriteToFile: ", err)
			contentFile = "No-Content"
		}
	}

	// in case response is raw JSON
	result = GenHash(res.Body)
	if !strings.Contains(res.ContentType, "html") && !strings.Contains(res.ContentType, "xml") {
		if !strings.Contains(res.Body, "<html>") && !strings.Contains(res.Body, "<a>") {
			hash = GenHash(fmt.Sprintf("%v-%v", title, result))
			//return fmt.Sprintf("%v ;; %v ;; %v ;; %v", url, title, GenHash(res.Body), contentFile)
			overview.CheckSum = GenHash(res.Body)
			overview.Title = title
			overview.ContentFile = contentFile
			PrintOverview(options, overview)
		}
	}

	// parse body
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res.Body))
	if err != nil {
		ErrorF("Error Parsing Body: %v", url)
		return fmt.Sprintf("%v ;; %v ;; %v ;; %v", url, title, GenHash(res.Body), contentFile)
	}
	title = GetTitle(doc)
	hash = GenHash(fmt.Sprintf("%v-%v", title, result))

	// wordlist builder
	BuildWordlists(options, url, doc)

	// calculate Hash based on level
	switch options.Level {
	case 0:
		result = ParseDocLevel0(options, doc)
	case 1:
		result = ParseDocLevel1(options, doc)
	case 2:
		result = ParseDocLevel2(options, doc)
	}
	if result != "" {
		hash = GenHash(fmt.Sprintf("%v-%v", title, result))
	}

	DebugF("Checksum-lv-%v: %v \n", options.Level, result)
	overview.CheckSum = hash
	overview.Title = title
	overview.ContentFile = contentFile
	return PrintOverview(options, overview)
	//return fmt.Sprintf("%v ;; %v ;; %v ;; %v", url, title, hash, contentFile)
}

// GetTitle get title of response
func GetTitle(doc *goquery.Document) string {
	var title string
	doc.Find("title").Each(func(i int, s *goquery.Selection) {
		title = strings.TrimSpace(s.Text())
	})
	if title == "" {
		title = "Blank Title"
	}

	// clean title if if have new line here
	if strings.Contains(title, "\n") {
		title = regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(strings.TrimSpace(title), "\n")
	}

	return title
}

// ParseDocLevel0 calculate Hash based on src in scripts
func ParseDocLevel0(options Options, doc *goquery.Document) string {
	var result []string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src != "" {
			result = append(result, src)
		}
	})

	if options.SortTag {
		sort.Strings(result)
	}
	return strings.Join(result, "-")
}

// ParseDocLevel1 calculate Hash based on src in scripts
func ParseDocLevel1(options Options, doc *goquery.Document) string {
	var result []string
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		result = append(result, tag)
		if tag == "script" {
			src, _ := s.Attr("src")
			if src != "" {
				result = append(result, src)
			}
		}
	})

	if options.SortTag {
		sort.Strings(result)
	}
	return strings.Join(result, "-")
}

// ParseDocLevel2 calculate Hash based on src in scripts
func ParseDocLevel2(options Options, doc *goquery.Document) string {
	var result []string
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		result = append(result, tag)
		if tag == "script" || tag == "img" {
			src, _ := s.Attr("src")
			if src != "" {
				result = append(result, src)
			}
		}

		if tag == "a" {
			src, _ := s.Attr("href")
			if src != "" {
				result = append(result, src)
			}
		}
	})
	if options.SortTag {
		sort.Strings(result)
	}
	return strings.Join(result, "-")
}
