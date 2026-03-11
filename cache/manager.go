package cache

import (
	"EtuEDT-Go/domain"
	"strconv"
	"sync"
	"time"
)

type TimetableCache struct {
	AdeResources      int                `json:"adeResources"`
	LastUpdate        time.Time          `json:"lastUpdate"`
	Ical              string             `json:"calendar"`
	Json              []domain.JsonEvent `json:"json"`
	RequestTimestamps []time.Time        `json:"requestTimestamps"`
}

var cache = make(map[string]TimetableCache)
var cacheMu sync.RWMutex

func GetTimetableByIds(adeResources int) (TimetableCache, bool) {
	key := getKey(adeResources)
	cacheMu.RLock()
	timetable, ok := cache[key]
	cacheMu.RUnlock()
	return timetable, ok
}

func SetTimetableByIds(adeResources int, ical string, json []domain.JsonEvent) TimetableCache {
	key := getKey(adeResources)
	cacheMu.Lock()
	timetable, ok := cache[key]
	if ok {
		timetable.LastUpdate = time.Now()
		timetable.Ical = ical
		timetable.Json = json
	} else {
		timetable = TimetableCache{
			AdeResources:      adeResources,
			LastUpdate:        time.Now(),
			Ical:              ical,
			Json:              json,
			RequestTimestamps: []time.Time{},
		}
	}
	cache[key] = timetable
	cacheMu.Unlock()
	return timetable
}

func RecordHit(adeResources int) {
	key := getKey(adeResources)
	cacheMu.Lock()
	timetable, ok := cache[key]
	if !ok {
		timetable = TimetableCache{
			AdeResources:      adeResources,
			RequestTimestamps: []time.Time{},
		}
	}

	timetable.RequestTimestamps = append(timetable.RequestTimestamps, time.Now())
	// Cleanup old timestamps (older than 7 days)
	var recentTimestamps []time.Time
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	for _, t := range timetable.RequestTimestamps {
		if t.After(sevenDaysAgo) {
			recentTimestamps = append(recentTimestamps, t)
		}
	}
	timetable.RequestTimestamps = recentTimestamps
	cache[key] = timetable
	cacheMu.Unlock()
}

func IsPopular(adeResources int) bool {
	key := getKey(adeResources)
	cacheMu.RLock()
	timetable, ok := cache[key]
	cacheMu.RUnlock()
	if !ok {
		return false
	}

	count := 0
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	for _, t := range timetable.RequestTimestamps {
		if t.After(sevenDaysAgo) {
			count++
		}
	}

	return count > 5
}

func getKey(adeResources int) string {
	return strconv.Itoa(adeResources)
}
