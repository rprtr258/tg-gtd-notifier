package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const _dir = "/home/rprtr258/GTD/calendar"

func main() {
	f, _ := os.ReadDir(_dir)
	for _, x := range f {
		if path.Ext(x.Name()) != ".md" {
			continue
		}
		file, _ := os.OpenFile(_dir+"/"+x.Name(), os.O_RDWR, os.ModeType)
		a, _ := ioutil.ReadAll(file)
		sa := string(a)
		if !strings.Contains(sa, "Period: ") {
			continue
		}
		var (
			date        string
			periodCount int
			periodUnit  string
		)
		for _, line := range strings.Split(sa, "\n") {
			if strings.Contains(line, "Date: ") {
				date = strings.Split(line, " ")[1]
			}
			if strings.Contains(line, "Period: ") {
				fmt.Sscanf(strings.Split(line, " ")[1], "%d%s", &periodCount, &periodUnit)
			}
		}
		t, _ := time.Parse("02.01.2006", date)
		fmt.Println(x.Name())
		fmt.Printf("%s %d %s\n", t.Format("02.01.2006"), periodCount, periodUnit)
		switch periodUnit {
		case "y":
			t = t.AddDate(periodCount, 0, 0)
		case "m":
			t = t.AddDate(0, periodCount, 0)
		case "w":
			t = t.AddDate(0, 0, 7*periodCount)
		case "d":
			t = t.AddDate(0, 0, periodCount)
		default:
			panic(periodUnit)
		}
		fmt.Println(t.Format("02.01.2006"))
	}
}
