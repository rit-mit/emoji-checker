package dcreader

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"

	"github.com/kyoh86/go-docbase/v2/docbase"
	"github.com/slack-go/slack"
)

func Call(message string, channelID string) error {
	dcDomain := os.Getenv("DOCBASE_DOMAIN")
	r := regexp.MustCompile(`https://` + dcDomain + `.docbase.io/posts/([0-9]{7})`)
	fs := r.FindStringSubmatch(message)
	id, err := strconv.Atoi(fs[1])
	if err != nil {
		log.Println(err)
		return err
	}

	fmt.Println(id)
	postID := docbase.PostID(id)
	if err = postDocBaseArticle(postID, channelID); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func postDocBaseArticle(postID docbase.PostID, channelID string) error {
	body, title, err := readDocBaseArticle(postID)
	if err != nil {
		fmt.Println("readDocBaseArticle")
		return err
	}

	api := slack.New(os.Getenv("SLACK_BOT_TOKEN"))

	params := slack.FileUploadParameters{
		Content:  body,
		Title:    title,
		Channels: []string{channelID},
		Filetype: "markdown",
	}
	_, err = api.UploadFileContext(context.Background(), params)
	if err != nil {
		return err
	}

	return nil
}

func readDocBaseArticle(postID docbase.PostID) (string, string, error) {
	client := docbase.NewAuthClient(os.Getenv("DOCBASE_DOMAIN"), os.Getenv("DOCBASE_TOKEN"))
	post, res, err := client.
		Post.
		Get(postID).
		Do(context.Background())
	fmt.Println(res.Response.StatusCode)
	if err != nil {
		fmt.Println("Post.Get error")
		return "", "", err
	}

	return post.Body, post.Title, nil
}
