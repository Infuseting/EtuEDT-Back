package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
)

type TimetableConfig struct {
	AdeResources int    `json:"adeResources"`
	Year         int    `json:"year"`
	Label        string `json:"label"`
}

type RoomConfig struct {
	AdeResources int    `json:"adeResources"`
	Label        string `json:"label"`
}

type GroupConfig struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Timetables []TimetableConfig `json:"timetables"`
}

type UniversityConfig struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	AdeUrl       string        `json:"adeUrl"`
	AdeProjectId int           `json:"adeProjectId"`
	Rooms        []RoomConfig  `json:"rooms"`
	Groups       []GroupConfig `json:"groups"`
}

type Config struct {
	RefreshMinutes int                `json:"refreshMinutes"`
	Universities   []UniversityConfig `json:"univs"`
}

var AppConfig Config

func LoadConfig() error {
	file, err := os.Open("./config.json")
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&AppConfig); err != nil {
		return err
	}

	return validateConfig(&AppConfig)
}

func validateConfig(config *Config) error {
	if config.RefreshMinutes < 1 {
		return errors.New("refreshMinutes must be greater than 0")
	}

	univIDs := make(map[int]bool)
	groupIDs := make(map[int]bool)
	adeResourcesSet := make(map[int]bool)

	for _, univ := range config.Universities {
		if univIDs[univ.ID] {
			return fmt.Errorf("duplicate university id: %d", univ.ID)
		}
		univIDs[univ.ID] = true

		for _, room := range univ.Rooms {
			if adeResourcesSet[room.AdeResources] {
				return fmt.Errorf("duplicate adeResources: %d", room.AdeResources)
			}
			adeResourcesSet[room.AdeResources] = true
		}

		for _, group := range univ.Groups {
			if groupIDs[group.ID] {
				return fmt.Errorf("duplicate group id: %d in university %d", group.ID, univ.ID)
			}
			groupIDs[group.ID] = true

			for _, tt := range group.Timetables {
				if adeResourcesSet[tt.AdeResources] {
					return fmt.Errorf("duplicate adeResources: %d", tt.AdeResources)
				}
				adeResourcesSet[tt.AdeResources] = true
			}
		}
	}

	return nil
}
