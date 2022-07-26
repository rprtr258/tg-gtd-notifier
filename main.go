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
	"text/template"
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

// res = append(res, fmt.Sprintf("(%s) %s", x.When.Format("02.01.2006"), x.Title))

func composeMessage(today time.Time, todayTasks []CalendarTask, todoTasks []Task) string {
	var res strings.Builder
	messageTemplate := template.Must(template.New("").Parse(`<b>📆 Сегодня {{.Today.Format "02.01.2006"}}</b>

{{if (gt (len .TodayTasks) 0)}}<i>🌟 Планы на сегодня:</i>{{range .TodayTasks}}
- ({{.When.Format "02.01.2006"}}) {{.Title}}{{end}}{{end}}
{{if (gt (len .DelayedTasks) 0)}}<i>⌛ Дедлайны:</i>{{range .DelayedTasks}}
- ({{.When.Format "02.01.2006"}}) {{.Title}}{{end}}{{end}}
{{if (gt (len .TodoTasks) 0)}}<i>✨ Что еще можно сделать:</i>{{range .TodoTasks}}
- {{.Title}}{{end}}{{end}}`))
	if err := messageTemplate.Execute(&res, struct {
		Today      time.Time
		TodayTasks []CalendarTask
		TodoTasks  []Task
		// TODO: implement
		DelayedTasks []CalendarTask
	}{
		Today:      today,
		TodayTasks: todayTasks,
		TodoTasks:  todoTasks,
	}); err != nil {
		panic(err.Error())
	}
	return res.String()
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
	lines := strings.Split(fileContent, "\n")
	return Task{
		Title: lines[0],
	}
}

func listMDFiles(dir string) []string {
	allFiles := githubApiGetFilesList(dir)
	res := make([]string, 0, len(allFiles))
	for _, file := range allFiles {
		if path.Ext(file.Name) != ".md" {
			continue
		}

		tmp := strings.Join(strings.Split(githubApiGetFileContent(dir, file.Name).Content, "\n"), "")
		taskContent, err := base64.StdEncoding.DecodeString(tmp)
		if err != nil {
			panic(err.Error())
		}

		res = append(res, string(taskContent))
	}
	return res
}

func gtdGetItems(dir string) []Task {
	var res []Task
	for _, taskContent := range listMDFiles(dir) {
		res = append(res, parseTask(taskContent))
	}
	return res
}

func gtdGetCalendarItems(dir string) []CalendarTask {
	var res []CalendarTask
	for _, taskContent := range listMDFiles(dir) {
		res = append(res, parseCalendarTask(string(taskContent)))
	}
	return res
}

func getTodayDate() time.Time {
	now := time.Now().In(_moscowTZ)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, _moscowTZ)
}

func run() error {
	today := getTodayDate()

	message := composeMessage(
		today,
		getTodayTasks(gtdGetCalendarItems("calendar"), today),
		gtdGetItems("next_actions"),
	)

	if err := sendTgMessage(message); err != nil {
		return err
	}

	return nil
}

// TODO: remove
func wtf[A any](a A, err error) A {
	if err != nil {
		panic(err.Error())
	}
	return a
}

// TODO: cron 0 9 * * *
func main() {
	if err := run(); err != nil {
		log.Fatal(err.Error())
	}
}
