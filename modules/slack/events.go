package slack

import "github.com/iamhectordev/hector/pkg/waffle"

// SlackMessageRecieved is the event definition for an incoming Slack message
var SlackMessageRecieved = waffle.Define[string]("slack.message_recieved", 1)
