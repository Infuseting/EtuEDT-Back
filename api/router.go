package api

import (
	"EtuEDT-Go/cache"
	"EtuEDT-Go/domain"
	"slices"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func v2Router(router fiber.Router) {
	router.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"univ":  "/v2/univ",
			"rooms": "/v2/rooms",
		})
	})

	router.Get("/univ", func(c *fiber.Ctx) error {
		var universitiesResponse []domain.UniversityResponse
		for i := range domain.AppConfig.Universities {
			university := &domain.AppConfig.Universities[i]
			universitiesResponse = append(universitiesResponse, createUniversityResponse(university))
		}
		return c.JSON(universitiesResponse)
	})

	router.Get("/univ/:id", func(c *fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid parameter"})
		}

		var targetUniversity *domain.UniversityConfig
		for i := range domain.AppConfig.Universities {
			if domain.AppConfig.Universities[i].Id == id {
				targetUniversity = &domain.AppConfig.Universities[i]
				break
			}
		}

		if targetUniversity == nil {
			return c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "university not found"})
		}

		var timetablesResponse []domain.TimetableResponse
		for i := range targetUniversity.Timetables {
			timetable := &targetUniversity.Timetables[i]
			timetablesResponse = append(timetablesResponse, createTimetableResponse(targetUniversity, timetable))
		}
		return c.JSON(timetablesResponse)
	})

	router.Get("/rooms", func(c *fiber.Ctx) error {
		var timetablesResponse []domain.TimetableResponse
		var defaultUniv *domain.UniversityConfig
		if len(domain.AppConfig.Universities) > 0 {
			defaultUniv = &domain.AppConfig.Universities[0]
		}
		for j := range domain.AppConfig.Rooms {
			room := &domain.AppConfig.Rooms[j]
			timetablesResponse = append(timetablesResponse, createRoomResponse(defaultUniv, room))
		}
		return c.JSON(timetablesResponse)
	})

	router.Get("/rooms/:adeResources/:format?", func(c *fiber.Ctx) error {
		adeResources, err := strconv.Atoi(c.Params("adeResources"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid parameter"})
		}

		var targetUniversity *domain.UniversityConfig
		if len(domain.AppConfig.Universities) > 0 {
			targetUniversity = &domain.AppConfig.Universities[0]
		}
		var targetRoom *domain.RoomConfig

		roomIndex := slices.IndexFunc(domain.AppConfig.Rooms, func(r domain.RoomConfig) bool { return r.AdeResources == adeResources })
		if roomIndex >= 0 {
			targetRoom = &domain.AppConfig.Rooms[roomIndex]
		}

		if targetRoom == nil {
			return c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "room not found"})
		}

		return serveRoomConfig(c, targetUniversity, targetRoom)
	})

	router.Get("/:adeResources/:format?", func(c *fiber.Ctx) error {
		adeResources, err := strconv.Atoi(c.Params("adeResources"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid parameter"})
		}

		var targetUniversity *domain.UniversityConfig
		var targetTimetable *domain.TimetableConfig

		for i := range domain.AppConfig.Universities {
			timetableIndex := slices.IndexFunc(domain.AppConfig.Universities[i].Timetables, func(t domain.TimetableConfig) bool { return t.AdeResources == adeResources })
			if timetableIndex >= 0 {
				targetUniversity = &domain.AppConfig.Universities[i]
				targetTimetable = &domain.AppConfig.Universities[i].Timetables[timetableIndex]
				break
			}
		}

		if targetUniversity == nil {
			return c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "timetable not found"})
		}

		return serveTimetable(c, targetUniversity, targetTimetable)
	})
}

func serveRoomConfig(c *fiber.Ctx, university *domain.UniversityConfig, room *domain.RoomConfig) error {
	cache.RecordHit(room.AdeResources)

	timetableCache, ok := cache.GetTimetableByIds(room.AdeResources)
	isStale := !ok || time.Since(timetableCache.LastUpdate).Minutes() > float64(domain.AppConfig.RefreshMinutes)

	if isStale {
		calendar, err := cache.FetchTimetable(*university, room.AdeResources, room.AdeProjectId)
		if err == nil {
			timetableCache = cache.SetTimetableByIds(room.AdeResources, calendar.Serialize(), cache.CalendarToJson(calendar))
		} else if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(domain.ErrorResponse{Error: "could not fetch timetable and no cache available"})
		}
	}

	format := c.Params("format")
	if len(format) == 0 {
		return c.JSON(createRoomResponse(university, room))
	} else if format == "json" {
		return c.JSON(timetableCache.Json)
	} else if format == "ics" {
		c.Set("Content-Type", "text/calendar")
		return c.SendString(timetableCache.Ical)
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{
			Error: "invalid format",
		})
	}
}

func serveTimetable(c *fiber.Ctx, university *domain.UniversityConfig, timetable *domain.TimetableConfig) error {
	cache.RecordHit(timetable.AdeResources)

	timetableCache, ok := cache.GetTimetableByIds(timetable.AdeResources)
	isStale := !ok || time.Since(timetableCache.LastUpdate).Minutes() > float64(domain.AppConfig.RefreshMinutes)

	if isStale {
		calendar, err := cache.FetchTimetable(*university, timetable.AdeResources, timetable.AdeProjectId)
		if err == nil {
			timetableCache = cache.SetTimetableByIds(timetable.AdeResources, calendar.Serialize(), cache.CalendarToJson(calendar))
		} else if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(domain.ErrorResponse{Error: "could not fetch timetable and no cache available"})
		}
		// If fetch fails but we have stale cache, we continue with stale cache
	}

	format := c.Params("format")
	if len(format) == 0 {
		return c.JSON(createTimetableResponse(university, timetable))
	} else if format == "json" {
		return c.JSON(timetableCache.Json)
	} else if format == "ics" {
		c.Set("Content-Type", "text/calendar")
		return c.SendString(timetableCache.Ical)
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{
			Error: "invalid format",
		})
	}
}

func createUniversityResponse(university *domain.UniversityConfig) domain.UniversityResponse {
	return domain.UniversityResponse{
		Id:       university.Id,
		NameUniv: university.NameUniv,
		AdeUniv:  university.AdeUniv,
	}
}

func createTimetableResponse(university *domain.UniversityConfig, timetable *domain.TimetableConfig) domain.TimetableResponse {
	timetableCache, ok := cache.GetTimetableByIds(timetable.AdeResources)
	lastUpdate := time.Time{}
	if ok {
		lastUpdate = timetableCache.LastUpdate
	}

	return domain.TimetableResponse{
		NameUniv:     university.NameUniv,
		DescTT:       timetable.DescTT,
		NumYearTT:    timetable.NumYearTT,
		AdeResources: timetable.AdeResources,
		AdeProjectId: timetable.AdeProjectId,
		LastUpdate:   lastUpdate,
	}
}

func createRoomResponse(university *domain.UniversityConfig, room *domain.RoomConfig) domain.TimetableResponse {
	timetableCache, ok := cache.GetTimetableByIds(room.AdeResources)
	lastUpdate := time.Time{}
	if ok {
		lastUpdate = timetableCache.LastUpdate
	}

	nameUniv := ""
	if university != nil {
		nameUniv = university.NameUniv
	}

	return domain.TimetableResponse{
		NameUniv:     nameUniv,
		DescTT:       room.DescTT,
		NumYearTT:    0,
		AdeResources: room.AdeResources,
		AdeProjectId: room.AdeProjectId,
		LastUpdate:   lastUpdate,
	}
}
