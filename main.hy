(import
  sys
  [glob [glob]]
  [datetime [datetime date]]
  [json [dumps loads]]
  [configparser [ConfigParser]]
  [random [sample]]
  [time [sleep]]
  [os [environ name path]]
  [pytz [timezone]])
(import requests)

(if (= name "nt") (sys.stdout.reconfigure :encoding "utf-8"))

(defn safe-get [from key] (try (get from key) (except [KeyError] None)))

(setv config (ConfigParser))
((. config read) "config.ini")
(setv TELEGRAM-TOKEN (or (safe-get environ "TELEGRAM-TOKEN") (get config "tg" "TOKEN")))
(setv TELEGRAM-CHAT-ID (or (safe-get environ "TELEGRAM-CHAT-ID") (get config "tg" "CHAT-ID")))

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

(defn get-today-plans [calendar-items today-date] (do
  (->
    (lfor
      x calendar-items
      :if (=
        (.date (datetime.strptime (.join "" (take 10 (get x "properties" "Date" "date" "start"))) "%Y-%m-%d"))
        today-date)
      (do
        (setv title (get x "properties" "Name" "title" 0 "plain_text"))
        (setv date (get x "properties" "Date" "date" "start"))
        (+ (if (> (len date) 10) (+ (get (->
          (datetime.strptime
            (.join "" (get (get x "properties" "Date" "date" "start") (slice 11 16)))
            "%H:%M")
          .time
          str) (slice None 5)) " ") "") title)))
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

(defn gtd/get-items [dir]
  (lfor filename (glob (path.join "/home/rprtr258/GTD/" dir "*.md"))
    (.readline (open filename))))

(defn gtd/get-todos [] (gtd/get-items "next_actions"))

(defn gtd/get-calendar-items [] (gtd/get-items "calendar"))

(defn send-notification [] (do
  (setv todo-items (gtd/get-todos))
  (setv calendar-items (gtd/get-calendar-items))
  (setv today-date (.date (get-now)))
  ;(setv today-plans (get-today-plans calendar-items today-date))
  (setv today-plans ["aboba"])
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
    (print "Sent succesfuly"))))

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
