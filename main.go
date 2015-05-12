package main

import (
    "github.com/nlopes/slack"
    "encoding/json"
    "log"
    "os"
    "regexp"
    "strings"
    "time"
)

const botName = "bugbot"
const botSlackId = "U04BTN9D2"
const bugNumberRegex = `(?:[\s(\/]|^)#?([13]\d{5})\b(?:[^-]|$)`

type Config struct {
    SlackKey      string
    MysqlHost     string
    MysqlDatabase string
    MysqlUsername string
    MysqlPassword string
}

var messageParameters = slack.NewPostMessageParameters()
var historyParameters = slack.NewHistoryParameters()
var slackApi *slack.Slack

var config = Config{}

func main() {
    file, _ := os.Open("config.json")
    decoder := json.NewDecoder(file)
    decoder.Decode(&config)

    slackApi = slack.New(config.SlackKey)

    messageParameters.AsUser = true
    messageParameters.EscapeText = false
    historyParameters.Count = 10

    chReceiver := make(chan slack.SlackEvent, 100)
    // Seems like the protocol is optional, and the origin can be any URL
    rtmAPI, err := slackApi.StartRTM("", "http://example.com")
    if err != nil {
        log.Printf("Error starting RTM: %s", err)
    }
    go rtmAPI.HandleIncomingEvents(chReceiver)
    go rtmAPI.Keepalive(20 * time.Second)
    log.Printf("RTM is started")

    bugNbRegex := regexp.MustCompile(bugNumberRegex)

    for {
        event := <-chReceiver
        message, ok := event.Data.(*slack.MessageEvent)
        if ok && message.SubType != "bot_message" && message.UserId != botSlackId { // If this is a MessageEvent
            // Remove stuff in codequotes
            message.Text = regexp.MustCompile("```.*```").ReplaceAllString(message.Text, "")
            // That event doesn't contain the Username, so we can't use message.Username
            log.Printf("Message from %s in channel %s: %s\n", message.UserId, message.ChannelId, message.Text)

            if strings.Contains(message.Text, botName) || strings.Contains(message.Text, botSlackId) {
                go bugbotMention(message)
            } else if matches := bugNbRegex.FindAllStringSubmatch(message.Text, -1); matches != nil {
                // We only care about the first capturing group
                matchesNb := make([]string, len(matches))
                for i, _ := range matches {
                    matchesNb[i] = matches[i][1]
                }
                go bugMentions(matchesNb, message)
            }
        }
    }
}

func inArray(value string, array []string) bool {
    for _, arrayVal := range array {
        if value == arrayVal {
            return true
        }
    }
    return false
}