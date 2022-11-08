module.exports = {
  apps: [{
    name: "tg-gtd-bot",
    script: "rwenv -i -e .env go run main.go",
    watch: ".",
  }],
};
