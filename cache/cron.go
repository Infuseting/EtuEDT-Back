package cache

import (
	"EtuEDT-Go/domain"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	keys := make([]string, 0, len(cache))
	for k := range cache {
		keys = append(keys, k)
	}
	cacheMu.RUnlock()

	for _, adeResourcesStr := range keys {
		adeResources, _ := strconv.Atoi(adeResourcesStr)
		if IsPopular(adeResources) {
			var targetUniv *domain.UniversityConfig
			var targetProjectId int
			found := false

			for i := range domain.AppConfig.Universities {
				university := &domain.AppConfig.Universities[i]
				for j := range university.Timetables {
					tt := &university.Timetables[j]
					if tt.AdeResources == adeResources {
						targetUniv = university
						targetProjectId = tt.AdeProjectId
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				for j := range domain.AppConfig.Rooms {
					room := &domain.AppConfig.Rooms[j]
					if room.AdeResources == adeResources {
						if len(domain.AppConfig.Universities) > 0 {
							targetUniv = &domain.AppConfig.Universities[0]
						}
						targetProjectId = room.AdeProjectId
						found = true
						break
					}
				}
			}

			if found && targetUniv != nil {
				calendar, err := FetchTimetable(*targetUniv, adeResources, targetProjectId)
				if err == nil {
					SetTimetableByIds(adeResources, calendar.Serialize(), CalendarToJson(calendar))
				}
			}
		}
	}
}

func FetchTimetable(university domain.UniversityConfig, adeResources int, adeProjectId int) (*ics.Calendar, error) {
	firstDate := time.Now().AddDate(0, -4, 0).Format("2006-01-02")
	lastDate := time.Now().AddDate(0, 4, 0).Format("2006-01-02")
	req, err := http.NewRequest(http.MethodGet, university.AdeUniv, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("resources", strconv.Itoa(adeResources))
	q.Add("projectId", strconv.Itoa(adeProjectId))
	q.Add("calType", "ical")
	q.Add("firstDate", firstDate)
	q.Add("lastDate", lastDate)
	req.URL.RawQuery = q.Encode()

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
