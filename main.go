package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Product struct{}

func (p *Product) Resolve(_url string) (string, error) {
	req, _ := http.NewRequest("POST", "https://pricehistory.app/api/search", strings.NewReader("{\"url\":\""+_url+"\"}"))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	var result map[string]interface{}

	err = json.NewDecoder(res.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	if slug, ok := result["code"].(string); ok {
		return slug, nil
	}

	return "", fmt.Errorf("slug for the given url not found")
}

type Data struct {
	Slug         string              `json:"slug"`
	Title        string              `json:"title"`
	Image        string              `json:"image"`
	Price        string              `json:"price"`
	PriceHistory []map[string]string `json:"price_history"`
	MRP          string              `json:"mrp"`
	Discount     string              `json:"discount"`
	ProductInfo  map[string]string   `json:"product_info"`
}

func (p *Product) Get(slug string) (*Data, error) {
	resp, err := http.Get("https://pricehistory.app/p/" + slug)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("product not found")
	}

	var result Data
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	result.Slug = slug
	doc.Find("div .ph-title").Each(func(i int, s *goquery.Selection) {
		result.Title = strings.TrimSpace(s.Text())
	})
	result.Image, _ = doc.Find("div .card-img").First().Find("img").Attr("src")

	doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
		data := map[string]string{}
		data["type"] = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s.Find("th").Text())), " ", "_")
		data["value"] = strings.TrimSpace(s.Find("td").Text())
		data["date"] = strings.TrimSpace(s.Find("td").Next().Text())

		data["value"] = strings.TrimSpace(strings.ReplaceAll(data["value"], data["date"], ""))
		result.PriceHistory = append(result.PriceHistory, data)
	})

	result.ProductInfo = map[string]string{}

	doc.Find("table").Last().Find("tr").Each(func(i int, s *goquery.Selection) {
		result.ProductInfo[strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s.Find("th").Text()), " ", "_"))] = strings.TrimSpace(s.Find("td").Text())
	})

	doc.Find("table").Each(func(i int, s *goquery.Selection) {
		if !strings.Contains(s.Text(), "Price:") {
			return
		}
		s.Find("td").Each(func(i int, s *goquery.Selection) {
			if i == 0 {
				result.Price = strings.TrimSpace(s.Text())
			} else if i == 1 {
				result.MRP = strings.TrimSpace(s.Text())
			} else if i == 2 {
				result.Discount = strings.TrimSpace(s.Text())
			}
		})
	})

	return &result, nil
}

func (p *Product) GetByURL(_url string) (*Data, error) {
	slug, err := p.Resolve(_url)
	if err != nil {
		return nil, err
	}

	return p.Get(slug)
}

func (p *Product) GetHtml(slug string) ([]byte, error) {
	resp, err := http.Get("https://pricehistory.app/p/" + slug)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("product not found")
	}

	var response string
	var rcvd bool
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	doc.Find("div .row").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "Price:") && !rcvd {
			response, _ = s.Html()

			rcvd = true
		}
	})

	return []byte(response), nil
}

func main() {
	http.ListenAndServe(":8080", nil)
}

func init() {
	p := &Product{}
	fmt.Println("Starting server...")

	http.HandleFunc("/resolve", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("url")
		sl, err := p.Resolve(u)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		html, err := p.GetHtml(sl)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		template_ := template.Must(template.New("webpage.html").ParseFiles("webpage.html"))
		// {{.Content}}
		template_.Execute(w, struct {
			Content template.HTML
		}{
			Content: template.HTML(html),
		})
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		data, err := p.GetByURL(slug)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(data)
	})

	http.HandleFunc("/api/product", func(w http.ResponseWriter, r *http.Request) {
		urx := r.URL.Query().Get("url")
		if urx == "" {
			w.Write([]byte("Please provide url"))
			return
		}

		data, err := p.GetByURL(urx)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(data)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
}
