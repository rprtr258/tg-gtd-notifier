(import
  sys
  [datetime [datetime date]]
  [json [dumps loads]]
  [configparser [ConfigParser]]
  [random [sample]]
  [time [sleep]]
  [os [environ]]
  [pytz [timezone]])
(import requests)

(sys.stdout.reconfigure :encoding "utf-8")

(defn safe-get [from key] (try (get from key) (except [KeyError] None)))

(setv config (ConfigParser))
((. config read) "config.ini")
(setv TOKEN-V2 (or (safe-get environ "TOKEN-V2") (get config "notion" "TOKEN-V2")))
(setv TODOS-ID (or (safe-get environ "TODOS-ID") (get config "notion" "TODOS-ID")))
(setv CALENDAR-ID (or (safe-get environ "CALENDAR-ID") (get config "notion" "CALENDAR-ID")))
(setv TELEGRAM-TOKEN (or (safe-get environ "TELEGRAM-TOKEN") (get config "tg" "TOKEN")))
(setv TELEGRAM-CHAT-ID (or (safe-get environ "TELEGRAM-CHAT-ID") (get config "tg" "CHAT-ID")))

(defn format-date [date] (date.strftime "%d %B %Y"))

(defn tag [tag-name text] f"<{tag-name}>{text}</{tag-name}>")

(defn format-list [xs] (->>
  xs
  (map (fn [x] f"- {x}"))
  list))

(defn sample-todos [items] (->>
  items
  (sample :k 3)
  (map (fn [x] f"{x}"))
  format-list))

(defn format-dues [items] (->>
  items
  (map (fn [x] f"{(format-date x.due.start)} - {x}"))
  format-list))

(defn get-today-plans [calendar-items today-date] (do
  (->
    (lfor
      x calendar-items
      :if (= (.date (datetime.strptime (get x "properties" "Date" "date" "start") "%Y-%m-%d")) today-date)
      (get x "properties" "Name" "title" 0 "plain_text"))
    format-list)))

(defn split [pred lst] [
  (list (filter pred lst))
  (list (remove pred lst))])

(defn compose-message [today-date due-plans todo-plans today-plans] ((. "\n" join)
  (+
    [(tag "b" f"📆 Сегодня {(format-date today-date)}")]
    [""]
    [(tag "i" "🌟 Планы на сегодня:")]
    today-plans
    [""]
    [(tag "i" "⌛ Дедлайны:")]
    due-plans
    [""]
    [(tag "i" "✨ Что еще можно сделать:")]
    todo-plans)))

(defn get-now [] (datetime.now :tz (timezone "Europe/Moscow")))

(defn notion/list [id]
  (-> f"https://api.notion.com/v1/databases/{id}/query"
    (requests.post :headers {
    "Authorization" f"Bearer {TOKEN-V2}"
    "Notion-Version" "2021-05-13"})
    (. content)
    loads
    (get "results")))

(defn send-notification [] (do
  (setv todo-items (lfor x (notion/list TODOS-ID) (get x "properties" "Name" "title" 0 "plain_text")))
  (setv calendar-items (notion/list CALENDAR-ID))
  (setv today-date (.date (get-now)))
  (setv today-plans (get-today-plans calendar-items today-date))
  (setv [due-todos not-due-todos] (split (fn [x] False) todo-items)) ;x.due) todo-plans)) ; TODO: fix
  (setv message (compose-message today-date (format-dues due-todos) (sample-todos not-due-todos) today-plans))
  (setv tg-response (->
    (requests.get
      f"https://api.telegram.org/bot{TELEGRAM-TOKEN}/sendMessage"
      :data {
        "chat_id" TELEGRAM-CHAT-ID
        "parse_mode" "HTML"
        "text" message
      })
    (. content)
    ((fn [x] (x.decode "utf-8")))
    loads))
  (if-not
    (get tg-response "ok")
    (do
      (print "Error occured. Telegram API response:")
      (->
        tg-response
        (dumps :indent 2)
        print))
    (print "Sent succesfuly"))))

(defn sleep-hour [] (sleep (* 60 60)))

(if (and (> (len sys.argv) 1) (= (get sys.argv 1) "--debug")) (send-notification))
(while
  True
  (do
    (if (= (. (get-now) hour) 9) (send-notification))
      (sleep-hour)))