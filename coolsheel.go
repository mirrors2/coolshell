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
	"github.com/spf13/cobra"
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
	log.Println(_url)
}

func pages() {
	filename := "pages.csv"
	// 创建文件
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)

	task := make(chan string, 1000)
	page := make(chan string, 1000)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range task {
				GetPages(u, page)
			}
		}()
	}

	for i := 1; i <= 74; i++ {
		task <- fmt.Sprintf(prefix+"%d", i)
	}
	close(task)
	go func() {
		wg.Wait()
		close(page)
	}()

	for line := range page {
		if line == "" {
			continue
		}
		writer.WriteString(line + "\n")
		writer.Flush()
	}

}

func GetArticle(art *Articles, aurl string) error {
	req := &fasthttp.Request{}
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")
	req.SetRequestURI(aurl)
	resp := &fasthttp.Response{}
	if err := fasthttp.Do(req, resp); err != nil || resp.StatusCode() != 200 {
		log.Println(err)
		return err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		return err
	}
	content := doc.Find("div.entry-content")
	content.Find("#wp_rp_first").Remove()
	content.Find("div.post-ratings").Remove()
	content.Find("div.post-ratings-loading").Remove()
	art.Content = strings.TrimSpace(content.Text())
	html, _ := content.Html()
	art.Html = strings.TrimSpace(html)
	return nil
}

type record struct {
	idx int
	str string
}

func dl() {
	filename := "pages.csv"
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
	for j := 0; j < 10; j++ { //
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				line := strings.Split(task.str, ",")
				var art Articles
				length := len(line) - 1
				art.Title = line[length-2]
				art.Url = line[length]
				art.Time, _ = time.Parse(time.RFC3339, line[length-1])
				err1 := GetArticle(&art, line[2])
				if err1 != nil {
					log.Println(task.idx, err)
				}
				err2 := InsertArticle(&art)
				if err != nil {
					log.Println(task.idx, err)
				}
				log.Println(task.idx)
				if err1 != nil || err2 != nil {
					results <- fmt.Errorf("%d:%s|%s", task.idx, err1, err2)
				}

			}
		}()
	}
	for scanner.Scan() {
		tasks <- record{idx: i, str: scanner.Text()}
		i++
	}
	close(tasks)

	go func() {
		defer close(results)
		wg.Wait()
	}()
	var errs []error
	for res := range results {
		if res != nil {
			errs = append(errs, res)
		}
	}
	for _, e := range errs {
		fmt.Println(e)
	}

}

func fixdb() {
	var as []Articles
	if err := db.Where("content = ?", "").Find(&as).Error; err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	tasks := make(chan Articles, 100)
	results := make(chan error, 100)
	for j := 0; j < 5; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				var err error
				err = GetArticle(&task, task.Url)
				if err != nil {
					log.Println(task.ID, err)
				}
				if task.Content != "" {
					if err = db.Model(&Articles{}).Where("id = ?", task.ID).Updates(task).Error; err != nil {
						log.Println(task.ID, err)
					}
				} else {
					err = fmt.Errorf("获取文章失败:%s", err)
				}

				log.Println(task.ID)
				if err != nil {
					results <- fmt.Errorf("%d:%s", task.ID, err)
				}
			}
		}()
	}
	for _, a := range as {
		tasks <- a
	}
	close(tasks)

	go func() {
		defer close(results)
		wg.Wait()
	}()
	var errs []error
	for res := range results {
		if res != nil {
			errs = append(errs, res)
		}
	}
	for _, e := range errs {
		fmt.Println(e)
	}
}

func main() {
	rootCmd := &cobra.Command{Use: "help"}
	pagesCmd := &cobra.Command{
		Use:   "pages",
		Short: "1.下载文章索引 pages.csv",
		Run: func(cmd *cobra.Command, args []string) {
			pages()
		},
	}
	dlCmd := &cobra.Command{
		Use:   "dl",
		Short: "2.下载文章内容 -> coolshell.db",
		Run: func(cmd *cobra.Command, args []string) {
			dl()
		},
	}
	fixCmd := &cobra.Command{
		Use:   "fix",
		Short: "3.更新 coolshell.db 的空数据",
		Run: func(cmd *cobra.Command, args []string) {
			fixdb()
		},
	}

	rootCmd.AddCommand(pagesCmd, dlCmd, fixCmd)
	rootCmd.Execute()
}
