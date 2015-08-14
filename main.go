package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

const URL = "http://www2.correios.com.br/sistemas/rastreamento/resultado.cfm"

type result struct {
	selection  *goquery.Selection
	lastStatus string
	code       string
}

func getResult(code string) *result {
	r := new(result)
	r.code = code
	if err := r.get(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	r.getLastStatus()
	return r
}

func (r *result) get() error {
	body := strings.NewReader(url.Values{"objetos": {r.code}}.Encode())
	req, err := http.NewRequest("POST", URL, body)
	if err != nil {
		return err
	}

	req.Header.Add("Referer", "http://www.correios.com.br/para-voce")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := new(http.Client).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return err
	}

	r.selection = doc.Find(".ctrlcontent")
	return nil
}

func (r *result) getLastStatus() string {
	highlight := r.selection.Find(".highlightSRO")
	title := highlight.Find("strong").Text()
	h, _ := highlight.Html()
	info := strings.Split(h, "\n")[3]
	r.lastStatus = title + " - " + strings.Split(info, "<br/>")[2]
	return r.lastStatus
}

func main() {
	flags := &cobra.Command{Use: "correios"}
	checker := &cobra.Command{
		Use:   "check [code]",
		Short: "Check the status of the code",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				println(cmd.UsageString())
				return
			}
			res := getResult(strings.Join(args, ";"))
			println(res.getLastStatus())
		},
	}

	flags.AddCommand(checker)
	flags.Execute()
}
