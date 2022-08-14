package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
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
	_telegramToken  = must(getEnv("TELEGRAM_TOKEN"))
	_telegramChatID = must(getEnv("TELEGRAM_CHAT_ID"))
	_githubOAuth    = must(getEnv("GITHUB_OAUTH"))

	_telegramApiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", _telegramToken)

	_dateRegex       = regexp.MustCompile(`(Date|Until): (\d{2}\.\d{2}\.\d{4})`)
	_moscowTZ        = must(time.LoadLocation("Europe/Moscow"))
	_messageTemplate = must(template.New("").Parse(`<b>📆 Сегодня {{.Today.Format "02 January 2006"}}</b>{{if gt (len .TodayTasks) 0}}

<i>🌟 Планы на сегодня:</i>{{range .TodayTasks}}
- {{if (.When.Before $.Today)}}({{.When.Format "02.01.2006"}}) {{end}}{{.Title}}{{end}}{{end}}{{if gt (len .DelayedTasks) 0}}

<i>⌛ Дедлайны:</i>{{range .DelayedTasks}}
- ({{.When.Format "02.01.2006"}}) {{.Title}}{{end}}{{end}}{{if gt (len .NextActions) 0}}

<i>✨ Что еще можно сделать:</i>{{range .NextActions}}
- {{.Title}}{{end}}{{end}}`))
)

func getEnv(key string) (string, error) {
	res, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("not found in env: %q", key)
	}
	return res, nil
}

func must[A any](a A, err error) A {
	if err != nil {
		panic(err.Error())
	}
	return a
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

func githubApiRequest[A any](addr string) (*A, error) {
	request, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth("rprtr258", _githubOAuth)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	s, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var v A
	if err := json.Unmarshal(s, &v); err != nil {
		return nil, fmt.Errorf("error parsing json into %T: %q", v, string(s))
	}

	return &v, nil
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

func githubApiGetFilesList(dir string) ([]GithubFileEntry, error) {
	res, err := githubApiRequest[[]GithubFileEntry](_githubApiURL + dir)
	if err != nil {
		return nil, err
	}
	return *res, nil
}

func githubApiGetFileContent(dir, filename string) (*GithubFileContent, error) {
	return githubApiRequest[GithubFileContent](_githubApiURL + fmt.Sprintf("%s/%s", dir, filename))
}

type Task struct {
	Title string
}

type CalendarTask struct {
	Task
	When    time.Time
	Delayed bool
}

func sample[A any](items []A, k int) []A {
	if len(items) < k {
		return items
	}
	perm := rand.Perm(len(items))
	res := make([]A, 0, k)
	for i := 0; i < len(perm) && i < k; i++ {
		res = append(res, items[perm[i]])
	}
	return res
}

func getTodayTasks(calendarItems []CalendarTask, todayDate time.Time) []CalendarTask {
	res := make([]CalendarTask, 0, len(calendarItems))
	for _, item := range calendarItems {
		if !item.Delayed && (item.When.Before(todayDate) || item.When.Equal(todayDate)) {
			res = append(res, item)
		}
	}
	return res
}

func getDelayedTasks(calendarItems []CalendarTask) []CalendarTask {
	res := make([]CalendarTask, 0, len(calendarItems))
	for _, item := range calendarItems {
		if item.Delayed {
			res = append(res, item)
		}
	}
	return res
}

type messageData struct {
	Today        time.Time
	TodayTasks   []CalendarTask
	NextActions  []Task
	DelayedTasks []CalendarTask
}

func composeMessage(messageData messageData) string {
	var res strings.Builder
	if err := _messageTemplate.Execute(&res, messageData); err != nil {
		panic(err.Error())
	}
	return res.String()
}

func sendTgMessage(message string) error {
	request, err := http.NewRequest(http.MethodGet, _telegramApiURL, nil)
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

func parseCalendarTask(fileContent string) CalendarTask {
	r := _dateRegex.FindSubmatch([]byte(fileContent))
	lines := strings.Split(fileContent, "\n")
	return CalendarTask{
		Task: Task{
			Title: strings.Trim(lines[0], "# "),
		},
		When:    must(time.Parse("02.01.2006-07:00", string(r[2])+"+03:00")),
		Delayed: string(r[1]) == "Until",
	}
}

func parseTask(fileContent string) Task {
	lines := strings.Split(fileContent, "\n")
	return Task{
		Title: lines[0],
	}
}

func listMDFiles(dir string) ([]string, error) {
	allFiles, err := githubApiGetFilesList(dir)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(allFiles))
	for _, file := range allFiles {
		if path.Ext(file.Name) != ".md" {
			continue
		}

		content, err := githubApiGetFileContent(dir, file.Name)
		if err != nil {
			return nil, err
		}

		tmp := strings.Join(strings.Split(content.Content, "\n"), "")
		taskContent, err := base64.StdEncoding.DecodeString(tmp)
		if err != nil {
			panic(err.Error())
		}

		res = append(res, string(taskContent))
	}
	return res, nil
}

func gtdGetItems(dir string) ([]Task, error) {
	var res []Task
	files, err := listMDFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, taskContent := range files {
		res = append(res, parseTask(taskContent))
	}
	return res, nil
}

func gtdGetCalendarItems(dir string) ([]CalendarTask, error) {
	var res []CalendarTask
	files, err := listMDFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, taskContent := range files {
		res = append(res, parseCalendarTask(string(taskContent)))
	}
	return res, nil
}

func getTodayDate() time.Time {
	now := time.Now().In(_moscowTZ)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, _moscowTZ)
}

func run() error {
	today := getTodayDate()

	calendarTasks, err := gtdGetCalendarItems("calendar")
	if err != nil {
		return err
	}
	todayTasks := getTodayTasks(calendarTasks, today)
	delayedTasks := getDelayedTasks(calendarTasks)

	nextActionsTasks, err := gtdGetItems("next_actions")
	if err != nil {
		return err
	}

	message := composeMessage(messageData{
		Today:        today,
		TodayTasks:   todayTasks,
		NextActions:  sample(nextActionsTasks, 3),
		DelayedTasks: delayedTasks,
	})

	if err := sendTgMessage(message); err != nil {
		return err
	}

	return nil
}

func main() {
	rand.Seed(time.Now().Unix())
	if err := run(); err != nil {
		log.Fatal(err.Error())
	}
}
