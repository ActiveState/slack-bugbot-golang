package main

import (
    "time"
    "log"
    "fmt"
    "database/sql"
    "strconv"
    "io/ioutil"
    "strings"
)

const bugLimit = 5
const frequency = 5 * time.Second
const slackChannelId = "C03A734HK" // #bugs
const processedNewBugsFile = "processedBugs.txt"

func announceNewBugsLoop() {
    for {
        opBugs, err := fetchRecentOpenProjectBugs()

        if err != nil {
            time.Sleep(30 * time.Second)
            continue
        }

        newBugs := getNewBugs(opBugs)

        for _, newBug := range newBugs {
            log.Printf("New bug created by %s: %s: %s", newBug.CreatedBy, newBug.Number, newBug.Subject)
            postNewBug(newBug)
        }

        time.Sleep(frequency)
    }
}

func fetchRecentOpenProjectBugs() ([]OpenProjectBug, error) {
    var recentBugs []OpenProjectBug

    connectionURL := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?allowOldPasswords=1",
        config.MysqlUsername, config.MysqlPassword, config.MysqlHost, config.MysqlDatabase)
    db, err := sql.Open("mysql", connectionURL)
    if err != nil {
        log.Printf("Mysql database is unavailable! %s", err.Error())
        return recentBugs, err
    }
    defer db.Close()

    sqlStatement := `SELECT work_packages.id, subject, types.name, firstname, lastname
                     FROM work_packages
                     LEFT JOIN types ON work_packages.type_id=types.id
                     LEFT JOIN users ON work_packages.author_id=users.id
                     ORDER BY work_packages.created_at DESC
                     LIMIT ` + strconv.Itoa(bugLimit)
    rows, err := db.Query(sqlStatement)

    for rows.Next() {
        var opBug OpenProjectBug
        var firstName string
        var lastName string
        rows.Scan(&opBug.Number, &opBug.Subject, &opBug.Type, &firstName, &lastName)
        opBug.CreatedBy = firstName + " " + lastName
        recentBugs = append(recentBugs, opBug)
    }

    return recentBugs, nil
}

func getNewBugs(recentBugs []OpenProjectBug) []OpenProjectBug {
    var newBugs []OpenProjectBug
    var newBugsNumbers []string
    fileContents, err := ioutil.ReadFile(processedNewBugsFile)
    processedBugs := strings.Split(string(fileContents), "\n")

    if err != nil {
        // The file doesn't exist, this is our first time running
        // Create the file, but return empty array
        log.Printf("Creating %s file", processedNewBugsFile)
        for _, recentBug := range recentBugs {
            newBugsNumbers = append(newBugsNumbers, recentBug.Number)
        }
        fileContents := strings.Join(newBugsNumbers, "\n")
        ioutil.WriteFile(processedNewBugsFile, []byte(fileContents), 776)
        return newBugs
    }

    for _, recentBug := range recentBugs {
        if inArray(recentBug.Number, processedBugs) {
            // Since recentBugs is ordered, if one is found, all the next ones will be found too
            // so there is no reason to continue
            break
        }
        // If we reach this point, the recent bug is not part of the processed bugs
        newBugs = append([]OpenProjectBug{recentBug}, newBugs...) // Prepend, so that oldest bugs are first
        newBugsNumbers = append([]string{recentBug.Number}, newBugsNumbers...)
    }
    if len(newBugs) > 0 {
        // If there was any change, write the new bugs to the text file
        lastProcessedBugs := append(newBugsNumbers, processedBugs...)[:bugLimit]
        fileContents := strings.Join(lastProcessedBugs, "\n")
        ioutil.WriteFile(processedNewBugsFile, []byte(fileContents), 776)
    }
    return newBugs
}

func postNewBug(opBug OpenProjectBug) {
    messageText := fmt.Sprintf("New work package created by %s:\n<%s|*%s #%s:* %s>",
        opBug.CreatedBy,
        fmt.Sprintf(openProjectBugUrl, opBug.Number),
        opBug.Type, opBug.Number,
        escapeLinkText(opBug.Subject))
    slackApi.PostMessage(slackChannelId, messageText, messageParameters)
}
