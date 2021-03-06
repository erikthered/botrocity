package main

import (
	"log"
	"os"
	"time"

	"github.com/hostables/botrocity/modules/eightball"
	"github.com/hostables/botrocity/modules/gygax"

	"github.com/julienschmidt/httprouter"
	"github.com/tylerb/graceful"
	"github.com/urfave/cli"
	"github.com/urfave/negroni"
	"github.com/mattermost/platform/model"
	"strings"
	"regexp"
)

// TODO: Move me somewhere nicer
var client *model.Client
var webSocketClient *model.WebSocketClient
var debuggingChannel *model.Channel
////

func applyRoutes(router *httprouter.Router) {
	router.POST("/outgoing/getEightball", eightball.HandleMagicEightballText)
	router.POST("/outgoing/getRoll", gygax.HandleDiceRollText)
}

func run(ctx *cli.Context) error {
	log.Print("Starting...")
	log.Printf("Using config: %s", ctx.String("config"))
	baseRouter := httprouter.New()


	// FIXME: handle golem being added to channels. it doesn't currently pick those up

	// TODO: move me out of here
	// FIXME: remove hardcoded http here
	client = model.NewClient("http://" + ctx.String("server"))
	// TODO: make sure server is there
	// FIXME: remove hardcoded login, bother to check the error codes
	client.Login("golem@golem.com", "testtest")
	initialLoadResults, _ := client.GetInitialLoad()
	initialLoad := initialLoadResults.Data.(*model.InitialLoad)
	var botTeam *model.Team
	for _, team := range initialLoad.Teams {
		if team.Name == ctx.String("team") {
			botTeam = team
			break
		}
	}
	client.SetTeamId(botTeam.Id)
	channelsResult, _ := client.GetChannels("")
	channelList := channelsResult.Data.(*model.ChannelList)
	for _, channel := range *channelList {

		// FIXME: don't hardcode this
		if channel.Name == "bots" {
			debuggingChannel = channel
			break
		}
	}
	//client.CreatePost(&model.Post{
	//	ChannelId: debuggingChannel.Id,
	//	Message: "Golem online",
	//})

	// FIXME, do stuff about this failing?
	webSocketClient, _ := model.NewWebSocketClient("ws://" + ctx.String("server") + ":8065", client.AuthToken)

	webSocketClient.Listen()

	// FIXME: this looks ghetto
	go func() {
		for {
			select {
			case resp := <-webSocketClient.EventChannel:
				HandleWebSocketResponse(resp)
			}
		}
	}()


	defer func() {
		if webSocketClient != nil {
			webSocketClient.Close()
		}
	}()
	////

	applyRoutes(baseRouter)

	n := negroni.New()
	n.Use(negroni.NewRecovery())
	n.Use(negroni.NewLogger())
	n.UseHandler(baseRouter)
	timeout := 10 * time.Second
	graceful.Run(ctx.String("port"), timeout, n)

	return nil
}

func HandleWebSocketResponse(event *model.WebSocketEvent) {
	// Lets only reponded to messaged posted events
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		log.Println("Event: ", event.Event)
		return
	}

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {
		if matched, _ := regexp.MatchString(`(?:^|\W)golem(?:$|\W)`, post.Message); matched {
			client.CreatePost(&model.Post{
				ChannelId: event.Broadcast.ChannelId,
				Message: "I am here to serve.",
			})
		}
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "botrocity"
	app.Author = "David J Felix <felix.davidj@gmail.com>"
	app.Usage = "Run a bot with fixed responses for mattermost"
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "port, p",
			Value: ":8080",
			Usage: "the host address to run botrocity on.",
		},
		cli.StringFlag{
			Name:  "config, f",
			Value: "botrocity.json",
			Usage: "The config file location.",
		},
		cli.StringFlag{
			Name: "server, s",
			Value: "localhost:8080",
			Usage: "The mattermost server.",
		},
		cli.StringFlag{
			Name: "team, t",
			Value: "bots",
			Usage: "The team the bot should listen on.",
		},
	}
	app.Run(os.Args)
}
