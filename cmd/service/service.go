package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mattermost/mattermost-load-test-ng/loadtest/user/websocket"
	"github.com/mattermost/mattermost-server/v6/model"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
)

type config struct {
	siteURL  string
	wsURL    string
	username string
	password string
}

type service struct {
	cfg config

	apiClient *model.Client4
	wsClient  *websocket.Client
}

func newService() *service {
	siteURL := os.Getenv("MM_SITE_URL")
	if siteURL == "" {
		log.Fatalf("invalid siteURL: should not be empty")
	}

	username := os.Getenv("MM_USERNAME")
	if username == "" {
		log.Fatalf("invalid username: should not be empty")
	}

	password := os.Getenv("MM_PASSWORD")
	if password == "" {
		log.Fatalf("invalid password: should not be empty")
	}

	u, err := url.Parse(siteURL)
	if err != nil {
		log.Fatalf(err.Error())
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
		u.Path = "/ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	u.Path = ""
	wsURL := u.String()

	return &service{
		cfg: config{
			siteURL:  siteURL,
			wsURL:    wsURL,
			username: username,
			password: password,
		},
	}
}

func (s *service) startRecording(channelID, teamID string) {
	team, _, err := s.apiClient.GetTeam(teamID, "")
	if err != nil {
		log.Printf("failed to get team: %s", err.Error())
		return
	}

	cli, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		log.Printf("failed to create docker client: %s", err.Error())
		return
	}
	defer cli.Close()

	f := filters.NewArgs()
	f.Add("name", "calls-recorder-"+channelID)
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		log.Printf("failed to run container list: %s", err.Error())
		return
	}

	if len(containers) > 1 {
		log.Printf("unexpected containers length %d", len(containers))
		return
	}

	if len(containers) == 1 {
		log.Printf(containers[0].ID)
		log.Printf(containers[0].State)
		if containers[0].State == "exited" {
			if err := cli.ContainerRemove(context.Background(), containers[0].ID, types.ContainerRemoveOptions{}); err != nil {
				log.Printf("failed to remove container: %s", err.Error())
				return
			}
		} else {
			log.Printf("unexpected containers state %s", containers[0].State)
			return
		}
	}

	env := []string{
		fmt.Sprintf("MM_SITE_URL=%s", s.cfg.siteURL),
		fmt.Sprintf("MM_USERNAME=%s", s.cfg.username),
		fmt.Sprintf("MM_PASSWORD=%s", s.cfg.password),
		fmt.Sprintf("MM_TEAM_NAME=%s", team.Name),
		fmt.Sprintf("MM_CHANNEL_ID=%s", channelID),
	}

	containerName := "calls-recorder-" + channelID
	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image:   "streamer45/calls-recorder",
		Tty:     false,
		Env:     env,
		Volumes: map[string]struct{}{"calls-recorder-volume:/recs": {}},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Target: "/recs",
				Source: "calls-recorder-volume",
				Type:   "volume",
			},
		},
	}, nil, nil, containerName)
	if err != nil {
		log.Printf("failed to create container: %s", err.Error())
		return
	}

	if err := cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Printf("failed to start container: %s", err.Error())
		return
	}

	log.Printf("container successfully started")
}

func (s *service) stopRecording(channelID string) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		log.Printf("failed to create docker client: %s", err.Error())
		return
	}
	defer cli.Close()

	f := filters.NewArgs()
	f.Add("name", "calls-recorder-"+channelID)
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		log.Printf("failed to run container list: %s", err.Error())
		return
	}

	if len(containers) == 0 {
		log.Printf("no container found to stop")
		return
	}

	cnt := containers[0]

	timeout := 300 * time.Second
	if err := cli.ContainerStop(context.Background(), cnt.ID, &timeout); err != nil {
		log.Printf("failed to stop container: %s", err.Error())
		return
	}

	log.Printf("container successfully stopped")
}

func (s *service) eventHandler(reconnectCounter *int) {
	for ev := range s.wsClient.EventChannel {
		switch ev.EventType() {
		case "hello":
			log.Printf("ws connected: %+v", ev.GetData())
			*reconnectCounter = 0
		case "custom_com.mattermost.calls_recording_start", "custom_com.mattermost.calls_recording_stop":
			log.Printf("got stop event")
			log.Printf("%+v", ev.GetData())

			data := ev.GetData()
			if len(data) == 0 {
				log.Printf("invalid ws event data")
				continue
			}

			channelID, ok := data["channelID"].(string)
			if !ok {
				log.Printf("invalid or missing channelID in event data")
				continue
			}

			teamID, ok := data["teamID"].(string)
			if !ok {
				log.Printf("invalid or missing teamID in event data")
				continue
			}

			if strings.HasSuffix(ev.EventType(), "start") {
				s.startRecording(channelID, teamID)
			} else {
				go s.stopRecording(channelID)
			}
		case "custom_com.mattermost.calls_call_end":
			log.Printf("got call end event, will stop the recording")
			channelID := ev.GetBroadcast().ChannelId
			if channelID == "" {
				log.Printf("invalid or missing channelID")
				continue
			}
			go s.stopRecording(channelID)
		default:
			continue
		}
	}
}

func (s *service) Start() {
	client := model.NewAPIv4Client(s.cfg.siteURL)
	_, _, err := client.Login(s.cfg.username, s.cfg.password)
	if err != nil {
		log.Fatalf(err.Error())
	}
	s.apiClient = client

	waitTime := 2 * time.Second
	var reconnectCounter int
	for {
		log.Printf("connecting to websocket: %s", s.cfg.wsURL)

		ws, err := websocket.NewClient4(&websocket.ClientParams{
			WsURL:     s.cfg.wsURL,
			AuthToken: client.AuthToken,
		})
		if err != nil {
			log.Printf("failed to create ws client: %s", err.Error())
		} else {
			s.wsClient = ws
			s.eventHandler(&reconnectCounter)
		}

		if reconnectCounter == 10 {
			log.Printf("max reconnect attempts reached, exiting")
			os.Exit(-1)
		}

		reconnectCounter++
		time.Sleep(waitTime)
	}
}

func main() {
	service := newService()
	service.Start()
}
