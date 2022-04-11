(import
  sys
  base64
  re
  [glob [glob]]
  [datetime [datetime date]]
  [json [dumps loads]]
  [configparser [ConfigParser]]
  [random [sample]]
  [time [sleep]]
  [os [environ name]]
  [pytz [timezone]])
(import requests)

(if (= name "nt") (sys.stdout.reconfigure :encoding "utf-8"))

(defn safe-get [from key] (try (get from key) (except [KeyError] None)))

(setv config (ConfigParser))
((. config read) "config.ini")
(setv TELEGRAM-TOKEN (or (safe-get environ "TELEGRAM-TOKEN") (get config "tg" "TOKEN")))
(setv TELEGRAM-CHAT-ID (or (safe-get environ "TELEGRAM-CHAT-ID") (get config "tg" "CHAT-ID")))
(setv GITHUB-OAUTH (or (safe-get environ "GITHUB-OAUTH") (get config "github" "OAUTH")))

(defn format-date [date] (date.strftime "%d %B %Y"))

(defn tag [tag-name text] f"<{tag-name}>{text}</{tag-name}>")

(defn format-list [xs] (->>
  xs
  (map (fn [x] f"- {x}"))
  list))

(defn my-sample-3 [items]
  (if (< (len items) 3) items (sample items :k 3)))

(defn sample-todos [items] (->>
  items
  (my-sample-3)
  (map (fn [x] f"{x}"))
  format-list))

(defn format-dues [items] (->>
  items
  (map (fn [x] f"{(format-date x.due.start)} - {x}"))
  format-list))

(defn get-today-plans [calendar-items today-date]
  (format-list
    (lfor
      [title the_date] calendar-items
      :if (= the_date today-date)
      title)))

(defn split [pred lst] [
  (list (filter pred lst))
  (list (remove pred lst))])

(defn compose-message [today-date due-plans todo-plans today-plans] ((. "\n" join)
  (+
    [(tag "b" f"📆 Сегодня {(format-date today-date)}") "" (tag "i" "🌟 Планы на сегодня:")]
    today-plans
    ["" (tag "i" "⌛ Дедлайны:")]
    due-plans
    ["" (tag "i" "✨ Что еще можно сделать:")]
    todo-plans)))

(defn get-now [] (datetime.now :tz (timezone "Europe/Moscow")))

(defn send-tg-message [message]
  (->
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

(defn github/api [url]
  (->> url
    (+ "https://api.github.com")
    (requests.get :auth (, "rprtr258" GITHUB_OAUTH))
    .json))

(defn gtd/get-items [dir]
  (lfor file (github/api f"/repos/rprtr258/gtd-backup/contents/{dir}")
    :if (.endswith (get file "name") ".md")
    (do
      (setv filename (get file "name"))
      (setv v (github/api f"/repos/rprtr258/gtd-backup/contents/{dir}/{filename}"))
      (-> v
        (get "content")
        base64.b64decode
        (.decode "utf-8")
        (.split "\n")))))

(defn gtd/get-todos [] (lfor x (gtd/get-items "next_actions")
  (-> x (get 0))))

(defn gtd/get-calendar-items [] (lfor x (gtd/get-items "calendar")
  (,
    (get x 0)
    (->
      (->> x
        (.join "\n")
        (re.search r"Date: (\d{2}\.\d{2}\.\d{4})"))
      (.groups)
      (get 0)
      (datetime.strptime "%d.%m.%Y")
      .date))))

(defn send-notification []
  ; TODO: union setv-s
  (setv todo-items (gtd/get-todos))
  (setv calendar-items (gtd/get-calendar-items))
  (setv today-date (.date (get-now)))
  (setv today-plans (get-today-plans calendar-items today-date))
  (setv [due-todos not-due-todos] (split (fn [x] False) todo-items)) ;x.due) todo-items)) ; TODO: fix
  (setv message (compose-message today-date (format-dues due-todos) (sample-todos not-due-todos) today-plans))
  (setv tg-response (send-tg-message message))
  (if-not
    (get tg-response "ok")
    (do
      (print "Error occured. Telegram API response:")
      (->
        tg-response
        (dumps :indent 2)
        print))
    (print "Sent succesfuly")))

(defn sleep-hour [] (sleep (* 60 60)))

(defn main []
  (if (and (> (len sys.argv) 1) (= (get sys.argv 1) "--debug")) (send-notification))
  (while
    True
    (if (= (. (get-now) hour) 9) (send-notification))
      (sleep-hour)))

(try
  (main)
  (except [x [Exception]]
    (import traceback [html [escape]])
    (setv traceback (traceback.format_exc))
    (setv message (.join "\n"
      [(* "=" 30)
      f"Exception occured. Traceback:"
      (* "=" 30)
      traceback]))
    (print message)
    (->
      message
      escape
      send-tg-message)))
