package api

import (
	"EtuEDT-Go/cache"
	"EtuEDT-Go/domain"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func v2Router(router fiber.Router) {
	router.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"univ": "/v2/univ",
			"room": "/v2/room",
		})
	})

	router.Get("/univ", func(c *fiber.Ctx) error {
		var universitiesResponse []domain.UniversityResponse
		for i, university := range domain.AppConfig.Universities {
			universitiesResponse = append(universitiesResponse, createUniversityResponse(i, &university))
		}
		return c.JSON(universitiesResponse)
	})

	router.Get("/univ/:numUniv", func(c *fiber.Ctx) error {
		numUniv, err := strconv.Atoi(c.Params("numUniv"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid parameter"})
		}

		if numUniv < 0 || numUniv >= len(domain.AppConfig.Universities) {
			return c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "university not found"})
		}

		university := domain.AppConfig.Universities[numUniv]
		var timetablesResponse []domain.TimetableResponse
		for _, timetable := range university.Timetables {
			timetablesResponse = append(timetablesResponse, createTimetableResponse(numUniv, &university, &timetable))
		}
		return c.JSON(timetablesResponse)
	})

	router.Get("/room", func(c *fiber.Ctx) error {
		var timetablesResponse []domain.TimetableResponse
		university := domain.AppConfig.Room
		for _, timetable := range university.Timetables {
			timetablesResponse = append(timetablesResponse, createTimetableResponse(-1, &university, &timetable))
		}
		return c.JSON(timetablesResponse)
	})

	router.Get("/room/:adeResources/:format?", func(c *fiber.Ctx) error {
		university := domain.AppConfig.Room
		timetable, statusCode, err := getTimetableFromParam(c, &university)
		if err != nil {
			return c.Status(statusCode).JSON(domain.ErrorResponse{Error: err.Error()})
		}

		return serveTimetable(c, -1, &university, timetable)
	})

	router.Get("/:adeResources/:format?", func(c *fiber.Ctx) error {
		adeResources, err := strconv.Atoi(c.Params("adeResources"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid parameter"})
		}

		var targetUniversity *domain.UniversityConfig
		var targetTimetable *domain.TimetableConfig
		var targetNumUniv int = -1

		for i, university := range domain.AppConfig.Universities {
			timetableIndex := slices.IndexFunc(university.Timetables, func(t domain.TimetableConfig) bool { return t.AdeResources == adeResources })
			if timetableIndex >= 0 {
				targetUniversity = &university
				targetTimetable = &university.Timetables[timetableIndex]
				targetNumUniv = i
				break
			}
		}

		if targetUniversity == nil {
			return c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "timetable not found"})
		}

		return serveTimetable(c, targetNumUniv, targetUniversity, targetTimetable)
	})
}

func serveTimetable(c *fiber.Ctx, numUniv int, university *domain.UniversityConfig, timetable *domain.TimetableConfig) error {
	cache.RecordHit(timetable.AdeResources)

	timetableCache, ok := cache.GetTimetableByIds(timetable.AdeResources)
	isStale := !ok || time.Since(timetableCache.LastUpdate).Minutes() > float64(domain.AppConfig.RefreshMinutes)

	if isStale {
		calendar, err := cache.FetchTimetable(*university, *timetable)
		if err == nil {
			timetableCache = cache.SetTimetableByIds(timetable.AdeResources, calendar.Serialize(), cache.CalendarToJson(calendar))
		} else if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(domain.ErrorResponse{Error: "could not fetch timetable and no cache available"})
		}
		// If fetch fails but we have stale cache, we continue with stale cache
	}

	format := c.Params("format")
	if len(format) == 0 {
		return c.JSON(createTimetableResponse(numUniv, university, timetable))
	} else if format == "json" {
		return c.JSON(timetableCache.Json)
	} else if format == "ics" {
		c.Set("Content-Type", "text/calendar")
		return c.SendString(timetableCache.Ical)
	} else {
		return c.JSON(domain.ErrorResponse{
			Error: "invalid format",
		})
	}
}

func getTimetableFromParam(c *fiber.Ctx, university *domain.UniversityConfig) (*domain.TimetableConfig, int, error) {
	adeResources, err := strconv.Atoi(c.Params("adeResources"))
	if err != nil {
		return nil, fiber.StatusBadRequest, errors.New("invalid parameter")
	}
	timetableIndex := slices.IndexFunc(university.Timetables, func(c domain.TimetableConfig) bool { return c.AdeResources == adeResources })
	if timetableIndex < 0 {
		return nil, fiber.StatusNotFound, errors.New("timetable not found")
	}
	timetable := university.Timetables[timetableIndex]
	return &timetable, 0, nil
}

func createUniversityResponse(numUniv int, university *domain.UniversityConfig) domain.UniversityResponse {
	return domain.UniversityResponse{
		NumUniv:  numUniv,
		NameUniv: university.NameUniv,
		AdeUniv:  university.AdeUniv,
	}
}

func createTimetableResponse(numUniv int, university *domain.UniversityConfig, timetable *domain.TimetableConfig) domain.TimetableResponse {
	timetableCache, ok := cache.GetTimetableByIds(timetable.AdeResources)
	lastUpdate := time.Time{}
	if ok {
		lastUpdate = timetableCache.LastUpdate
	}

	return domain.TimetableResponse{
		NumUniv:      numUniv,
		NameUniv:     university.NameUniv,
		DescTT:       timetable.DescTT,
		NumYearTT:    timetable.NumYearTT,
		AdeResources: timetable.AdeResources,
		AdeProjectId: timetable.AdeProjectId,
		LastUpdate:   lastUpdate,
	}
}
