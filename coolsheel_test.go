package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/valyala/fasthttp"
)

func getpage(_url string) {
	req := &fasthttp.Request{}
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")
	req.SetRequestURI(_url)
	resp := &fasthttp.Response{}
	if err := fasthttp.Do(req, resp); err != nil || resp.StatusCode() != 200 {
		log.Println(_url, err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		log.Println(_url, err)
	}

	doc.Find("main>article").Each(func(i int, s *goquery.Selection) {
		fmt.Println(s.Find("h2>a").Text())
		fmt.Println(s.Find("time").AttrOr("datetime", s.Find("time").Text()))
		val, _ := s.Find("h2>a").Attr("href")
		fmt.Println(val)
		fmt.Println()
	})
}

func Test_getpage(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "page", url: "https://coolshell.cn/page/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getpage(tt.url)
		})
	}
}

func getactical(_url string) {
	req := &fasthttp.Request{}
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")
	req.SetRequestURI(_url)
	resp := &fasthttp.Response{}
	if err := fasthttp.Do(req, resp); err != nil || resp.StatusCode() != 200 {
		log.Println(_url, err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		log.Println(_url, err)
	}
	content := doc.Find("div.entry-content")
	content.Find("#wp_rp_first").Remove()
	content.Find("div.post-ratings").Remove()
	content.Find("div.post-ratings-loading").Remove()
	fmt.Println(strings.TrimSpace(content.Text()))
	html, _ := content.Html()
	fmt.Println(strings.TrimSpace(html))

}

func Test_getactical(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "actical", url: "https://coolshell.cn/articles/2913.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getactical(tt.url)
		})
	}
}
