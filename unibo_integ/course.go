package unibo_integ

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/csunibo/unibo-go/curriculum"
	"github.com/csunibo/unibo-go/timetable"
	"github.com/rs/zerolog/log"

	"github.com/patrickmn/go-cache"
)

type Course struct {
	AnnoAccademico       string
	Immatricolabile      string
	Codice               int
	Descrizione          string
	Url                  string
	Campus               string
	Ambiti               string
	Tipologia            string
	DurataAnni           int
	Internazionale       bool
	InternazionaleTitolo string
	InternazionaleLingua string
	Lingue               string
	Accesso              string
	SedeDidattica        string
}

type CourseId struct {
	Tipologia string
	Id        string
}

var websiteIdCache = cache.New(cache.NoExpiration, cache.NoExpiration)

// GetCourseWebsiteId returns the [CourseWebsiteId] of the course.
//
// If the course website id is already set, it returns it,
// otherwise it scrapes it from the course website.
func (c Course) GetCourseWebsiteId() (CourseId, error) {
	codeStr := strconv.Itoa(c.Codice)

	// If the course website id is already in the cache, return it
	websiteIdAny, found := websiteIdCache.Get(codeStr)
	if found {
		return websiteIdAny.(CourseId), nil
	}

	// Scrape the course website id and set it
	websiteId, err := c.scrapeCourseWebsiteId()
	if err != nil {
		return CourseId{}, err
	}

	websiteIdCache.Set(codeStr, websiteId, cache.DefaultExpiration)
	return websiteId, nil
}

var reg = regexp.MustCompile(`<a .* href="https://corsi\.unibo\.it/(.+?)"`)

func (c Course) scrapeCourseWebsiteId() (CourseId, error) {

	resp, err := Client.Get(c.Url)
	if err != nil {
		return CourseId{}, fmt.Errorf("unable to get course website: %w", err)
	}

	log.Debug().Str("url", c.Url).Msg("scraping course website")

	buf := new(bytes.Buffer)

	// Read all body
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return CourseId{}, fmt.Errorf("unable to read course website: %w", err)
	}

	// Close body
	err = resp.Body.Close()
	if err != nil {
		return CourseId{}, fmt.Errorf("unable to close course website: %w", err)
	}

	// Convert body to string
	found := reg.FindStringSubmatch(buf.String())
	if found == nil {
		return CourseId{}, fmt.Errorf("unable to find course website")
	} else if len(found) != 2 {
		return CourseId{}, fmt.Errorf("unexpected number of matches: %d (the website has changed?)", len(found))
	}

	// full url -> laurea/IngegneriaInformatica
	id := found[1]

	// laurea/IngegneriaInformatica -> IngegneriaInformatica
	split := strings.Split(id, "/")
	if len(split) != 2 {
		return CourseId{}, fmt.Errorf("unexpected number of splits: %d (the website has changed?)", len(split))
	}

	return CourseId{split[0], split[1]}, nil
}

func (c Course) GetCurricula(year int) (curriculum.Curricula, error) {
	id, err := c.GetCourseWebsiteId()
	if err != nil {
		return nil, err
	}

	curricula, err := curriculum.FetchCurricula(id.Tipologia, id.Id, year)
	if err != nil {
		return nil, err
	}

	return curricula, nil
}

func (c Course) GetAllCurricula() (map[int]curriculum.Curricula, error) {
	id, err := c.GetCourseWebsiteId()
	if err != nil {
		return nil, fmt.Errorf("could not get course website id: %w", err)
	}

	errCh := make(chan error, c.DurataAnni)
	var wg sync.WaitGroup

	var mapMutex sync.Mutex
	curriculaMap := make(map[int]curriculum.Curricula, c.DurataAnni)

	for year := 1; year <= c.DurataAnni; year++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			curricula, err := curriculum.FetchCurricula(id.Tipologia, id.Id, year)
			if err != nil {
				errCh <- err
			} else {
				mapMutex.Lock()
				curriculaMap[year] = curricula
				mapMutex.Unlock()
			}
		}()
	}

	wg.Wait()
	select {
	case e := <-errCh:
		close(errCh)
		return nil, e
	default:
		return curriculaMap, nil
	}
}

func (c Course) GetTimetable(year int, curriculum curriculum.Curriculum, period *timetable.Interval) (timetable.Timetable, error) {
	id, err := c.GetCourseWebsiteId()
	if err != nil {
		return nil, err
	}

	t, err := timetable.FetchTimetable(id.Tipologia, id.Id, curriculum.Value, year, period)
	if err != nil {
		return nil, err
	}

	return t, nil
}

type CoursesMap map[int]Course

func (c CoursesMap) ToList() []Course {
	courses := make([]Course, 0, len(c))
	for _, course := range c {
		courses = append(courses, course)
	}
	return courses
}

func (c CoursesMap) FindById(id int) (*Course, bool) {
	course, found := c[id]
	return &course, found
}
