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
	for adeResourcesStr, _ := range cache {
		adeResources, _ := strconv.Atoi(adeResourcesStr)
		if IsPopular(adeResources) {
			var targetUniv *domain.UniversityConfig
			var targetTT *domain.TimetableConfig

			for _, tt := range domain.AppConfig.Room.Timetables {
				if tt.AdeResources == adeResources {
					targetUniv = &domain.AppConfig.Room
					targetTT = &tt
					break
				}
			}

			if targetUniv == nil {
				for _, univ := range domain.AppConfig.Universities {
					for _, tt := range univ.Timetables {
						if tt.AdeResources == adeResources {
							targetUniv = &univ
							targetTT = &tt
							break
						}
					}
					if targetUniv != nil {
						break
					}
				}
			}

			if targetUniv != nil && targetTT != nil {
				calendar, err := FetchTimetable(*targetUniv, *targetTT)
				if err == nil {
					SetTimetableByIds(targetTT.AdeResources, calendar.Serialize(), CalendarToJson(calendar))
				}
			}
		}
	}
}

func FetchTimetable(university domain.UniversityConfig, timetable domain.TimetableConfig) (*ics.Calendar, error) {
	firstDate := time.Now().AddDate(0, -4, 0).Format("2006-01-02")
	lastDate := time.Now().AddDate(0, 4, 0).Format("2006-01-02")
	req, err := http.NewRequest(http.MethodGet, university.AdeUniv, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("resources", strconv.Itoa(timetable.AdeResources))
	q.Add("projectId", strconv.Itoa(timetable.AdeProjectId))
	q.Add("calType", "ical")
	q.Add("firstDate", firstDate)
	q.Add("lastDate", lastDate)
	req.URL.RawQuery = q.Encode()

	body, err := MakeRequest(fmt.Sprintf("%d", timetable.AdeResources), req)
	if err != nil {
		return nil, err
	}

	ical, err := ics.ParseCalendar(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	return ical, nil
}
