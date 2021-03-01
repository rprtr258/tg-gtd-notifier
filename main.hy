(import
  sys
  [datetime [datetime date]]
  [json [dumps loads]]
  [configparser [ConfigParser]]
  [random [sample]])
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


(setv client (NotionClient :token-v2 NOTION-TOKEN-V2))
(setv page (client.get-block GTD-URL))

(defn get-block-by-title [title]
  (next
    (filter
      (fn [x] (= x.title title))
      page.children)))
(setv todos-list (get-block-by-title "Следующие конкретные действия"))
(setv calendar (get-block-by-title "Ежедневник"))

(setv today-date (.date (datetime.today)))

(defn tag [tag-name text] f"<{tag-name}>{text}</{tag-name}>")

(defn format-list [xs] (->>
  xs
  (map (fn [x] f"- {x}"))
  list))

(setv today-plans (->
  (lfor
    x (calendar.collection.get-rows)
    :if (= x.date.start today-date)
    x.title)
  format-list))

(defn then-or-empty-string [x y] (if x y ""))

(defn get-todo-icon [todo-item] (do
  (setv projects (todo-item.get-property "project"))
  (then-or-empty-string
    (and projects (> (len projects) 0))
    (do
      (setv project (get projects 0))
      (setv icon (. project icon))
      (then-or-empty-string icon icon)))))

(setv todo-plans (->>
  (todos-list.collection.get-rows)
  list
  (sample :k 3)
  (map (fn [x] f"{(get-todo-icon x)}{x.title}"))
  format-list))

(setv message ((. "\n" join)
  (+
    [(tag "b" f"📆 Сегодня {today-date}")]
    [""]
    [(tag "i" "✨ Что еще можно сделать:")]
    todo-plans
    [""]
    [(tag "i" "🌟 Планы на сегодня:")]
    today-plans)))


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
  (print "Sent succesfuly"))
