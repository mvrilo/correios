package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

const (
	multiURL    = "http://www2.correios.com.br/sistemas/rastreamento/multResultado.cfm"
	detailedURL = "http://www2.correios.com.br/sistemas/rastreamento/resultado.cfm"
)

type result struct {
	codes  string
	orders []order
}

type order struct {
	id     string
	status string
	date   string
}

func fetchOrders(codes string) *result {
	r := new(result)
	r.codes = codes
	r.orders = r.getOrders()
	return r
}

func (r *result) get(u string) (*goquery.Selection, error) {
	body := strings.NewReader(url.Values{"objetos": {r.codes}}.Encode())
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Referer", "http://www.correios.com.br/para-voce")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := new(http.Client).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return nil, err
	}

	return doc.Find(".ctrlcontent").First(), nil
}

func (r *result) getOrders() (o []order) {
	content, err := r.get(multiURL)
	if err != nil {
		panic(err)
	}

	content.Find("tbody tr").Each(func(i int, tr *goquery.Selection) {
		td := tr.Find("td")
		o = append(o, order{
			id:     td.Eq(0).Text(),
			status: td.Eq(1).Text(),
			date:   td.Eq(2).Text(),
		})
	})
	return
}

func (r *result) String() (ret string) {
	if len(r.orders) == 0 {
		return "No orders found"
	}
	for i, o := range r.orders {
		if i > 0 {
			ret += "\n"
		}
		ret += fmt.Sprintf("[%s] %s - %s", o.id, o.status, o.date)
	}
	return
}

func main() {
	correios := &cobra.Command{
		Use:  "correios",
		Long: "Simple command line tool to track your orders from Correios",
	}
	checker := &cobra.Command{
		Use:   "check [codes]",
		Short: "Check the status of one or more orders",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println(cmd.Usage())
				return
			}
			res := fetchOrders(strings.Join(args, ";"))
			fmt.Println(res)
		},
	}

	correios.AddCommand(checker)
	correios.Execute()
}
