package slack

// hector: So this was a server mode in second mode at first, but later we might want an option to run it in either mode at first, just in second mode about server that receives events and we need some filtering, some very simple filtering to only route the relevant events which are messages to a handler that will shape its API
type BotServer struct{}
