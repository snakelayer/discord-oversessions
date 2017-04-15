package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/snakelayer/discord-oversessions/owbot"
)

func main() {
	var (
		token         string
		battleTagFile string
		dbFile        string
		debug         bool
	)
	flag.StringVar(&token, "token", "", "The 	secret token for the bot")
	flag.StringVar(&battleTagFile, "battleTags", "", "A file mapping discord userIds to battleTags. One entry per line. Space delimited.")
	flag.StringVar(&dbFile, "dbfile", "", "A path to a file to be used for bolt database")
	flag.BoolVar(&debug, "debug", false, "Set to true to log debug messages")
	flag.Parse()

	// TODO: This is not a great solution for required config...
	if token == "" {
		println("Bot token is required")
		os.Exit(-1)
	}

	logger := logrus.New()
	if debug {
		logger.Level = logrus.DebugLevel
	}

	// reference time: Mon Jan 2 15:04:05 -0700 MST 2006
	logFileName := time.Now().Format("oversessions.log.2006-01-02-15-04-05")
	logFileName = strings.Replace(logFileName, " ", "_", -1)
	logFileName = strings.Replace(logFileName, ":", "", -1)
	logFile, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"module":   "main",
			"filename": logFileName,
			"error":    err,
		}).Fatal("Could not open file for logging")
	}
	defer logFile.Close()
	logger.Formatter = &logrus.TextFormatter{ForceColors: true}
	logger.Out = logFile

	var battleTagMap = getBattleTagMapFromFile(logger, battleTagFile)

	bot, err := owbot.NewBot(logger, token, battleTagMap)
	if err != nil {
		logger.WithFields(logrus.Fields{"module": "main", "error": err}).Error("Could not creating bot")
		return
	}

	if err := bot.Start(); err != nil {
		logger.WithFields(logrus.Fields{"module": "main", "error": err}).Error("Could not start bot")
		return
	}

	// Run until asked to quit
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, os.Kill)
	<-interruptChan

	bot.Stop()
}

func getBattleTagMapFromFile(logger *logrus.Logger, file string) map[string]string {
	var battleTagMap = make(map[string]string)

	if file == "" {
		return battleTagMap
	}

	battleTagData, err := ioutil.ReadFile(file)
	if err != nil {
		logger.WithField("file name", file).Fatal("could not read battleTag file")
	}

	entries := strings.Split(string(battleTagData), "\n")
	for _, entry := range entries {
		if len(entry) <= 0 {
			continue
		}

		pair := strings.Split(entry, " ")
		if len(pair) != 2 {
			logger.WithField("entry", entry).Error("invalid battleTag entry")
			continue
		}

		userId := pair[0]
		battleTag := pair[1]
		battleTagMap[userId] = battleTag
	}

	return battleTagMap
}
