package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/codegangsta/cli"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"
)

const multiURL = "http://www2.correios.com.br/sistemas/rastreamento/multResultado.cfm"

var file *os.File
var config map[string]interface{}

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

func getAll() map[string]string {
	data := make(map[string]string)
	for code, i := range config {
		info, _ := i.(string)
		data[code] = info
	}
	return data
}
func get(code string) (string, string, bool) {
	for c, i := range config {
		info, _ := i.(string)
		if strings.Index(code, c) == 0 || strings.Index(code, info) == 0 {
			return c, info, true
		}
	}
	return "", "", false
}

func set(code, info string) bool {
	if _, _, ok := get(code); ok {
		return false
	}
	config[code] = info
	return true
}

func del(in string) bool {
	if code, _, ok := get(in); ok {
		delete(config, code)
		return true
	}
	return false
}

func dump() {
	b, err := yaml.Marshal(config)
	fatal(err)

	file.Truncate(0)
	_, err = file.Write(b)
	fatal(err)
}

func write(code, info string) {
	if !set(code, info) {
		fatal("Tracking code already added")
	}
	dump()
}

func remove(in string) {
	if !del(in) {
		fatal("Tracking code not found")
	}
	dump()
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
		if _, info, _ := get(o.id); info == "" {
			ret += fmt.Sprintf("[%s] - %s", o.id, o.status, o.date)
		} else {
			ret += fmt.Sprintf("%s [%s] - %s - %s", info, o.id, o.status, o.date)
		}
	}
	return
}

func check(c *cli.Context) {
	args := c.Args()
	var codes string
	if len(args) > 0 {
		for i, c := range args {
			if code, _, ok := get(c); ok {
				if i > 0 {
					codes += ";"
				}
				codes += code
			}
		}
	} else {
		var i int
		for code := range config {
			if i > 0 {
				codes += ";"
			}
			codes += code
			i++
		}
	}
	fmt.Println(fetchOrders(codes))
}

func list(c *cli.Context) {
	for code, info := range getAll() {
		if info == "" {
			fmt.Println("[" + code + "]")
		} else {
			fmt.Println(info + " [" + code + "]")
		}
	}
}

func add(c *cli.Context) {
	args := c.Args()
	if len(args) < 1 {
		cli.CommandHelpTemplate = strings.Replace(cli.CommandHelpTemplate, "[arguments...]", "<code> [info]", -1)
		cli.ShowCommandHelp(c, "add")
		os.Exit(1)
	}
	if len(args[0]) != 13 {
		fatal("Tracking code must have 13 characters")
	}

	if len(args) > 1 {
		write(args[0], args[1])
	} else {
		write(args[0], "")
	}
}

func rm(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		cli.CommandHelpTemplate = strings.Replace(cli.CommandHelpTemplate, "[arguments...]", "<code || info>", -1)
		cli.ShowCommandHelp(c, "remove")
		os.Exit(1)
	}
	remove(args[0])
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
	correios.Version = "1.0.0"
	correios.EnableBashCompletion = true
	correios.Before = func(c *cli.Context) error {
		var f *os.File
		var err error
		defer func() {
			if err != nil {
				fatal(err)
			}
			file = f
			var b []byte
			b, err = ioutil.ReadAll(f)
			fatal(err)
			fatal(yaml.Unmarshal(b, &config))
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
			Value:  home + "/.correios.yml",
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
