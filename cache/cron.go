package cache

import (
	"EtuEDT-Go/domain"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/go-co-op/gocron"
)

func StartScheduler() error {
	scheduler := gocron.NewScheduler(time.UTC)
	_, err := scheduler.Every(domain.AppConfig.RefreshMinutes).Minutes().Do(func() {
		timeStart := time.Now()
		log.Println("[Info] [Cron] Refreshing popular timetables")
		refreshPopularTimetables()
		log.Println("[Info] [Cron] Refresh done in " + time.Since(timeStart).String())
	})
	if err != nil {
		return err
	}

	scheduler.StartAsync()

	return nil
}

func refreshPopularTimetables() {
	cacheMu.RLock()
	keys := make([]string, 0, len(cacheMap))
	for k := range cacheMap {
		keys = append(keys, k)
	}
	cacheMu.RUnlock()

	for _, adeResourcesStr := range keys {
		var adeResources int
		fmt.Sscanf(adeResourcesStr, "%d", &adeResources)
		if !IsPopular(adeResources) {
			continue
		}

		// Find the resource in universities (groups or rooms)
		found := false
		for i := range domain.AppConfig.Universities {
			univ := &domain.AppConfig.Universities[i]

			// Search in groups
			for _, group := range univ.Groups {
				for _, tt := range group.Timetables {
					if tt.AdeResources == adeResources {
						fetchAndCache(univ.AdeUrl, adeResources, univ.AdeProjectId)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				break
			}

			// Search in rooms
			for _, room := range univ.Rooms {
				if room.AdeResources == adeResources {
					fetchAndCache(univ.AdeUrl, adeResources, univ.AdeProjectId)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
}

func fetchAndCache(adeBaseUrl string, adeResources int, adeProjectId int) {
	calendar, err := FetchTimetable(adeBaseUrl, adeResources, adeProjectId)
	if err != nil {
		return
	}
	SetTimetableByAdeResources(adeResources, calendar.Serialize(), CalendarToJson(calendar))
}

func FetchTimetable(adeBaseUrl string, adeResources int, adeProjectId int) (*ics.Calendar, error) {
	firstDate, lastDate := domain.GetAcademicYearDates(time.Now())
	fullUrl := domain.BuildAdeUrl(adeBaseUrl, adeResources, adeProjectId, firstDate, lastDate)

	req, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return nil, err
	}

	body, err := MakeRequest(fmt.Sprintf("%d", adeResources), req)
	if err != nil {
		return nil, err
	}

	ical, err := ics.ParseCalendar(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	return ical, nil
}
