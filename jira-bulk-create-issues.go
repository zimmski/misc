package main

import (
	"bufio"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/djimenez/iconv-go"
	"github.com/jessevdk/go-flags"
)

const (
	returnOk = iota
	returnHelp
)

var opts struct {
	InputFile          string            `long:"input-file" description:"The CSV input file" required:"true"`
	InputFileEncoding  string            `long:"input-file-encoding" description:"Encoding for the CSV input file" default:"utf-8"`
	CSVColumnSeparator string            `long:"csv-column-separator" description:"CSV column separator" default:","`
	As                 map[string]string `long:"as" description:"Rename columns from key to value for the API call. e.g. --as Name:summary"`
	Convert            map[string]string `long:"convert" description:"Convert column key to type value. e.g. --convert count:integer"`
	URL                string            `long:"url" description:"Full URL to the JIRA server. e.g. https://jira.url.com" required:"true"`
	Verbose            bool              `long:"verbose" description:"Verbose output"`

	User     string `long:"jira-user" description:"Jira User" required:"true"`
	Password string `long:"jira-password" description:"Jira Password" required:"true"`

	Assignee    string `long:"assignee" description:"Defines the default assignee"`
	Component   string `long:"component" description:"Defines the default component"`
	Description string `long:"description" description:"Defines the default description"`
	IssueType   string `long:"issue-type" description:"Defines the default issue type"`
	ProjectKey  string `long:"project-key" description:"Defines the default project key"`
}

func arguments() {
	p := flags.NewNamedParser("jira-bulk-create-issues", flags.HelpFlag)
	p.ShortDescription = "Create issues in jira in bulk out of a CSV file"

	_, err := p.AddGroup("Arguments", "", &opts)
	if err != nil {
		panic(err)
	}

	if _, err := p.ParseArgs(os.Args); err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			panic(err)
		} else {
			p.WriteHelp(os.Stdout)

			os.Exit(returnHelp)
		}
	}
}

func main() {
	var err error
	ask := bufio.NewReader(os.Stdin)

	arguments()

	var file io.Reader

	file, err = os.Open(opts.InputFile)
	if err != nil {
		panic(err)
	}

	if opts.InputFileEncoding != "utf-8" {
		file, err = iconv.NewReader(file, opts.InputFileEncoding, "utf-8")
		if err != nil {
			panic(err)
		}
	}

	c := csv.NewReader(file)

	c.Comma = rune(opts.CSVColumnSeparator[0])

	headers, err := c.Read()
	if err != nil {
		panic(err)
	}

	if opts.Verbose {
		fmt.Printf("Found headers\n\t%#v\n", headers)

		fmt.Println("Press ENTER")
		ask.ReadLine()
	}

	if len(opts.As) != 0 {
		for i, k := range headers {
			if v, ok := opts.As[k]; ok {
				headers[i] = v
			}
		}

		if opts.Verbose {
			fmt.Printf("Renamed headers to\n\t%#v\n", headers)

			fmt.Println("Press ENTER")
			ask.ReadLine()
		}
	}

	rows := make([]map[string]string, 0)

	if opts.Verbose {
		fmt.Println("Found rows")
	}

	for {
		r, err := c.Read()

		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}

		row := make(map[string]string)

		for i, v := range r {
			row[headers[i]] = strings.Trim(v, " \n\r")
		}

		if _, ok := row["assignee"]; !ok || row["assignee"] == "" {
			row["assignee"] = opts.Assignee
		}
		if _, ok := row["components"]; !ok || row["components"] == "" {
			row["components"] = opts.Component
		}
		if _, ok := row["description"]; !ok || row["description"] == "" {
			row["description"] = opts.Description
		}
		if _, ok := row["issuetype"]; !ok || row["issuetype"] == "" {
			row["issuetype"] = opts.IssueType
		}
		if _, ok := row["project"]; !ok || row["project"] == "" {
			row["project"] = opts.ProjectKey
		}

		if opts.Verbose {
			fmt.Printf("\t%#v\n", row)
		}

		rows = append(rows, row)
	}

	if opts.Verbose {
		fmt.Printf("%d rows found\n", len(rows))

		fmt.Println("Press ENTER")
		ask.ReadLine()
	}

	if opts.Verbose {
		fmt.Println("Start bulk create")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
	}

	if opts.Verbose {
		fmt.Println("Try to login via HTTP")
	}

	data := url.Values{}

	data.Add("os_username", opts.User)
	data.Add("os_password", opts.Password)
	data.Add("login", "Log In")

	resp, err := client.Post(opts.URL+"/login.jsp", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 302 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Response\n\t%#v\nBody\n\t%s\n", resp, body)

		panic("login failed")
	}

	for i, row := range rows {
		if opts.Verbose {
			fmt.Printf("Row %d\n", i)
		}

		data := make(map[string]interface{})

		for k, v := range row {
			if typ, ok := opts.Convert[k]; ok {
				switch typ {
				case "MultiSelect":
					data[k] = splitSelection(v, "value")
				case "MultiUserPicker":
					data[k] = splitSelection(v, "name")
				case "NumberField":
					data[k], err = strconv.Atoi(v)
					if err != nil {
						panic(err)
					}
				case "SelectList":
					data[k] = map[string]string{"value": v}
				case "UserPicker":
					data[k] = map[string]string{"name": v}
				default:
					panic(fmt.Sprintf("Type %s not defined", typ))
				}
			} else {
				switch k {
				case "assignee", "issuetype":
					data[k] = map[string]string{"name": v}
				case "components":
					data[k] = []map[string]string{
						map[string]string{"name": v},
					}
				case "project":
					data[k] = map[string]string{"key": v}
				default:
					data[k] = v
				}
			}
		}

		reqData := map[string]interface{}{"fields": data}

		out, err := json.Marshal(reqData)
		if err != nil {
			panic(err)
		}

		if opts.Verbose {
			fmt.Printf("\tWill send\n\t\t%s\n", out)

			fmt.Println("Press ENTER")
			ask.ReadLine()
		}

		resp, err := client.Post(opts.URL+"/rest/api/2/issue/", "application/json", strings.NewReader(string(out)))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		if opts.Verbose {
			fmt.Printf("\tResponse\n\t\t%#v\n", resp)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		if opts.Verbose {
			fmt.Printf("\tBody\n\t\t%s\n", body)

			fmt.Println("Press ENTER")
			ask.ReadLine()
		}
	}
}

func splitSelection(value string, key string) []map[string]string {
	sel := make([]map[string]string, 0)

	for _, s := range strings.Split(value, ",") {
		sel = append(sel, map[string]string{key: strings.Trim(s, " \n\r")})
	}

	return sel
}
