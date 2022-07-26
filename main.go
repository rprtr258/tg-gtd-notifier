package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

const _dateFormat = "02 January 2006"

// [tg]
// TOKEN = 0000000000:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
// CHAT-ID = 000000000
// [github]
// OAUTH = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
var (
	_telegramToken  = os.Getenv("TELEGRAM_TOKEN")
	_telegramChatID = os.Getenv("TELEGRAM_CHAT_ID")
	_githubOAuth    = os.Getenv("GITHUB_OAUTH")

	_moscowTZ = wtf(time.LoadLocation("Europe/Moscow"))
)

type Plan struct {
	Date  time.Time
	Title string
}

func tag(tagName, text string) string {
	return fmt.Sprintf("<%[1]>%[2]</%[1]>", tagName, text)
}

// TODO: template
func formatList(xs []any) []string {
	res := make([]string, 0, len(xs))
	for _, x := range xs {
		res = append(res, fmt.Sprintf(" %+v", x))
	}
	return res
}

func mySample3(items []any) []any {
	if len(items) < 3 {
		return items
	}
	// TODO: sample 3 items out of `items`
	return items
}

func sampleTodos(items []any) []string {
	res := make([]string, 0, len(items))
	for _, x := range mySample3(items) {
		res = append(res, fmt.Sprint(x))
	}
	return res
}

func formatDues(items []struct {
	Due struct {
		Start time.Time
	}
}) []string {
	res := make([]string, 0, len(items))
	for _, x := range mySample3(items) {
		res = append(res, fmt.Sprintf("%s %s", x.Due.Start.Formate(_dateFormat), x))
	}
	return res
}

func getTodayPlans(calendarItems []Plan, todayDate time.Time) []string {
	res := make([]any, 0, len(calendarItems))
	for _, item := range calendarItems {
		if item.Date.Before(todayDate) || item.Date.Equal(todayDate) {
			res = append(res, item.Title)
		}
	}
	return formatList(res)
}

func composeMessage(todayDate time.Time, duePlans, todoPlans, todayPlans []string) string {
	var lines []string
	lines = append(lines, tag("b", fmt.Sprintf("📆 Сегодня %s", todayDate.Format(_dateFormat))))
	lines = append(lines, "")
	lines = append(lines, tag("i", "🌟 Планы на сегодня:")) // TODO: don't print if empty
	lines = append(lines, todayPlans...)
	lines = append(lines, "")
	lines = append(lines, tag("i", "⌛ Дедлайны:"))
	lines = append(lines, duePlans...)
	lines = append(lines, "")
	lines = append(lines, tag("i", "✨ Что еще можно сделать:"))
	lines = append(lines, todoPlans...)
	return strings.Join(lines, "\n")
}

func getNow() time.Time {
	return time.Now().In(_moscowTZ)
}

func sendTgMessage(message string) error {
	http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", _telegramToken))
	// :data {
	//   "chat_id" TELEGRAMcHATiD
	//   "parse_mode" "HTML"
	//   "text" message
	// }
	// TODO: get response json
	// if !tgResponse["ok"] {
	// 	return fmt.Errorf("Error occured. Telegram API response: %s", tgResponse)
	// }
	return nil
}

func githubApiGetFilesList(url string) []struct {
	Name string
} {
	http.Get("https://api.github.com" + url)
	// :auth ("rprtr258", GITHUB_OAUTH)
	// TODO: get response json
	return nil
}

func githubApiGetFileContent(url string) string {
	http.Get("https://api.github.com" + url)
	// :auth ("rprtr258", GITHUB_OAUTH)
	// TODO: get response json
	// TODO: get ["content"]
	// TODO: decode base64, then utf8, then split by "\n"
	return ""
}

func gtdGetItems(dir string) []string {
	var res []string
	for _, file := range githubApiGetFilesList(fmt.Sprintf("/repos/rprtr258/gtdBackup/contents/%s", dir)) {
		if path.Ext(file.Name) == ".md" {
			filename := file.Name
			v := githubApiGetFileContent(fmt.Sprintf("/repos/rprtr258/gtdBackup/contents/%s/%s", dir, filename))
			res = append(res, strings.Split(v, "\n")...)
		}
	}
	return nil
}

func gtdGetTodos() []struct {
	Due struct {
		Start time.Time
	}
} {
	var res []struct {
		Due struct {
			Start time.Time
		}
	}
	for _, x := range gtdGetItems("next_actions") {
		res = append(res, struct{ Due struct{ Start time.Time } }{
			Due: struct{ Start time.Time }{
				Start: time.Now(),
			},
		})
	}
	return res
}

func gtdGetCalendarItems() []Plan {
	var res []Plan
	for _, x := range gtdGetItems("calendar") {
		res = append(res, Plan{
			Date:  wtf(time.Parse("02.01.2006", string(regexp.MustCompile(`Date: (\d{2}\.\d{2}\.\d{4})`).FindSubmatch([]byte(x))[0]))),
			Title: x,
		})
	}
	return res
}

func sendNotification() error {
	todoItems := gtdGetTodos()
	calendarItems := gtdGetCalendarItems()
	now := getNow()
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, _moscowTZ)
	todayPlans := getTodayPlans(calendarItems, todayDate)
	dueTodos, notDueTodos := todoItems, []any{} // TODO: fix
	message := composeMessage(todayDate, formatDues(dueTodos), sampleTodos(notDueTodos), todayPlans)
	if err := sendTgMessage(message); err != nil {
		return err
	}

	return nil
}

func sleepHour() {
	time.Sleep(time.Hour)
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "-debug" {
		sendNotification()
	}
	for {
		// TODO: cron
		if getNow().Hour() == 9 {
			if err := sendNotification(); err != nil {
				return err
			}
		}
		sleepHour()
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err.Error())
	}
}

// TODO: remove
func wtf[A any](a A, err error) A {
	if err != nil {
		panic(err.Error())
	}
	return a
}
