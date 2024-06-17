package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	role2       = "user"
	stream      = true
	temperature = 0.3
	maxTokens   = "500"
	role1       = "system"
	system_text = "Ты - умный помощник"
)

type api_request struct {
	ModeUri           string `json:"modelUri"`
	CompletionOptions struct {
		Stream      bool    `json:"stream"`
		Temperature float32 `json:"temperature"`
		MaxTokens   string  `json:"maxTokens"`
	} `json:"completionOptions"`
	Messages []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"messages"`
}

type gpt_response struct {
	Result struct {
		Alternatives []struct {
			Message struct {
				Role string `json:"role"`
				Text string `json:"text"`
			}
			Status string `json:"status"`
		}
		Usage struct {
			InputTextTokens  string `json:"inputTextTokens"`
			CompletionTokens string `json:"completionTokens"`
			TotalTokens      string `json:"totalTokens"`
		}
		ModelVersion string `json:"modelVersion"`
	}
}

type token_response struct {
	IamToken  string
	ExpiresAt string
}

type OauthToken struct {
	Oauth string `json:"yandexPassportOauthToken"`
}

// api request to gain new iam token
func request_iamtoken() string {
	oauth_token, err := os.ReadFile("store/oauth_token.txt")
	if err != nil {
		log.Fatal(err)
	}

	client := http.Client{}
	url := "https://iam.api.cloud.yandex.net/iam/v1/tokens"
	json_body := OauthToken{string(oauth_token)}

	json_body_marshd, err := json.Marshal(json_body)
	if err != nil {
		log.Fatal(err)
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(json_body_marshd))
	if err != nil {
		log.Fatal(err)
	}

	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Fatal(response.Status)
	}
	var api_response token_response

	error := json.NewDecoder(response.Body).Decode(&api_response)
	if error != nil {
		log.Fatal(error)
	}

	body, err := json.Marshal(api_response)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("store/iam.json", body, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return api_response.IamToken
}

func get_iam_token() string {
	tokenExists := false
	var iam_token string
	if _, err := os.Stat("store/iam.json"); err == nil {
		tokenExists = true
	}
	if tokenExists {
		data, err := os.ReadFile("store/iam.json")
		if err != nil {
			log.Fatal(err)
		}

		var iamtoken_json token_response
		err = json.Unmarshal(data, &iamtoken_json)
		if err != nil {
			log.Fatal(err)
		}

		date := iamtoken_json.ExpiresAt
		layout := time.RFC3339

		expireDate, err := time.Parse(layout, date)
		if err != nil {
			log.Fatal(err)
		}

		dateNow := time.Now()

		if expireDate.Before(dateNow) {
			iam_token = request_iamtoken()
		} else {
			iam_token = iamtoken_json.IamToken
		}

	} else {
		iam_token = request_iamtoken()
	}
	return iam_token
}

// directory id from Yandex Cloud
func getdir_ID() string {
	dir_id, err := os.ReadFile("store/dir_id.txt")
	if err != nil {
		log.Fatal(err)
	}
	return string(dir_id)
}

func messageDecode(response *http.Response) {
	defer response.Body.Close()
	fmt.Print(">>> ")

	decoder := json.NewDecoder(response.Body)
	bufStr := ""

	for {
		var json_response gpt_response

		if err := decoder.Decode(&json_response); err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Failed to decode JSON: %v", err)
		}

		messageText := json_response.Result.Alternatives[0].Message.Text
		renderedTokens := strings.ReplaceAll(messageText, bufStr, "")
		bufStr += renderedTokens
		fmt.Print(renderedTokens)
	}

	fmt.Println()
	//}
	// err := json.NewDecoder(response.Body).Decode(&json_response)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	//message := json_response.Result.Alternatives[len(json_response.Result.Alternatives)-1].Message.Text

}

func getUserInput() string {
	fmt.Print(">>> ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	err := scanner.Err()
	if err != nil {
		log.Fatal(err)
	}
	return scanner.Text()
}

func createJsonRequestBody(user_input string) []byte {
	dir_ID := getdir_ID()
	user_request := api_request{
		ModeUri: fmt.Sprintf("gpt://%v/yandexgpt/latest", dir_ID),
		CompletionOptions: struct {
			Stream      bool    `json:"stream"`
			Temperature float32 `json:"temperature"`
			MaxTokens   string  `json:"maxTokens"`
		}{
			Stream:      stream,
			Temperature: temperature,
			MaxTokens:   maxTokens,
		},
		Messages: []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		}{
			{
				Role: role1,
				Text: system_text,
			},
			{
				Role: role2,
				Text: user_input,
			},
		},
	}

	json_request, err := json.Marshal(user_request)
	if err != nil {
		log.Fatal(err)
	}

	return json_request
}

func postRequest(user_input string) *http.Response {
	client := http.Client{}
	dir_ID := getdir_ID()
	url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	user_request := createJsonRequestBody(user_input)
	iam_token := get_iam_token()

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(user_request))
	if err != nil {
		log.Fatal(err)
	}

	request.Header = http.Header{
		"Content-Type":  {"application/json"},
		"Authorization": {fmt.Sprintf("Bearer %v", iam_token)},
		"x-folder-id":   {dir_ID},
	}

	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		log.Fatal(response.StatusCode)
	}

	return response
}

func main() {
	fmt.Println("Enter your text")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			user_input := getUserInput()
			gptRes := postRequest(user_input)
			messageDecode(gptRes)
		}
	}()
	<-sig
	fmt.Print("Exit program")
	os.Exit(0)
}
