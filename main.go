package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

const (
	multiURL    = "http://www2.correios.com.br/sistemas/rastreamento/multResultado.cfm"
	detailedURL = "http://www2.correios.com.br/sistemas/rastreamento/resultado.cfm"
)

var db = &database{
	dir:   os.Getenv("HOME") + "/.correios",
	flags: os.O_CREATE | os.O_RDWR,
}

type result struct {
	codes  string
	orders []order
}

type order struct {
	id     string
	status string
	date   string
}

type database struct {
	*os.File
	flags int
	dir   string
}

func (d *database) open() (f *os.File, err error) {
	if f, err = os.OpenFile(d.dir, d.flags, 0644); err != nil {
		return
	}
	return
}

func (d *database) scan(fn func(*bufio.Scanner)) *os.File {
	f, err := d.open()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	s := bufio.NewScanner(f)
	for s.Scan() {
		fn(s)
	}

	if err = s.Err(); err != nil {
		fmt.Println(err)
	}

	return f
}

func (d *database) codeExists(code string) (b bool, f *os.File) {
	f = d.scan(func(s *bufio.Scanner) {
		b = strings.Contains(s.Text(), code)
	})
	return b, f
}

func (d *database) read() (data []string) {
	d.scan(func(s *bufio.Scanner) {
		data = append(data, s.Text())
	})
	return data
}

func (d *database) write(code string) (bool, error) {
	d.flags = d.flags | os.O_APPEND
	ok, f := d.codeExists(code)
	if f != nil {
		defer f.Close()
	}
	if ok {
		return true, nil
	}

	code = strings.Trim(code, "\n")
	if _, err := f.WriteString(fmt.Sprintf("%s\n", code)); err != nil {
		return false, err
	}
	return false, nil
}

func (d *database) remove(code string) (bool, error) {
	d.flags = d.flags | os.O_APPEND
	f, err := d.open()
	if f != nil {
		defer f.Close()
	}
	if err != nil {
		return false, err
	}

	var newData []string
	for _, line := range d.read() {
		if code != line {
			newData = append(newData, line)
		}
	}

	if err = f.Truncate(0); err != nil {
		return false, err
	}

	if _, err = f.WriteString(strings.Join(newData, "\n") + "\n"); err != nil {
		return false, err
	}
	return false, nil
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

func check(cmd *cobra.Command, args []string) {
	var codes string
	if len(args) > 0 {
		codes = strings.Join(args, ";")
	} else {
		for i, line := range db.read() {
			if i > 0 {
				codes += ";"
			}
			codes += strings.Split(line, " ")[0]
		}
	}
	res := fetchOrders(codes)
	fmt.Println(res)
}

func list(cmd *cobra.Command, args []string) {
	fmt.Println(strings.Join(db.read(), "\n"))
}

func add(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Usage()
		return
	}

	exists, err := db.write(args[0])
	if exists {
		fmt.Println("code already added")
		os.Exit(1)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func rm(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Usage()
		return
	}

	noExists, err := db.remove(args[0])
	if noExists {
		fmt.Println("code not found")
		os.Exit(1)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	correios := &cobra.Command{
		Use:  "correios",
		Long: "Simple command line tool to track your orders from Correios",
	}
	checkCmd := &cobra.Command{
		Use:   "check [code] [code]",
		Short: "Check the status of one or more orders or the ones that you previously added",
		Run:   check,
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List codes",
		Run:   list,
	}
	addCmd := &cobra.Command{
		Use:   "add <code>",
		Short: "Store a order code to check later without specifying it",
		Long: `After adding a order, simply just check the orders without passing the code to check command
e.g.
$ correios check`,
		Run: add,
	}
	rmCmd := &cobra.Command{
		Use:   "rm <code>",
		Short: "Remove an order code from the storage file",
		Run:   rm,
	}

	correios.AddCommand(checkCmd)
	correios.AddCommand(listCmd)
	correios.AddCommand(addCmd)
	correios.AddCommand(rmCmd)
	correios.Execute()
	db.Close()
}
