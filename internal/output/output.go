package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type Mode string

const (
	Auto  Mode = "auto"
	Table Mode = "table"
	JSON  Mode = "json"
)

func IsTable(mode Mode) bool {
	if mode == Table {
		return true
	}
	return mode == Auto && isTTY()
}

func PrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func PrintTable(headers []string, rows [][]string) {
	w := make([]int, len(headers))
	for i, h := range headers {
		w[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if n := utf8.RuneCountInString(c); n > w[i] {
				w[i] = n
			}
		}
	}
	printRow(headers, w)
	for _, r := range rows {
		printRow(r, w)
	}
}

func printRow(r []string, widths []int) {
	for i, c := range r {
		fmt.Printf("%-*s", widths[i], c)
		if i < len(r)-1 {
			fmt.Print("  ")
		}
	}
	fmt.Println()
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func ToStrings(v ...any) []string {
	o := make([]string, len(v))
	for i := range v {
		o[i] = strings.TrimSpace(fmt.Sprint(v[i]))
	}
	return o
}
