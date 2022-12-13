package main

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gm "github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v2"
)

const (
	_dateFormat   = "02 January 2006"
	_githubApiURL = "https://api.github.com/repos/rprtr258/gtd-backup/contents/"
)

var (
	_telegramToken  = must(getEnv("TELEGRAM_TOKEN"))
	_telegramChatID = must(strconv.Atoi(must(getEnv("TELEGRAM_CHAT_ID"))))
	_githubOAuth    = must(getEnv("GITHUB_OAUTH"))

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

	s, err := io.ReadAll(response.Body)
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
		if item.Delayed {
			res = append(res, item)
			continue
		}
		if item.When.Year() != todayDate.Year() {
			if item.When.Year() < todayDate.Year() {
				res = append(res, item)
			}
			continue
		}
		if item.When.Month() != todayDate.Month() {
			if item.When.Month() < todayDate.Month() {
				res = append(res, item)
			}
			continue
		}
		if item.When.Day() <= todayDate.Day() {
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

func parseCalendarTask(fileContent string) (CalendarTask, error) {
	node := gm.New().Parser().Parse(text.NewReader([]byte(fileContent)))

	var metadata string
	if true { // TODO: many safety checks
		lines := node.
			FirstChild().
			NextSibling().
			Lines()
		metadata = string(fileContent[lines.At(0).Start:lines.At(lines.Len()-1).Stop])
	} else {
		return CalendarTask{}, errors.New("incorrect file")
	}

	type metadataT struct {
		Title *string `yaml:"title"`
		Date  *myDate `yaml:"date"`
	}
	var meta metadataT
	if err := yaml.NewDecoder(bytes.NewReader([]byte(metadata + "\n"))).Decode(&meta); err != nil {
		return CalendarTask{}, err
	}

	if meta.Date == nil {
		return CalendarTask{}, errors.New("date is missing")
	}

	if meta.Title == nil {
		return CalendarTask{}, errors.New("title is missing")
	}

	return CalendarTask{
		Task: Task{
			Title: *meta.Title,
		},
		When:    time.Time(*meta.Date),
		Delayed: false, // string(r[1]) == "Until",
	}, nil
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
		task, err := parseCalendarTask(string(taskContent))
		if err != nil {
			return nil, fmt.Errorf("invalid file %q: %w", taskContent, err)
		}

		res = append(res, task)
	}
	return res, nil
}

func getTodayDate() time.Time {
	now := time.Now().In(time.Local)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

func run() error {
	bot, err := tg.NewBotAPI(_telegramToken)
	if err != nil {
		return err
	}

	updateConfig := tg.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	when := time.Now().
		Truncate(time.Hour*24).
		AddDate(0, 0, 1).
		Add(time.Hour * 6)
	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			f, err := os.OpenFile("/home/rprtr258/GTD/done.md", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				panic(err)
			}

			defer f.Close()

			message := strings.ReplaceAll(update.Message.Text, "\n", " ")
			if _, err = f.WriteString(fmt.Sprintf("%s: %s\n", time.Now().Format(time.RFC1123), message)); err != nil {
				panic(err)
			}

			msg := tg.NewMessage(update.Message.Chat.ID, "noted")
			msg.ReplyToMessageID = update.Message.MessageID

			if _, err := bot.Send(msg); err != nil {
				return err
			}
		case <-time.After(time.Until(when)):
			when = time.Now().
				Truncate(time.Hour*24).
				AddDate(0, 0, 1).
				Add(time.Hour * 6)

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

			msg := tg.NewMessage(int64(_telegramChatID), message)
			msg.ParseMode = "HTML"
			if _, err := bot.Send(msg); err != nil {
				return err
			}
		}
	}
}

type myDate time.Time

var _ = encoding.TextUnmarshaler(&myDate{})

func (date *myDate) UnmarshalText(data []byte) error {
	date1, err := time.Parse("02.01.2006", string(data))
	if err != nil {
		return fmt.Errorf("invalid date, date=%q: %w", string(data), err)
	}

	*date = myDate(date1)
	return nil
}

func main() {
	rand.Seed(time.Now().Unix())
	if err := run(); err != nil {
		log.Fatal(err.Error())
	}
}
