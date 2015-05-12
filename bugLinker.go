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
    Number     string
    Subject    string
    Type       string
    Status     string
    Parent     string
    AssignedTo string
    IsClosed   bool
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

func formatOpenProjectBugMessage(opBug OpenProjectBug, err error) string {
    var messageText string
    if err != nil && err.Error() == "This bug doesn't exist!" {
        return fmt.Sprintf("Bug %s doesn't exist!", opBug.Number)
    } else if opBug.Subject == "" {
        messageText += fmt.Sprintf("<%s|*#%s*> (Couldn't fetch info)",
            fmt.Sprintf(openProjectBugUrl, opBug.Number), opBug.Number)
    } else if opBug.IsClosed {
        messageText += fmt.Sprintf("<%s|_%s #%s:_ %s> (Assigned to %s: %s)",
            fmt.Sprintf(openProjectBugUrl, opBug.Number),
            opBug.Type, opBug.Number,
            escapeLinkText(opBug.Subject), opBug.AssignedTo, opBug.Status)
    } else {
        messageText += fmt.Sprintf("<%s|*%s #%s:* %s> (Assigned to %s: %s)",
            fmt.Sprintf(openProjectBugUrl, opBug.Number),
            opBug.Type, opBug.Number,
            escapeLinkText(opBug.Subject), opBug.AssignedTo, opBug.Status)
    }
    return messageText
}

func escapeLinkText(text string) string {
    replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
    return replacer.Replace(text)
}

func bugNumberWasLinkedRecently(number string, channelId string, messageTime string) bool {
    // If this is from a private channel, then ignore history
    if strings.HasPrefix(channelId, "D") {
        return false
    }
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
    var opBug OpenProjectBug
    opBug.Number = bugNumber
    connectionURL := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?allowOldPasswords=1",
        config.MysqlUsername, config.MysqlPassword, config.MysqlHost, config.MysqlDatabase)
    db, err := sql.Open("mysql", connectionURL)
    if err != nil {
        log.Printf("Mysql database is unavailable! %s", err.Error())
        return opBug, err
    }
    defer db.Close()

    sqlStatement := `SELECT subject, types.name, statuses.name, firstname, lastname, parent_id, is_closed
                     FROM work_packages
                     LEFT JOIN types ON work_packages.type_id=types.id
                     LEFT JOIN users ON work_packages.assigned_to_id=users.id
                     LEFT JOIN statuses ON work_packages.status_id=statuses.id
                     WHERE work_packages.id=?`
    stmtIns, err := db.Prepare(sqlStatement)
    if err != nil {
        log.Printf("MySQL statement preparation failed! %s", err.Error())
        return opBug, err
    }
    defer stmtIns.Close()

    var firstName string
    var lastName string
    stmtIns.QueryRow(bugNumber).Scan(
        &opBug.Subject, &opBug.Type, &opBug.Status, &firstName, &lastName,
        &opBug.Parent, &opBug.IsClosed)
    if opBug.Type == "none" {
        opBug.Type = ""
    }
    if firstName != "" && lastName != "" {
        opBug.AssignedTo = firstName + " " + lastName
    } else {
        opBug.AssignedTo = "nobody"
    }
    log.Printf("OP bug: %+v", opBug)

    if err != nil {
        log.Printf("MySQL statement failed! %s", err.Error())
        return opBug, err
    }

    if opBug.Subject == "" {
        return opBug, errors.New("This bug doesn't exist!")
    }

    return opBug, nil
}
