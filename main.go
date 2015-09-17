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

const multiURL = "http://www2.correios.com.br/sistemas/rastreamento/multResultado.cfm"

var file *os.File

func fatal(i interface{}) {
	var s string
	if in, ok := i.(string); ok {
		s = in
	}
	if in, ok := i.(error); ok {
		s = in.Error()
	}
	if s != "" {
		fmt.Println(s)
		os.Exit(1)
	}
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

func scan(fn func(*bufio.Scanner)) {
	s := bufio.NewScanner(file)
	for s.Scan() {
		fn(s)
	}

	if err := s.Err(); err != nil {
		fmt.Println(err)
	}
}

func codeExists(code string) (b bool) {
	scan(func(s *bufio.Scanner) {
		b = strings.Contains(s.Text(), code)
	})
	return b
}

func read() (data []string) {
	scan(func(s *bufio.Scanner) {
		data = append(data, s.Text())
	})
	return data
}

func write(code string) (bool, error) {
	ok := codeExists(code)
	if ok {
		return true, nil
	}

	code = strings.Trim(code, "\n")
	if _, err := file.WriteString(fmt.Sprintf("%s\n", code)); err != nil {
		return false, err
	}
	return false, nil
}

func remove(code string) (bool, error) {
	var newData []string
	lines := read()
	for _, line := range lines {
		if code != line {
			newData = append(newData, line)
		}
	}

	if len(newData) == len(lines) {
		return true, nil
	}

	if err := file.Truncate(0); err != nil {
		return false, err
	}

	if _, err := file.WriteString(strings.Join(newData, "\n") + "\n"); err != nil {
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
		for _, line := range read() {
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
	fmt.Println(strings.Join(read(), "\n"))
}

func add(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fatal(c.App.Usage)
	}

	if len(args[0]) != 13 {
		fatal("Tracking code must have 13 characters")
	}

	exists, err := write(args[0])
	if exists {
		fatal("Tracking code already added")
	}
	fatal(err)
}

func rm(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fatal(c.App.Usage)
	}

	noExists, err := remove(args[0])
	if noExists {
		fatal("Tracking code not found")
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
		var f *os.File
		var err error
		defer func() {
			if err != nil {
				fatal(err)
			}
			file = f
		}()
		filestore := c.String("f")
		if _, err = os.Stat(filestore); os.IsNotExist(err) {
			if f, err = os.Create(filestore); err == nil && len(c.Args()) > 0 {
				fmt.Println(" + " + filestore + " created.")
			}
			return nil
		}

		f, err = os.OpenFile(filestore, os.O_APPEND|os.O_RDWR, 0644)
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
	file.Close()
}
