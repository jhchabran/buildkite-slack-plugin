package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/slack-go/slack"
)

type config struct {
	message              string
	channelName          string
	slackTokenEnvVarName string
	conditions           conditionsConfig
}

type conditionsConfig struct {
	exitCodes []int
	failed    bool
	branches  []string
}

func readPluginEnv(key string) string {
	return os.Getenv(fmt.Sprintf("BUILDKITE_PLUGIN_BUILDKITE_SLACK_GIT_%s", key))
}

func readConfig() *config {
	cfg := config{}
	cfg.message = readPluginEnv("MESSAGE")
	cfg.channelName = readPluginEnv("CHANNEL_NAME")
	cfg.slackTokenEnvVarName = readPluginEnv("SLACK_TOKEN_ENV_VAR_NAME")

	if cfg.slackTokenEnvVarName == "" {
		cfg.slackTokenEnvVarName = "SLACK_TOKEN"
	}

	branches := readPluginEnv("CONDITIONS_BRANCHES")
	cfg.conditions.branches = strings.Split(branches, ",")

	exitCodes := readPluginEnv("CONDITIONS_EXIT_CODES")
	for _, exitCode := range strings.Split(exitCodes, ",") {
		i, err := strconv.Atoi(exitCode)
		if err != nil {
			log.Fatal(err)
		}
		cfg.conditions.exitCodes = append(cfg.conditions.exitCodes, i)
	}

	cfg.conditions.failed = readPluginEnv("CONDITIONS_FAILED") == "true"

	return &cfg
}

func evaluateConditions(buildkiteExitStatus string, buildkiteBranch string, cfg *config) bool {
	// Check if conditions are met

	if len(cfg.conditions.branches) > 0 {
		found := false
		for _, branch := range cfg.conditions.branches {
			if branch == buildkiteBranch {
				found = true
				break
			}
		}
		if !found {
			log.Println("no branch conditions matching")
			return false
		}
	}

	if len(cfg.conditions.exitCodes) > 0 {
		buildkiteExitCode, err := strconv.Atoi(buildkiteExitStatus)
		if err != nil {
			log.Fatal(err)
		}
		found := false
		for _, exitCode := range cfg.conditions.exitCodes {
			if exitCode == buildkiteExitCode {
				found = true
				break
			}
		}
		if !found {
			log.Println("no exit code conditions matching")
			return false
		}
	}

	if cfg.conditions.failed && buildkiteExitStatus == "0" {
		return false
	}

	return true
}

func main() {
	cfg := readConfig()

	slackToken := os.Getenv(cfg.slackTokenEnvVarName)
	if slackToken == "" {
		log.Fatal("Blank Slack token, aborting")
	}

	buildkiteExitStatus := os.Getenv("BUILDKITE_COMMAND_EXIT_STATUS")
	buildkiteBranch := os.Getenv("BUILDKITE_BRANCH")
	wantMessage := evaluateConditions(buildkiteExitStatus, buildkiteBranch, cfg)
	if !wantMessage {
		log.Println("no conditions matching, exiting.")
		os.Exit(0)
	}

	api := slack.New(slackToken)
	allChannels := []slack.Channel{}
	var next string

	// List all channels so we can find the id of the one we're looking for.
	for {
		channels, next, err := api.GetConversations(&slack.GetConversationsParameters{
			Cursor:          next,
			ExcludeArchived: true,
			Types:           []string{"public_channel"},
			Limit:           200, // recommended value
		})

		if err != nil {
			log.Fatal(err)
		}

		allChannels = append(allChannels, channels...)
		if next == "" {
			break
		}
	}

	// Grab the channel ID
	var targetChannelID string
	for _, channel := range allChannels {
		if strings.ToLower(channel.Name) == strings.ToLower(cfg.channelName) {
			targetChannelID = channel.ID
		}
	}

	_, _, err := api.PostMessage(
		targetChannelID,
		slack.MsgOptionText("testing", false),
	)

	if err != nil {
		log.Fatal(err)
	}
}