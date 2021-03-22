(import
  sys
  [datetime [datetime date]]
  [json [dumps loads]]
  [configparser [ConfigParser]]
  [random [sample]]
  [time [sleep]]
  [os [environ]]
  [pytz [timezone]])
(import
  [notion.client [NotionClient]]
  requests)
(import hotfix)

; (sys.stdout.reconfigure :encoding "utf-8")


(setv config (ConfigParser))
((. config read) "config.ini")
(setv NOTION-TOKEN-V2 (or (get environ "NOTION-TOKEN-V2") (get config "notion" "TOKEN-V2")))
(setv GTD-URL (or (get environ "GTD-URL") (get config "notion" "GTD-URL")))
(setv CALENDAR-URL (or (get environ "CALENDAR-URL") (get config "notion" "CALENDAR-URL")))
(setv TELEGRAM-TOKEN (or (get environ "TELEGRAM-TOKEN") (get config "tg" "TOKEN")))
(setv TELEGRAM-CHAT-ID (or (get environ "TELEGRAM-CHAT-ID") (get config "tg" "CHAT-ID")))

(defn format-date [date] (date.strftime "%d %B %Y"))

(defn get-block-by-title [page title]
  (next
    (filter
      (fn [x] (and (hasattr x "title") (= x.title title)))
      page.children)))

(defn tag [tag-name text] f"<{tag-name}>{text}</{tag-name}>")

(defn format-list [xs] (->>
  xs
  (map (fn [x] f"- {x}"))
  list))

(defn get-todo-icon [todo-item] (do
  (setv projects (todo-item.get-property "project"))
  (if
    (> (len projects) 0)
    (do
      (setv project (get projects 0))
      (setv icon (. project icon))
      (if icon icon ""))
    "")))

(defn get-todo-plans [page] (do
  (setv todos-list (get-block-by-title page "Следующие конкретные действия"))
  (->>
    (todos-list.collection.get-rows)
    list)))

(defn sample-todos [items] (->>
  items
  (sample :k 3)
  (map (fn [x] f"{(get-todo-icon x)}{x.title}"))
  format-list))

(defn format-dues [items] (->>
  items
  (map (fn [x] f"{(format-date x.due.start)} - {(get-todo-icon x)}{x.title}"))
  format-list))

(defn get-today-plans [calendar today-date] (do
  (->
    (lfor
      x (calendar.collection.get-rows)
      :if (or
        (= x.date.start today-date)
        (and
          (isinstance x.date.start datetime)
          (= (x.date.start.date) today-date)))
      x.title)
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

(defn send-notification [] (do
  (setv client (NotionClient :token-v2 NOTION-TOKEN-V2))
  (setv page (client.get-block GTD-URL))
  (setv calendar (client.get-block CALENDAR-URL))
  (setv today-date (.date (datetime.now :tz (timezone "Europe/Moscow"))))
  (setv today-plans (get-today-plans calendar today-date))
  (setv todo-plans (get-todo-plans page))
  (setv [due-todos not-due-todos] (split (fn [x] x.due) todo-plans))
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


(send-notification)
(while
  True
    (do
      (if (= (. (datetime.now) hour) 9) (send-notification))
      (sleep-hour)))
