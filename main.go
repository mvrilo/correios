package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/codegangsta/cli"
	"github.com/mitchellh/go-homedir"
)

func fatal(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func almostFatal(s string) {
	if s != "" {
		fmt.Println(s)
		os.Exit(1)
	}
}

const multiURL = "http://www2.correios.com.br/sistemas/rastreamento/multResultado.cfm"

var db = &database{
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
	flags   int
	storage string
}

func (d *database) scan(fn func(*bufio.Scanner)) *os.File {
	f, err := os.OpenFile(d.storage, d.flags, 0644)
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
	f, err := os.OpenFile(d.storage, d.flags, 0644)
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

func check(c *cli.Context) {
	args := c.Args()
	var codes string
	if len(args) > 0 {
		codes = strings.Join(args, ";")
	} else {
		var i int
		for _, line := range db.read() {
			if len(line) != 13 {
				continue
			}

			if i > 0 {
				codes += ";"
			}
			codes += strings.Split(line, " ")[0]
			i++
		}
	}
	fmt.Println(fetchOrders(codes))
}

func list(c *cli.Context) {
	fmt.Println(strings.Join(db.read(), "\n"))
}

func add(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		almostFatal(c.App.Usage)
	}

	if len(args[0]) != 13 {
		almostFatal("Tracking code must have 13 characters")
	}

	exists, err := db.write(args[0])
	if exists {
		almostFatal("Tracking code already added")
	}
	fatal(err)
}

func rm(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		almostFatal(c.App.Usage)
	}

	noExists, err := db.remove(args[0])
	if noExists {
		almostFatal("Tracking code not found")
	}
	fatal(err)
}

func main() {
	home, err := homedir.Dir()
	if err != nil {
		fatal(err)
	}
	correios := cli.NewApp()
	correios.Name = "correios"
	correios.Author = "Murilo Santana"
	correios.Email = "mvrilo@gmail.com"
	correios.Usage = "Simple command line tool to track your orders from Correios"
	correios.Version = "0.1.0"
	correios.EnableBashCompletion = true
	correios.Before = func(c *cli.Context) error {
		db.storage = c.String("f")
		return nil
	}
	correios.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "filestorage, f",
			Value:  home + "/.correios",
			Usage:  "File used as storage",
			EnvVar: "CORREIOS_FILESTORAGE",
		},
	}
	correios.Commands = []cli.Command{
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "Check the status of one or more orders or the ones that you previously added",
			Action:  check,
		}, {
			Name:    "list",
			Aliases: []string{"l", "ls"},
			Usage:   "List the codes stored",
			Action:  list,
		}, {
			Name:    "add",
			Aliases: []string{"a"},
			Usage:   "Store an order code to check later without specifying it",
			Action:  add,
		}, {
			Name:    "remove",
			Aliases: []string{"rm", "r"},
			Usage:   "Remove an order code from the storage file",
			Action:  rm,
		},
	}
	correios.Run(os.Args)
	db.Close()
}
