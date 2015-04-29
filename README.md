# slack-bugbot-golang

This Slack bot has two functions:

* When somebody posts a bug number on Slack, bugbot will fetch the bug title and link to it as a reply.
* If somebody types `@bugbot unmerged`, the unmerged-bugs.sh script will run, and bugbot will post the list of unerged bugs.

Bugbot needs to be in a channel to see its messages! Summon him by posting `@bugbot:`
