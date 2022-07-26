package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	_dateFormat   = "02 January 2006"
	_githubApiURL = "https://api.github.com/repos/rprtr258/gtd-backup/contents/"
)

var (
	_telegramToken  = os.Getenv("TELEGRAM_TOKEN")
	_telegramChatID = os.Getenv("TELEGRAM_CHAT_ID")
	_githubOAuth    = os.Getenv("GITHUB_OAUTH")

	_moscowTZ = wtf(time.LoadLocation("Europe/Moscow"))
)

type Task struct {
	Title string
}

type CalendarTask struct {
	Task
	When time.Time
}

func tag(tagName, text string) string {
	return fmt.Sprintf("<%[1]s>%[2]s</%[1]s>", tagName, text)
}

// TODO: template
func formatTasks(xs []Task) []string {
	res := make([]string, 0, len(xs))
	for _, x := range xs {
		res = append(res, x.Title)
	}
	return res
}

func formatCalendarTasks(xs []CalendarTask) []string {
	res := make([]string, 0, len(xs))
	for _, x := range xs {
		res = append(res, fmt.Sprintf("(%s) %s", x.When.Format("02.01.2006"), x.Title))
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

func formatDues(items []CalendarTask) []string {
	res := make([]string, 0, len(items))
	for _, x := range mySample3(items) {
		res = append(res, fmt.Sprintf("%s %s", x.When.Format(_dateFormat), x))
	}
	return res
}

func getTodayTasks(calendarItems []CalendarTask, todayDate time.Time) []CalendarTask {
	res := make([]CalendarTask, 0, len(calendarItems))
	for _, item := range calendarItems {
		if item.When.Before(todayDate) || item.When.Equal(todayDate) {
			res = append(res, item)
		}
	}
	return res
}

func composeMessage(todayDate time.Time, duePlans, todayTasks []CalendarTask, todoTasks []Task) string {
	var lines []string
	lines = append(lines, tag("b", fmt.Sprintf("📆 Сегодня %s", todayDate.Format(_dateFormat))))
	lines = append(lines, "")
	lines = append(lines, tag("i", "🌟 Планы на сегодня:")) // TODO: don't print if empty
	lines = append(lines, formatCalendarTasks(todayTasks)...)
	lines = append(lines, "")
	// TODO: implement
	// lines = append(lines, tag("i", "⌛ Дедлайны:"))
	// lines = append(lines, formatDues(duePlans)...)
	// lines = append(lines, "")
	lines = append(lines, tag("i", "✨ Что еще можно сделать:"))
	lines = append(lines, formatTasks(todoTasks)...)
	return strings.Join(lines, "\n")
}

func sendTgMessage(message string) error {
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", _telegramToken), nil)
	if err != nil {
		return err
	}

	query := request.URL.Query()
	query.Add("chat_id", _telegramChatID)
	query.Add("parse_mode", "HTML")
	query.Add("text", message)
	request.URL.RawQuery = query.Encode()

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var v struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal(body, &v); err != nil {
		return err
	}

	if !v.Ok {
		return fmt.Errorf("telegram api error: error_code=%d description=%q", v.ErrorCode, v.Description)
	}

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

func githubApiRequest(addr string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		panic(err.Error())
	}
	request.SetBasicAuth("rprtr258", _githubOAuth)

	return http.DefaultClient.Do(request)
}

func githubApiGetFilesList(dir string) []GithubFileEntry {
	response, err := githubApiRequest(_githubApiURL + dir)
	if err != nil {
		panic(err.Error())
	}

	s := wtf(ioutil.ReadAll(response.Body))

	var v []GithubFileEntry
	json.Unmarshal(s, &v)

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
	response, err := githubApiRequest(_githubApiURL + fmt.Sprintf("%s/%s", dir, filename))
	if err != nil {
		panic(err.Error())
	}

	s := wtf(ioutil.ReadAll(response.Body))

	var v GithubFileContent
	if err := json.Unmarshal(s, &v); err != nil {
		panic(err.Error())
	}
	return v
}

func parseCalendarTask(fileContent string) CalendarTask {
	r := regexp.MustCompile(`Date: (\d{2}\.\d{2}\.\d{4})`).FindSubmatch([]byte(fileContent))
	lines := strings.Split(fileContent, "\n")
	return CalendarTask{
		Task: Task{
			Title: lines[0],
		},
		When: wtf(time.Parse("02.01.2006", string(r[1]))),
	}
}

func parseTask(fileContent string) Task {
	return Task{
		Title: fileContent,
	}
}

func gtdGetItems(dir string) []Task {
	var res []Task
	for _, file := range githubApiGetFilesList(dir) {
		if path.Ext(file.Name) != ".md" {
			continue
		}

		tmp := strings.Join(strings.Split(githubApiGetFileContent(dir, file.Name).Content, "\n"), "")
		plan, err := base64.StdEncoding.DecodeString(tmp)
		if err != nil {
			panic(err.Error())
		}

		res = append(res, parseTask(string(plan)))
	}
	return res
}

func gtdGetCalendarItems(dir string) []CalendarTask {
	var res []CalendarTask
	for _, file := range githubApiGetFilesList(dir) {
		if path.Ext(file.Name) != ".md" {
			continue
		}

		tmp := strings.Join(strings.Split(githubApiGetFileContent(dir, file.Name).Content, "\n"), "")
		taskContent, err := base64.StdEncoding.DecodeString(tmp)
		if err != nil {
			panic(err.Error())
		}

		res = append(res, parseCalendarTask(string(taskContent)))
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
		[]CalendarTask{},
		getTodayTasks(gtdGetCalendarItems("calendar"), todayDate),
		gtdGetItems("next_actions"),
	)

	log.Println(message)

	if err := sendTgMessage(message); err != nil {
		return err
	}

	return nil
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "-debug" {
		if err := sendNotification(); err != nil {
			return err
		}
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

// TODO: remove
func wtf[A any](a A, err error) A {
	if err != nil {
		panic(err.Error())
	}
	return a
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err.Error())
	}
}
