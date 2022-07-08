package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mattermost/mattermost-server/v6/model"
)

func uploadRecording(cfg browserConfig, channelID, recPath string) error {
	file, err := os.Open(recPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	client := model.NewAPIv4Client(cfg.siteURL)
	user, _, err := client.Login(cfg.username, cfg.password)
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	defer func() {
		_, err := client.RemoveUserFromChannel(channelID, user.Id)
		if err != nil {
			log.Printf("failed to leave channel: %s", err.Error())
		}
	}()

	us, _, err := client.CreateUpload(&model.UploadSession{
		UserId:    user.Id,
		ChannelId: channelID,
		Filename:  filepath.Base(recPath),
		FileSize:  info.Size(),
	})
	if err != nil {
		return fmt.Errorf("failed to create upload session: %w", err)
	}

	fi, _, err := client.UploadData(us.Id, file)
	if err != nil {
		return fmt.Errorf("failed to upload data: %w", err)
	}

	_, _, err = client.CreatePost(&model.Post{
		UserId:    user.Id,
		ChannelId: channelID,
		FileIds:   []string{fi.Id},
	})
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}
