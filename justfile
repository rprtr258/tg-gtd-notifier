# run bot with instant first notification
@debug:
  rwenv -ie .env -o DEBUG=1 go run main.go

# run bot
@run:
  rwenv -ie .env go run main.go
