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
	univ := router.Group("/univ")

	// GET /v2/univ — list universities
	univ.Get("/", listUniversities)

	// GET /v2/univ/:univId — university detail
	univ.Get("/:univId", getUniversity)

	// GET /v2/univ/:univId/groups — list groups
	univ.Get("/:univId/groups", listGroups)

	// GET /v2/univ/:univId/groups/:groupId — list timetables in group
	univ.Get("/:univId/groups/:groupId", listTimetables)

	// GET /v2/univ/:univId/groups/:groupId/:adeResources — timetable metadata
	univ.Get("/:univId/groups/:groupId/:adeResources", getTimetableMetadata)

	// GET /v2/univ/:univId/groups/:groupId/:adeResources/events — timetable events
	univ.Get("/:univId/groups/:groupId/:adeResources/events", getTimetableEvents)

	// GET /v2/univ/:univId/rooms — list rooms
	univ.Get("/:univId/rooms", listRooms)

	// GET /v2/univ/:univId/rooms/:adeResources — room metadata
	univ.Get("/:univId/rooms/:adeResources", getRoomMetadata)

	// GET /v2/univ/:univId/rooms/:adeResources/events — room events
	univ.Get("/:univId/rooms/:adeResources/events", getRoomEvents)
}

// --- Handlers ---

func listUniversities(c *fiber.Ctx) error {
	var resp []domain.UniversityResponse
	for _, u := range domain.AppConfig.Universities {
		resp = append(resp, domain.UniversityResponse{
			ID:   u.ID,
			Name: u.Name,
		})
	}
	return c.JSON(resp)
}

func getUniversity(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	return c.JSON(domain.UniversityResponse{
		ID:   univ.ID,
		Name: univ.Name,
	})
}

func listGroups(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	var resp []domain.GroupResponse
	for _, g := range univ.Groups {
		resp = append(resp, domain.GroupResponse{
			ID:   g.ID,
			Name: g.Name,
		})
	}
	return c.JSON(resp)
}

func listTimetables(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	group, err := findGroup(c, univ)
	if err != nil {
		return err
	}
	firstDate, lastDate := domain.GetAcademicYearDates(time.Now())
	var resp []domain.TimetableResponse
	for _, tt := range group.Timetables {
		cached, _ := cache.GetTimetableByAdeResources(tt.AdeResources)
		resp = append(resp, domain.TimetableResponse{
			AdeResources: tt.AdeResources,
			AdeProjectId: univ.AdeProjectId,
			Year:         tt.Year,
			Label:        tt.Label,
			AdeUrl:       domain.BuildAdeUrl(univ.AdeUrl, tt.AdeResources, univ.AdeProjectId, firstDate, lastDate),
			LastUpdate:   cached.LastUpdate,
		})
	}
	return c.JSON(resp)
}

func getTimetableMetadata(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	group, err := findGroup(c, univ)
	if err != nil {
		return err
	}
	tt, err := findTimetable(c, group)
	if err != nil {
		return err
	}
	firstDate, lastDate := domain.GetAcademicYearDates(time.Now())
	cached, _ := cache.GetTimetableByAdeResources(tt.AdeResources)
	return c.JSON(domain.TimetableResponse{
		AdeResources: tt.AdeResources,
		AdeProjectId: univ.AdeProjectId,
		Year:         tt.Year,
		Label:        tt.Label,
		AdeUrl:       domain.BuildAdeUrl(univ.AdeUrl, tt.AdeResources, univ.AdeProjectId, firstDate, lastDate),
		LastUpdate:   cached.LastUpdate,
	})
}

func getTimetableEvents(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	group, err := findGroup(c, univ)
	if err != nil {
		return err
	}
	tt, err := findTimetable(c, group)
	if err != nil {
		return err
	}
	return serveEvents(c, univ, tt.AdeResources)
}

func listRooms(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	firstDate, lastDate := domain.GetAcademicYearDates(time.Now())
	var resp []domain.RoomResponse
	for _, room := range univ.Rooms {
		cached, _ := cache.GetTimetableByAdeResources(room.AdeResources)
		resp = append(resp, domain.RoomResponse{
			AdeResources: room.AdeResources,
			AdeProjectId: univ.AdeProjectId,
			Label:        room.Label,
			AdeUrl:       domain.BuildAdeUrl(univ.AdeUrl, room.AdeResources, univ.AdeProjectId, firstDate, lastDate),
			LastUpdate:   cached.LastUpdate,
		})
	}
	return c.JSON(resp)
}

func getRoomMetadata(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	room, err := findRoom(c, univ)
	if err != nil {
		return err
	}
	firstDate, lastDate := domain.GetAcademicYearDates(time.Now())
	cached, _ := cache.GetTimetableByAdeResources(room.AdeResources)
	return c.JSON(domain.RoomResponse{
		AdeResources: room.AdeResources,
		AdeProjectId: univ.AdeProjectId,
		Label:        room.Label,
		AdeUrl:       domain.BuildAdeUrl(univ.AdeUrl, room.AdeResources, univ.AdeProjectId, firstDate, lastDate),
		LastUpdate:   cached.LastUpdate,
	})
}

func getRoomEvents(c *fiber.Ctx) error {
	univ, err := findUniversity(c)
	if err != nil {
		return err
	}
	room, err := findRoom(c, univ)
	if err != nil {
		return err
	}
	return serveEvents(c, univ, room.AdeResources)
}

// --- Content Negotiation ---

// serveEvents returns cached events in the format requested by the Accept header.
// Defaults to JSON. Use Accept: text/calendar for iCal format.
// If cache is stale or missing, fetches on-demand.
func serveEvents(c *fiber.Ctx, univ *domain.UniversityConfig, adeResources int) error {
	cache.RecordHit(adeResources)

	timetableCache, ok := cache.GetTimetableByAdeResources(adeResources)
	isStale := !ok || time.Since(timetableCache.LastUpdate).Minutes() > float64(domain.AppConfig.RefreshMinutes)

	if isStale {
		calendar, err := cache.FetchTimetable(univ.AdeUrl, adeResources, univ.AdeProjectId)
		if err == nil {
			timetableCache = cache.SetTimetableByAdeResources(adeResources, calendar.Serialize(), cache.CalendarToJson(calendar))
		} else if !ok {
			return c.Status(fiber.StatusServiceUnavailable).JSON(domain.ErrorResponse{
				Error: "could not fetch timetable and no cache available, try again later",
			})
		}
		// If fetch fails but stale cache exists, serve stale cache
	}

	accept := c.Accepts("application/json", "text/calendar")
	switch accept {
	case "text/calendar":
		c.Set("Content-Type", "text/calendar")
		return c.SendString(timetableCache.Ical)
	default:
		return c.JSON(timetableCache.Json)
	}
}

// --- Lookup Helpers ---

func findUniversity(c *fiber.Ctx) (*domain.UniversityConfig, error) {
	univId, err := strconv.Atoi(c.Params("univId"))
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid univId parameter"})
	}
	idx := slices.IndexFunc(domain.AppConfig.Universities, func(u domain.UniversityConfig) bool {
		return u.ID == univId
	})
	if idx < 0 {
		return nil, c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "university not found"})
	}
	return &domain.AppConfig.Universities[idx], nil
}

func findGroup(c *fiber.Ctx, univ *domain.UniversityConfig) (*domain.GroupConfig, error) {
	groupId, err := strconv.Atoi(c.Params("groupId"))
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid groupId parameter"})
	}
	idx := slices.IndexFunc(univ.Groups, func(g domain.GroupConfig) bool {
		return g.ID == groupId
	})
	if idx < 0 {
		return nil, c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "group not found"})
	}
	return &univ.Groups[idx], nil
}

func findTimetable(c *fiber.Ctx, group *domain.GroupConfig) (*domain.TimetableConfig, error) {
	adeResources, err := strconv.Atoi(c.Params("adeResources"))
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid adeResources parameter"})
	}
	idx := slices.IndexFunc(group.Timetables, func(tt domain.TimetableConfig) bool {
		return tt.AdeResources == adeResources
	})
	if idx < 0 {
		return nil, c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "timetable not found"})
	}
	return &group.Timetables[idx], nil
}

func findRoom(c *fiber.Ctx, univ *domain.UniversityConfig) (*domain.RoomConfig, error) {
	adeResources, err := strconv.Atoi(c.Params("adeResources"))
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(domain.ErrorResponse{Error: "invalid adeResources parameter"})
	}
	idx := slices.IndexFunc(univ.Rooms, func(r domain.RoomConfig) bool {
		return r.AdeResources == adeResources
	})
	if idx < 0 {
		return nil, c.Status(fiber.StatusNotFound).JSON(domain.ErrorResponse{Error: "room not found"})
	}
	return &univ.Rooms[idx], nil
}
