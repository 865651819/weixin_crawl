package main

import (
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var urlTemplate = "http://weixin.sogou.com/weixin?type=1&s_from=input&query=%s&ie=utf8&_sug_=n&_sug_type_="

func searchKeyword(searchUrl, keyword string) error {
	fmt.Println("key:", keyword)
	resp, err := http.Get(searchUrl)
	if err != nil {
		fmt.Printf("getting %v error: %v\n", searchUrl, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("getting %v: %v\n", searchUrl, resp.Status)
		return err
	}
	keywordResult, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Errorf("parsing %s as HTML: %v", searchUrl, err)
		return err
	}
	var homepage string
	visitNode := func(n *html.Node) {
		if homepage != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			text := ""
			var f func(*html.Node)
			f = func(n *html.Node) {
				if n.Type == html.TextNode {
					text += n.Data
				}
				if n.FirstChild != nil {
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						f(c)
					}
				}
			}
			f(n)

			if text != keyword {
				return
			}
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				link, err := resp.Request.URL.Parse(a.Val)
				if err != nil {
					continue // ignore bad URLs
				}
				homepage = link.String()
			}
		}

	}
	forEachNode(keywordResult, visitNode, nil)

	crawlHomepage(homepage)
	return nil
}

func forEachNode(n *html.Node, pre, post func(n *html.Node)) {
	if pre != nil {
		pre(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		forEachNode(c, pre, post)
	}
	if post != nil {
		post(n)
	}
}

func crawlHomepage(homepageUrl string) error {
	fmt.Printf("homepage: %v\n", homepageUrl)
	if homepageUrl == "" {
		return errors.New("homepage url empty")
	}

	resp, err := http.Get(homepageUrl)
	if err != nil {
		fmt.Printf("getting %v error: %v\n", homepageUrl, err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf("ReadAll err: %v", err)
		return err
	}

	parsedUrl, err := url.Parse(homepageUrl)
	if err != nil {
		fmt.Printf("url.Parse err: %v", err)
		return err
	}

	var articleUrls []string
	r := regexp.MustCompile("\"content_url\":\"(.*?)\"")
	matches := r.FindAllStringSubmatch(string(body), -1)
	for _, match := range matches {
		path := strings.Replace(match[1], "amp;", "", -1)
		articleUrls = append(articleUrls, parsedUrl.Scheme+"://"+parsedUrl.Host+path)
	}

	for _, articleUrl := range articleUrls {
		getArticle(articleUrl)
	}

	return nil
}

func getArticle(articleUrl string) {
	fmt.Println(articleUrl)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <key_word>\n", os.Args[0])
		return
	}
	keyWord := os.Args[1]
	keyWordEscape := url.QueryEscape(os.Args[1])
	searchUrl := fmt.Sprintf(urlTemplate, keyWordEscape)
	fmt.Printf("search url: %v\n", searchUrl)
	searchKeyword(searchUrl, keyWord)
}
