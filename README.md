# Yandex CLI Chat
## About
Simple simple REST API client that provide simple chat with YandexGPT models
## How to build?
Requirements: Go 1.22+

To run this app, you need to add oauth token and directory from Yandex Cloud

This should look like this
```
store/dir_id.txt
store/oauth_token.txt
```

More info [HERE](https://yandex.cloud/en/docs/foundation-models/quickstart/yandexgpt) 
```
go build . && ./YandexCLIChat
```