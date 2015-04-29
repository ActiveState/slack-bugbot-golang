package main

import (
    "github.com/nlopes/slack"
    _ "github.com/go-sql-driver/mysql"
    "database/sql"
    "errors"
    "fmt"
    "log"
    "strings"
)

const openProjectBugUrl = "https://openproject.activestate.com/work_packages/%s"
const bugzillaBugUrl = "https://bugs.activestate.com/show_bug.cgi?id=%s"

func bugMentions(bugNumbers []string, message *slack.MessageEvent) {
    log.Printf("That message mentions these bugs: %s", bugNumbers)
    var messageText string

    for _, match := range bugNumbers {
        if bugNumberWasLinkedRecently(match, message.ChannelId, message.Timestamp) {
            log.Printf("Bug %s was already linked recently", match)
        } else {
            if string(match[0]) == "3" {
                messageText += formatOpenProjectBugMessage(match)
            } else {
                messageText += fmt.Sprintf(bugzillaBugUrl, match)
            }
            messageText += "\n"
        }
    }

    if messageText != "" {
        slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
    }
}

func formatOpenProjectBugMessage(bugNumber string) string {
    var messageText string
    bugTitle, err := fetchOpenProjectBugTitle(bugNumber)
    if err != nil && err.Error() == "This bug doesn't exist!" {
        messageText += fmt.Sprintf("Bug %s doesn't exist!", bugNumber)
    } else if bugTitle == "" {
        messageText += fmt.Sprintf("<%s|%s (Couldn't fetch title)>",
        fmt.Sprintf(openProjectBugUrl, bugNumber), bugNumber)
    } else {
        messageText += fmt.Sprintf("<%s|%s: %s>",
        fmt.Sprintf(openProjectBugUrl, bugNumber), bugNumber, bugTitle)
    }
    return messageText
}

func bugNumberWasLinkedRecently(number string, channelId string, messageTime string) bool {
    historyParameters.Latest = messageTime
    info, _ := slackApi.GetChannelHistory(channelId, historyParameters)
    // Last 10 messages (see historyParameters.Count)
    for _, message := range info.Messages {
        if strings.Contains(message.Text, number) {
            return true
        }
    }
    return false
}

func fetchOpenProjectBugTitle(bugNumber string) (string, error) {
    connectionURL := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?allowOldPasswords=1",
    mysqlConfig.Username, mysqlConfig.Password, mysqlConfig.Host, mysqlConfig.Database)
    log.Printf(connectionURL)
    db, err := sql.Open("mysql", connectionURL)
    if err != nil {
        log.Printf("Mysql database is unavailable! %s", err.Error())
        return "", err
    }
    defer db.Close()

    stmtIns, err := db.Prepare("SELECT subject FROM work_packages WHERE id=?")
    if err != nil {
        log.Printf("MySQL statement preparation failed! %s", err.Error())
        return "", err
    }
    defer stmtIns.Close()

    var bugTitle string
    stmtIns.QueryRow(bugNumber).Scan(&bugTitle)
    if err != nil {
        log.Printf("MySQL statement failed! %s", err.Error())
        return "", err
    }

    if bugTitle == "" {
        return "", errors.New("This bug doesn't exist!")
    }

    log.Printf("#%s: %s", bugNumber, bugTitle)
    return bugTitle, nil
}
