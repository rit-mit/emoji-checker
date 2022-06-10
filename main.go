package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

type (
	Request  = events.APIGatewayProxyRequest
	Response = events.APIGatewayProxyResponse
)

func ConvertHeaders(headers map[string]string) http.Header {
	h := http.Header{}
	for key, value := range headers {
		h.Set(key, value)
	}
	return h
}

// Note: Lambdaでの処理用
func lambdaHandler(ctx context.Context, request Request) (Response, error) {
	if _, ok := lambdacontext.FromContext(ctx); !ok {
		response := Response{
			StatusCode: http.StatusInternalServerError,
			Body:       "not invoked from aws lambda",
			// Headers:    map[string]string{},
		}
		return response, errors.New("verify error")
	} else {
		body := []byte(request.Body)
		header := ConvertHeaders(request.Headers)

		response, err := slackHandler(header, body)
		return response, err
	}
}

// Note: ローカルでの確認用
func httpHandler() {
	http.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response, err := slackHandler(r.Header, body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(response.StatusCode)
		if _, err := w.Write([]byte(response.Body)); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	log.Println("[INFO] Server listening")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}

// ToDo: 各処理を分割する
func slackHandler(header http.Header, body []byte) (Response, error) {
	response := Response{
		StatusCode: http.StatusOK,
		Body:       "",
		Headers:    map[string]string{},
	}

	////////// Verify
	verifier, err := slack.NewSecretsVerifier(header, os.Getenv("SLACK_SIGNING_SECRET"))
	if err != nil {
		log.Println(err)
		response.StatusCode = http.StatusInternalServerError
		return response, errors.New("verify error")
	}

	// log.Println(body)
	if _, err := verifier.Write(body); err != nil {
		response.StatusCode = http.StatusInternalServerError
		return response, errors.New("bad request")
	}

	if err := verifier.Ensure(); err != nil {
		log.Println(err)
		response.StatusCode = http.StatusBadRequest
		return response, errors.New("bad request")
	}

	////////// end Verify

	api := slack.New(os.Getenv("SLACK_BOT_TOKEN"))

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Println(err)
		response.StatusCode = http.StatusInternalServerError
		return response, errors.New("internal server error")
	}

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		var res *slackevents.ChallengeResponse

		if err := json.Unmarshal([]byte(body), &res); err != nil {
			log.Println(err)
			response.StatusCode = http.StatusInternalServerError
			return response, errors.New("internal server error")
		}
		response.Headers["Content-Type"] = "text/plain"
		response.Body = res.Challenge

	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		switch event := innerEvent.Data.(type) {
		// Note: アプリにメンションする
		case *slackevents.AppMentionEvent:
			message := strings.Split(event.Text, " ")
			if len(message) < 2 {
				response.StatusCode = http.StatusBadRequest
				return response, errors.New("bad request")
			}

			command := message[1]
			log.Println(command)
			switch command {
			// Note: pingとメンションすとpongを答える
			case "ping":
				if _, _, err := api.PostMessage(event.Channel, slack.MsgOptionText("pong", false)); err != nil {
					log.Println(err)
					response.StatusCode = http.StatusInternalServerError
					return response, errors.New("bad request")
				}
			}
		// Note: 絵文字の変更
		case *slackevents.EmojiChangedEvent:
			channelId := os.Getenv("NOTIFY_CHANNEL")

			name := event.Name
			message := name + " :" + name + ": が追加されました！"

			if _, _, err := api.PostMessage(channelId, slack.MsgOptionText(message, false)); err != nil {
				log.Println(err)
				response.StatusCode = http.StatusInternalServerError
				return response, errors.New("bad request")
			}
		// Note: チャンネルの追加
		case *slackevents.ChannelCreatedEvent:
			channelId := os.Getenv("NOTIFY_CHANNEL")

			newChannelId := event.Channel.ID

			message := "新しいチャンネル <" + newChannelId + "> が追加されました！"
			if _, _, err := api.PostMessage(channelId, slack.MsgOptionText(message, false)); err != nil {
				log.Println(err)
				response.StatusCode = http.StatusInternalServerError
				return response, errors.New("bad request")
			}
		}
	}

	return response, nil
}

func main() {
	if os.Getenv("_LAMBDA_SERVER_PORT") != "" {
		lambda.Start(lambdaHandler)
	} else {
		httpHandler()
	}
}
