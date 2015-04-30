package main

import (
    "github.com/nlopes/slack"
    "fmt"
    "log"
    "os/exec"
    "regexp"
    "sort"
    "strings"
)

func bugbotMention(message *slack.MessageEvent) {
    log.Printf("That message mentions bugbot")
    // Unmerged bugs
    matched, _ := regexp.MatchString(`^(?:[@/]?bugbot|<@U04BTN9D2>) unmerged`, message.Text)
    if matched {
        printUnmergedBugNumbers(message)
    }

    // Thanks
    matched, _ = regexp.MatchString(`[Tt]hanks`, message.Text)
    if matched {
        messageText := "You're welcome! :catbug:"
        slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
    }
}

func printUnmergedBugNumbers(message *slack.MessageEvent) {
    _, timestamp, _ := slackApi.PostMessage(message.ChannelId, "Working on it... :catbug:", messageParameters)
    lines, err := getUnMergedBugNumbers()

    if err != nil {
        // Cannot use UpdateMessage reliably, doesn't work if we try to update the message before it appears
        slackApi.DeleteMessage(message.ChannelId, timestamp)
        messageText := fmt.Sprintf("Oh no! Something went wrong with the unmerged bugs script!\n`%s`", err)
        slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
        return
    }

    filterBugs := []string{}
    filterCommand := regexp.MustCompile(`filter(?: 3\d{5})+`).FindString(message.Text)
    if len(filterCommand) > 0 {
        filterBugs = regexp.MustCompile(`3\d{5}`).FindAllString(filterCommand, -1)
    }
    filterMessage := ""
    if len(filterBugs) > 0 {
        bugGrammar := "bug"
        if len(filterBugs) > 1 {
            bugGrammar = "bugs"
        }
        filterMessage = fmt.Sprintf("(Without %s %v and children)", bugGrammar, filterBugs)
        filterMessage = strings.Replace(filterMessage, "[", "", 1)
        filterMessage = strings.Replace(filterMessage, "]", "", 1)
    }
    messageText := fmt.Sprintf("*Issues that are unmerged to master:* %s\n", filterMessage)
    log.Printf("Bugs to filter: %s", filterBugs)
    totalBugs := 0
    for _, bugNumber := range lines {
        if inArray(bugNumber, filterBugs) {
            continue
        }
        opBug, opErr := fetchOpenProjectBugInfo(bugNumber)
        if inArray(opBug.Parent, filterBugs) {
            continue
        }
        messageText += formatOpenProjectBugMessage(opBug, opErr)
        messageText += "\n"
        totalBugs++
    }
    messageText += fmt.Sprintf("*Total bugs: %v*", totalBugs)

    // Cannot use UpdateMessage since that doesn't support formatted links
    slackApi.DeleteMessage(message.ChannelId, timestamp)
    slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
}

func getUnMergedBugNumbers() ([]string, error) {
    log.Printf("Call for unmerged bug check")
    out, err := exec.Command("sh", "unmerged-bugs.sh").Output()
    if err != nil {
        log.Printf("Unmerged bug script failed: %s - Output: %s", err, out)
        if len(out) > 0 {
            return nil, fmt.Errorf("%s - Output: %s", err, out)
        } else {
            return nil, err
        }
    }
    lines := strings.Split(string(out), "\n")
    log.Printf("Unmerged bugs before duplicates: %s", lines)
    // Remove duplicates and non-bugs
    var result sort.StringSlice = []string{}
    seen := map[string]string{}
    for _, line := range lines {
        _, ok := seen[line]
        if !ok && len(line) > 0 && string(line[0]) == "3" {
            result = append(result, line)
            seen[line] = line
        }
    }
    result.Sort()
    log.Printf("Unmerged bugs: %s", result)
    return result, nil
}
