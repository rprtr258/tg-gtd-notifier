(import
  sys
  [datetime [datetime date]]
  [json [dumps loads]]
  [configparser [ConfigParser]]
  [random [sample]]
  [time [sleep]])
(import
  [notion.client [NotionClient]]
  requests)

(sys.stdout.reconfigure :encoding "utf-8")


(setv config (ConfigParser))
((. config read) "config.ini")
(setv NOTION-TOKEN-V2 (get config "notion" "TOKEN-V2"))
(setv GTD-URL (get config "notion" "GTD-URL"))
(setv TELEGRAM-TOKEN (get config "tg" "TOKEN"))
(setv TELEGRAM-CHAT-ID (get config "tg" "CHAT-ID"))


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

(defn compose-message [today-date todo-plans today-plans] ((. "\n" join)
    (+
      [(tag "b" f"📆 Сегодня {today-date}")]
      [""]
      [(tag "i" "✨ Что еще можно сделать:")]
      todo-plans
      [""]
      [(tag "i" "🌟 Планы на сегодня:")]
      today-plans)))

(defn get-todo-plans [page] (do
  (setv todos-list (get-block-by-title page "Следующие конкретные действия"))
  (->>
    (todos-list.collection.get-rows)
    list
    (sample :k 3)
    (map (fn [x] f"{(get-todo-icon x)}{x.title}"))
    format-list)))

(defn get-today-plans [page today-date] (do
  (setv calendar (get-block-by-title page "Ежедневник"))
  (->
    (lfor
      x (calendar.collection.get-rows)
      :if (= x.date.start today-date)
      x.title)
    format-list)))

(defn send-notification [] (do
  (setv client (NotionClient :token-v2 NOTION-TOKEN-V2))
  (setv page (client.get-block GTD-URL))
  (setv today-date (.date (datetime.today)))
  (setv today-plans (get-today-plans page today-date))
  (setv todo-plans (get-todo-plans page))
  (setv message (compose-message today-date todo-plans today-plans))
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

(while
  True
    (do
      (if (= (. (datetime.now) hour) 9) (send-notification))
      (sleep-hour)))
