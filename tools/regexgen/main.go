/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/mvdan/xurls"
)

var (
	tldsTmpl = template.Must(template.New("tlds").Parse(`// Generated by regexgen

package xurls

// TLDs is a sorted list of all public top-level domains
var TLDs = []string{
{{range $i, $value := .}}` + "\t`" + `{{$value}}` + "`" + `,
{{end}}}
`))
	regexTmpl = template.Must(template.New("regex").Parse(`// Generated by regexgen

package xurls

const (
	webURL = ` + "`" + `{{.WebURL}}` + "`" + `
	email  = ` + "`" + `{{.Email}}` + "`" + `
	all    = ` + "`" + `{{.All}}` + "`" + `
)
`))
)

func addFromIana(tlds map[string]struct{}) error {
	resp, err := http.Get("https://data.iana.org/TLD/tlds-alpha-by-domain.txt")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	re := regexp.MustCompile(`^[^#]+$`)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindString(line)
		if match == "" {
			continue
		}
		tld := strings.ToLower(match)
		tlds[tld] = struct{}{}
	}
	return nil
}

func addFromPublicSuffix(tlds map[string]struct{}) error {
	resp, err := http.Get("https://publicsuffix.org/list/effective_tld_names.dat")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	re := regexp.MustCompile(`(^([^/.]+)$|^// (xn--[^\s]+)[\s$])`)
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches == nil || len(matches) < 4 {
			continue
		}
		tld := matches[2]
		if tld == "" {
			tld = matches[3]
		}
		tlds[tld] = struct{}{}
	}
	return nil
}

func tldList() ([]string, error) {
	tlds := make(map[string]struct{})
	if err := addFromIana(tlds); err != nil {
		return nil, err
	}
	if err := addFromPublicSuffix(tlds); err != nil {
		return nil, err
	}
	list := make([]string, 0, len(tlds))
	for tld := range tlds {
		list = append(list, tld)
	}
	sort.Strings(list)
	return list, nil
}

func writeTlds(tlds []string) error {
	f, err := os.Create("tlds.go")
	if err != nil {
		return err
	}
	return tldsTmpl.Execute(f, tlds)
}

func reverseJoin(a []string, sep string) string {
	if len(a) == 0 {
		return ""
	}
	if len(a) == 1 {
		return a[0]
	}
	n := len(sep) * (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}

	b := make([]byte, n)
	bp := copy(b, a[len(a)-1])
	for i := len(a) - 2; i >= 0; i-- {
		s := a[i]
		bp += copy(b[bp:], sep)
		bp += copy(b[bp:], s)
	}
	return string(b)
}

const (
	letters  = "a-zA-Z\u00A0-\uD7FF\uF900-\uFDCF\uFDF0-\uFFEF"
	iriChar  = letters + `0-9`
	ipv4Addr = `((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[1-9][0-9]|[1-9])\.(25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[1-9][0-9]|[1-9]|0)\.(25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[1-9][0-9]|[1-9]|0)\.(25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[1-9][0-9]|[0-9]))`
	ipv6Addr = `(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`
	ipAddr   = `(` + ipv4Addr + `|` + ipv6Addr + `)`
	iri      = `[` + iriChar + `]([` + iriChar + `\-]{0,61}[` + iriChar + `]){0,1}`
)

func writeRegex(tlds []string) error {
	var allTlds []string
	for _, tld := range tlds {
		allTlds = append(allTlds, tld)
	}
	for _, tld := range xurls.PseudoTLDs {
		allTlds = append(allTlds, tld)
	}
	sort.Strings(allTlds)
	var (
		gtld       = `(?i)(` + reverseJoin(allTlds, `|`) + `)(?-i)`
		hostName   = `(` + iri + `\.)+` + gtld
		domainName = `(` + hostName + `|` + ipAddr + `|localhost)`
		webURL     = `((https?:\/\/(([a-zA-Z0-9\$\-\_\.\+\!\*\'\(\)\,\;\?\&\=]|(\%[a-fA-F0-9]{2})){1,64}(\:([a-zA-Z0-9\$\-\_\.\+\!\*\'\(\)\,\;\?\&\=]|(\%[a-fA-F0-9]{2})){1,25})?\@)?)?(` + domainName + `)(\:\d{1,5})?)(\/(([` + iriChar + `\;\/\?\:\@\&\=\#\~\-\.\+\!\*\'\(\)\,\_])|(\%[a-fA-F0-9]{2}))*)?(\b|$)`
		email      = `[a-zA-Z0-9\.\_\%\-\+]{1,256}\@` + domainName
	)

	f, err := os.Create("regex.go")
	if err != nil {
		return err
	}
	return regexTmpl.Execute(f, struct {
		WebURL, Email, All string
	}{
		WebURL: webURL,
		Email:  email,
		All:    "(` + webURL + `|` + email + `)",
	})
}

func main() {
	tlds, err := tldList()
	if err != nil {
		log.Fatalf("Could not get TLD list: %s", err)
	}
	if err := writeTlds(tlds); err != nil {
		log.Fatalf("Could not write tlds.go: %s", err)
	}
	if err := writeRegex(tlds); err != nil {
		log.Fatalf("Could not write regex.go: %s", err)
	}
}
