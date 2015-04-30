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

type OpenProjectBug struct {
    Number  string
    Subject string
    Type    string
    Parent  string
}

func bugMentions(bugNumbers []string, message *slack.MessageEvent) {
    log.Printf("That message mentions these bugs: %s", bugNumbers)
    var messageText string

    for _, match := range bugNumbers {
        if bugNumberWasLinkedRecently(match, message.ChannelId, message.Timestamp) {
            log.Printf("Bug %s was already linked recently", match)
        } else {
            if string(match[0]) == "3" {
                messageText += formatOpenProjectBugMessage(fetchOpenProjectBugInfo(match))
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

func formatOpenProjectBugMessage(openProjectBug OpenProjectBug, err error) string {
    var messageText string
    if err != nil && err.Error() == "This bug doesn't exist!" {
        messageText += fmt.Sprintf("Bug %s doesn't exist!", openProjectBug.Number)
    } else if openProjectBug.Subject == "" {
        messageText += fmt.Sprintf("<%s|%s (Couldn't fetch title)>",
        fmt.Sprintf(openProjectBugUrl, openProjectBug.Number), openProjectBug.Number)
    } else {
        messageText += fmt.Sprintf("<%s|*%s #%s:* %s>",
        fmt.Sprintf(openProjectBugUrl, openProjectBug.Number),
        openProjectBug.Type, openProjectBug.Number, openProjectBug.Subject)
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

func fetchOpenProjectBugInfo(bugNumber string) (OpenProjectBug, error) {
    var openProjectBug OpenProjectBug
    openProjectBug.Number = bugNumber
    connectionURL := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?allowOldPasswords=1",
    config.MysqlUsername, config.MysqlPassword, config.MysqlHost, config.MysqlDatabase)
    db, err := sql.Open("mysql", connectionURL)
    if err != nil {
        log.Printf("Mysql database is unavailable! %s", err.Error())
        return openProjectBug, err
    }
    defer db.Close()

    sqlStatement := `SELECT subject,types.name,parent_id FROM work_packages
                     LEFT JOIN types ON work_packages.type_id=types.id
                     WHERE work_packages.id=?`
    stmtIns, err := db.Prepare(sqlStatement)
    if err != nil {
        log.Printf("MySQL statement preparation failed! %s", err.Error())
        return openProjectBug, err
    }
    defer stmtIns.Close()

    stmtIns.QueryRow(bugNumber).Scan(&openProjectBug.Subject, &openProjectBug.Type, &openProjectBug.Parent)
    if openProjectBug.Type == "none" {
        openProjectBug.Type = ""
    }
    log.Printf("OP bug: %s, %s, %s", openProjectBug.Subject, openProjectBug.Type, openProjectBug.Parent)

    if err != nil {
        log.Printf("MySQL statement failed! %s", err.Error())
        return openProjectBug, err
    }

    if openProjectBug.Subject == "" {
        return openProjectBug, errors.New("This bug doesn't exist!")
    }

    log.Printf("#%s: %s", bugNumber, openProjectBug)
    return openProjectBug, nil
}
