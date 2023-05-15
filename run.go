package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/glebarez/sqlite"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

type Articles struct {
	ID      uint `gorm:"primarykey,"`
	Title   string
	Time    time.Time
	Url     string
	Imgs    string
	Desc    string
	Content string
	Html    string
	Ex      string
	Ex2     string
}

func (a Articles) TableName() string {
	return "articles"
}

var (
	db   *gorm.DB
	eArr []string

	prefix = "https://coolshell.cn/page/"

	wg     sync.WaitGroup
	ticker = time.NewTicker(10 * time.Microsecond)
)

type spider struct {
	db    *gorm.DB
	mu    sync.Mutex
	wg    sync.WaitGroup
	count int
	todo  chan interface{}
	out   chan interface{}
}

func init() {
	var err error
	db, err = gorm.Open(sqlite.Open("coolshell.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&Articles{})
}

func InsertArticle(a *Articles) error {
	err := db.Create(a).Error
	if err != nil {
		return err
	}
	return nil
}

func Query(title string) {

}

func GetPages(_url string, page chan string) {
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
		title := s.Find("h2>a").Text()
		time := s.Find("time").AttrOr("datetime", s.Find("time").Text())
		href, _ := s.Find("h2>a").Attr("href")
		page <- title + "," + time + "," + href
	})
}

func GetArticle(art *Articles, aurl string) error {
	req := &fasthttp.Request{}
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")
	req.SetRequestURI(aurl)
	resp := &fasthttp.Response{}
	if err := fasthttp.Do(req, resp); err != nil || resp.StatusCode() != 200 {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		return err
	}
	text := doc.Find("main > article > div").Children().Not("div").Text()
	html, _ := doc.Find("main > article > div.entry-content").Html()
	idx := strings.Index(html, `<div class="wp_rp_wrap`)
	art.Content = text
	art.Html = html[:idx]
	return nil

}

type record struct {
	idx int
	str string
}

func main() {
	filename := "pages.csv"
	// 创建文件
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var i = 1
	var wg sync.WaitGroup
	tasks := make(chan record, 100)
	results := make(chan error, 1000)
	for j := 0; j < 5; j++ {
		go func() {
			for task := range tasks {
				line := strings.Split(task.str, ",")
				var art Articles
				art.Title = line[0]
				art.Url = line[2]
				art.Time, _ = time.Parse(time.RFC3339, line[1])
				err := GetArticle(&art, line[2])
				if err != nil {
					log.Println(task.idx, err)
				}
				if err := InsertArticle(&art); err != nil {
					log.Println(task.idx, err)
				}
				log.Println(task.idx)
				results <- fmt.Errorf("%d:%s", task.idx, err)
			}
		}()
	}
	for scanner.Scan() {
		tasks <- record{idx: i, str: scanner.Text()}
		i++
	}
	close(tasks)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var errs []error
		for res := range results {
			if res != nil {
				errs = append(errs, res)
			}
		}
		for _, err := range errs {
			log.Println(err)
		}
	}()
	wg.Wait()
}
