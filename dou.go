package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

type DouVacancy struct {
	url          string
	name         string
	experience   string
	categoryId   string
	categoryName string
}

type DouCategory struct {
	url  string
	name string
	id   string
}

type DouWorker struct {
	storage           Storage
	categories        []DouCategory
	experienceFilters map[string]string
	newVacancyChan    chan DouVacancy
}

const (
	checkVacanciesInterval = 10
	feedUrl                = "https://jobs.dou.ua/vacancies/feeds/?category="
	categoriesUrl          = "https://jobs.dou.ua/vacancies/"
)

func CreateDouWorker(storage Storage) *DouWorker {
	return &DouWorker{
		storage:        storage,
		newVacancyChan: make(chan DouVacancy),
	}
}

func (dw *DouWorker) Run() error {
	res, err := scrapCategories()
	if err != nil {
		return err
	}

	dw.categories = res
	dw.experienceFilters = map[string]string{
		"< 1 року":         "0-1",
		"1…3 роки":         "1-3",
		"3…5 років":        "3-5",
		"5+ років":         "5plus",
		"Будь-який досвід": "",
	}
	go scrapVacancies(dw)
	return nil
}

func scrapVacancies(dw *DouWorker) {
	ticker := time.NewTicker(checkVacanciesInterval * time.Minute)
	for {
		for _, category := range dw.categories {
			for _, v := range dw.experienceFilters {
				lastTimeChecked := dw.storage.GetLastTimeCheckedUTC(category, v)
				if err := scrapCategory(dw, category, v, lastTimeChecked); err != nil {
					fmt.Println(err)
				} else {
					time.Sleep(1 * time.Second)
				}
			}
		}
		<-ticker.C
	}
}

func scrapCategory(dw *DouWorker, category DouCategory, exp string, lastTimeChecked time.Time) error {
	c := createCollector()
	c.OnXML("//item", func(e *colly.XMLElement) {
		pubDate, err := time.Parse(time.RFC1123Z, e.ChildText("//pubDate"))
		if err != nil {
			fmt.Println(err)
		} else if res := pubDate.UTC().Sub(lastTimeChecked); res.Minutes() > 0 {
			vac := DouVacancy{
				url:          strings.ReplaceAll(e.ChildText("//link"), "?utm_source=jobsrss", ""),
				name:         e.ChildText("//title"),
				categoryId:   category.id,
				categoryName: category.name,
				experience:   exp,
			}
			fmt.Printf("Detected new vacancy: %+v\n", vac)
			dw.newVacancyChan <- vac
		}

	})
	c.OnRequest(func(r *colly.Request) {
		dw.storage.SetLastTimeCheckedUTC(category, exp)
		fmt.Printf("Visiting Category: %s EXP:%s\n", category.name, exp)
	})

	err := c.Visit(category.url + "&exp=" + exp)
	if err != nil {
		return err
	}

	return nil
}

func scrapCategories() ([]DouCategory, error) {
	result := []DouCategory{}
	c := createCollector()
	c.OnHTML("select[name='category'] option", func(e *colly.HTMLElement) {
		result = append(result, DouCategory{
			id:   e.Attr("value"),
			name: e.Text,
			url:  feedUrl + url.QueryEscape(e.Attr("value")),
		})
	})
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.OnScraped(func(r *colly.Response) {
		for _, cat := range result {
			fmt.Printf("%+v\n", cat.name)
		}
	})
	err := c.Visit(categoriesUrl)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func createCollector() *colly.Collector {
	c := colly.NewCollector()
	c.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36"
	return c
}
