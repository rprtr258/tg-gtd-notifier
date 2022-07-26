package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const (
	_dateFormat   = "02 January 2006"
	_githubApiURL = "https://api.github.com/repos/rprtr258/gtdBackup/contents/"
)

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

type Task struct {
	Title string
	// TODO: separate for delayed tasks
	Date time.Time
}

func tag(tagName, text string) string {
	return fmt.Sprintf("<%[1]s>%[2]s</%[1]s>", tagName, text)
}

// TODO: template
func formatList[A any](xs []A) []string {
	res := make([]string, 0, len(xs))
	for _, x := range xs {
		res = append(res, fmt.Sprintf(" %+v", x))
	}
	return res
}

func mySample3[A any](items []A) []A {
	if len(items) < 3 {
		return items
	}
	// TODO: sample 3 items out of `items`
	return items
}

func formatDues(items []Task) []string {
	res := make([]string, 0, len(items))
	for _, x := range mySample3(items) {
		res = append(res, fmt.Sprintf("%s %s", x.Date.Format(_dateFormat), x))
	}
	return res
}

func getTodayTasks(calendarItems []Task, todayDate time.Time) []Task {
	res := make([]Task, 0, len(calendarItems))
	for _, item := range calendarItems {
		if item.Date.Before(todayDate) || item.Date.Equal(todayDate) {
			res = append(res, item)
		}
	}
	return res
}

func composeMessage(todayDate time.Time, duePlans, todoPlans, todayTasks []Task) string {
	var lines []string
	lines = append(lines, tag("b", fmt.Sprintf("📆 Сегодня %s", todayDate.Format(_dateFormat))))
	lines = append(lines, "")
	lines = append(lines, tag("i", "🌟 Планы на сегодня:")) // TODO: don't print if empty
	lines = append(lines, formatList(todayTasks)...)
	lines = append(lines, "")
	lines = append(lines, tag("i", "⌛ Дедлайны:")) // TODO: implement
	lines = append(lines, formatDues(duePlans)...)
	lines = append(lines, "")
	lines = append(lines, tag("i", "✨ Что еще можно сделать:"))
	lines = append(lines, formatList(todoPlans)...)
	return strings.Join(lines, "\n")
}

func sendTgMessage(message string) error {
	_, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", _telegramToken))
	// :data {
	//   "chat_id" TELEGRAMcHATiD
	//   "parse_mode" "HTML"
	//   "text" message
	// }
	if err != nil {
		return err
	}
	// TODO: get response json
	// if !tgResponse["ok"] {
	// 	return fmt.Errorf("Error occured. Telegram API response: %s", tgResponse)
	// }
	return nil
}

type GithubFileEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        uint   `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GITURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

func githubApiGetFilesList(dir string) []GithubFileEntry {
	var v []GithubFileEntry
	json.Unmarshal(wtf(ioutil.ReadAll(wtf(http.Get(_githubApiURL+dir)).Body)), &v)
	// :auth ("rprtr258", GITHUB_OAUTH)
	return v
}

type GithubFileContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        uint   `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GITURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
}

func githubApiGetFileContent(dir, filename string) GithubFileContent {
	http.Get(_githubApiURL + fmt.Sprintf("%s/%s", dir, filename))
	// :auth ("rprtr258", GITHUB_OAUTH)
	var v GithubFileContent
	json.Unmarshal(wtf(ioutil.ReadAll(wtf(http.Get(_githubApiURL+dir)).Body)), &v)
	// TODO: decode base64, then utf8
	return v
}

func parsePlan(fileContent string) Task {
	return Task{
		Title: fileContent,
		Date:  time.Now(),
		// Date:  wtf(time.Parse("02.01.2006", string(regexp.MustCompile(`Date: (\d{2}\.\d{2}\.\d{4})`).FindSubmatch([]byte(x))[0]))),
	}
}

func gtdGetItems(dir string) []Task {
	var res []Task
	for _, file := range githubApiGetFilesList(dir) {
		if path.Ext(file.Name) != ".md" {
			continue
		}
		plan := githubApiGetFileContent(dir, file.Name).Content
		res = append(res, parsePlan(plan))
	}
	return res
}

func getNow() time.Time {
	return time.Now().In(_moscowTZ)
}

func sendNotification() error {
	now := getNow()
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, _moscowTZ)

	message := composeMessage(
		todayDate,
		gtdGetItems("next_actions"),
		[]Task{},
		getTodayTasks(gtdGetItems("calendar"), todayDate),
	)
	if err := sendTgMessage(message); err != nil {
		return err
	}

	return nil
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
		time.Sleep(time.Hour)
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
