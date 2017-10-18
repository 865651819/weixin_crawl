package main

import (
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"bytes"
	"path"
)

var urlTemplate = "http://weixin.sogou.com/weixin?type=1&s_from=input&query=%s&ie=utf8&_sug_=n&_sug_type_="
var rootDir = flag.String("r","","root directory")

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

	crawlHomepage(homepage, keyword)
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

func getImg(url string, path string) (n int64, err error) {
	fmt.Println(url, path)
	out, err := os.Create(path)
	if err != nil {
		return
	}
	defer out.Close()
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	pix, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	n, err = io.Copy(out, bytes.NewReader(pix))
	return
}

func crawlHomepage(homepageUrl string, keyword string) error {
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

	makedirIfNotExist(*rootDir + "/" + keyword)

	rAccount := regexp.MustCompile("<p\\s*?class=\"profile_account\">微信号:\\s*(.*?)</p>")
	accountMatch := rAccount.FindStringSubmatch(string(body))
	if len(accountMatch) > 1 {
		fmt.Println("account:", accountMatch[1])
	} else {
		fmt.Println("accountMatch error")
	}

	rImg := regexp.MustCompile("\"radius_avatar profile_avatar\">\\s*?<img.*?src=\"(.*?)\"")
	imgMatch := rImg.FindStringSubmatch(string(body))
	if len(imgMatch) > 1 {
		getImg(imgMatch[1], path.Join(*rootDir, keyword, "header.jpeg"))
	} else {
		fmt.Println(imgMatch)
	}

	r := regexp.MustCompile("\"content_url\":\"(.*?)\"(.|\\s)*?\"cover\":\"(.*?)\"")
	matches := r.FindAllStringSubmatch(string(body), -1)
	for _, match := range matches {
		if len(matches) < 3 {
			continue
		}
		filePath := strings.Replace(match[1], "amp;", "", -1)
		var articleUrl string
		if strings.HasPrefix(filePath, "http") {
			articleUrl = filePath
		} else {
			articleUrl = parsedUrl.Scheme+"://"+parsedUrl.Host+filePath
		}
		headerPicUrl := match[3]
		getArticle(articleUrl, headerPicUrl, keyword)
	}

	return nil
}

func getArticle(articleUrl string, picUrl string, keyword string) error {
	fmt.Println()
	fmt.Println("articleUrl: ", articleUrl)
	fmt.Println("picUrl: ", picUrl)
	resp, err := http.Get(articleUrl)
	if err != nil {
		fmt.Printf("getting %v error: %v\n", articleUrl, err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ReadAll err: %v", err)
		return err
	}

	rTitle := regexp.MustCompile("<title>(.*?)</title>")
	titleMatch := rTitle.FindStringSubmatch(string(body))
	var title string
	if len(titleMatch) > 1 {
		title = titleMatch[1]
		fmt.Println("title:", title)
	} else {
		fmt.Println("titleMatch error")
		return nil
	}
	makedirIfNotExist(path.Join(*rootDir + "/" + keyword, title))
	getImg(picUrl, path.Join(*rootDir, keyword, title, "header.jpeg"))
	getImg(articleUrl, path.Join(*rootDir, keyword, title, "artical.html"))

	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func makedirIfNotExist(path string) error {
	isExist, err := pathExists(path)
	if err != nil {
		return err
	}
	if !isExist {
		err := os.MkdirAll(path, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func startCrawler() {
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

func main() {
	flag.Parse()
	var err error
	if *rootDir == "" {
		*rootDir, err = os.Getwd()
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Println(rootDir)
	startCrawler()
}
