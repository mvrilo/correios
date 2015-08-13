package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/PuerkitoBio/goquery"
	"github.com/kr/pretty"
)

const URL = "http://www2.correios.com.br/sistemas/rastreamento/resultado.cfm"

var storage = os.Getenv("HOME") + "/.correios"

func get(code string) *goquery.Selection {
	res, err := http.PostForm(URL, url.Values{
		"P_LINGUA": []string{"001"},
		"P_TIPO":   []string{"001"},
		"objetos":  []string{code},
	})
	checkErr(err)
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(res)
	checkErr(err)

	return doc.Find(".ctrlcontent").First()
}

func checkErr(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func getCodes() *bytes.Buffer {
	var f *os.File
	var err error
	if _, err = os.Stat(storage); os.IsNotExist(err) {
		f, err = os.Create(storage)
		checkErr(err)
	}

	if f == nil {
		f, err = os.Open(storage)
		checkErr(err)
	}

	var data []byte
	data, err = ioutil.ReadAll(f)
	checkErr(err)

	return bytes.NewBuffer(data)
}

func check(code string) {
	if code == "" {
	}
}

func main() {
	pretty.Println(get("RR017527799VN"))
}
