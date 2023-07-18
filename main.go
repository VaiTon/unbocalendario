package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"text/template"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/gin-contrib/multitemplate"
	limits "github.com/gin-contrib/size"
	"github.com/gin-gonic/gin"
	"github.com/lf4096/gin-compress"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/VaiTon/unibocalendar/unibo"
)

var (
	//go:embed templates/base.gohtml
	baseTemplate string
	//go:embed templates/index.gohtml
	indexTemplate string
	//go:embed templates/courses.gohtml
	coursesTemplate string
	//go:embed templates/course.gohtml
	courseTemplate string
)

func createMyRender() multitemplate.Renderer {
	funcMap := template.FuncMap{"anniRange": func(end int) []int {
		r := make([]int, 0, end)
		for i := 1; i <= end; i++ {
			r = append(r, i)
		}
		return r
	}}

	r := multitemplate.NewRenderer()

	r.AddFromString("base", baseTemplate)
	r.AddFromStringsFuncs("index", funcMap, baseTemplate, indexTemplate)
	r.AddFromStringsFuncs("courses", funcMap, baseTemplate, coursesTemplate)
	r.AddFromStringsFuncs("course", funcMap, baseTemplate, courseTemplate)
	return r
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	downloadOpenDataIfNewer()

	courses, err := openData()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to open open data file")
	}

	r := setupRouter(courses)

	err = r.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to start server")
	}
}

func setupRouter(courses unibo.CoursesMap) *gin.Engine {
	r := gin.Default()
	r.Use(compress.Compress())
	// Limit payload to 10 MB. This fixes zip bombs.
	r.Use(limits.RequestSizeLimiter(10 * 1024 * 1024))
	r.HTMLRender = createMyRender()

	r.Static("/static", "./static")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index", gin.H{})
	})

	coursesList := courses.ToList()
	sort.Sort(coursesList)
	r.GET("/courses", func(c *gin.Context) {
		c.HTML(http.StatusOK, "courses", gin.H{
			"courses": coursesList,
		})
	})

	r.GET("/courses/:id", coursePage(courses))

	r.GET("/cal/:id/:anno", getCoursesCal(&courses))
	return r
}

func coursePage(courses unibo.CoursesMap) func(c *gin.Context) {
	return func(c *gin.Context) {
		courseId := c.Param("id")
		if courseId == "" {
			c.String(http.StatusBadRequest, "Invalid course id")
			return
		}

		courseIdInt, err := strconv.Atoi(courseId)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid course id")
			return
		}

		course, found := courses.FindById(courseIdInt)
		if !found {
			c.String(http.StatusNotFound, "Course not found")
			return
		}

		curricula, err := course.GetAllCurricula()
		if err != nil {
			_ = c.Error(err)
			c.String(http.StatusInternalServerError, "Unable to retrieve curricula")
			return
		}

		c.HTML(http.StatusOK, "course", gin.H{
			"Course":    course,
			"Curricula": curricula,
		})
	}
}

var calcache = cache.New(time.Minute*10, time.Minute*30)

func getCoursesCal(courses *unibo.CoursesMap) func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		anno := c.Param("anno")

		cacheKey := fmt.Sprintf("%s-%s", id, anno)
		if cal, found := calcache.Get(cacheKey); found {
			successCalendar(c, cal.(*bytes.Buffer))
			return
		}

		// Check if id is a number, otherwise return 400
		annoInt, err := strconv.Atoi(anno)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid year")
			return
		}

		// Check if id is a number, otherwise return 400
		idInt, err := strconv.Atoi(id)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid id")
			return
		}

		// Check if course exists, otherwise return 404
		course, found := courses.FindById(idInt)
		if !found {
			c.String(http.StatusNotFound, "Course not found")
			return
		}

		if annoInt <= 0 || annoInt > course.DurataAnni {
			c.String(http.StatusBadRequest, "Invalid year")
			return
		}

		curriculumId := c.Query("curriculum")
		curriculum := unibo.Curriculum{}
		if curriculumId != "" {
			curriculum.Value = curriculumId
		}

		// Try to retrieve timetable, otherwise return 500
		timetable, err := course.GetTimetable(annoInt, curriculum)
		if err != nil {
			_ = c.Error(err)
			c.String(http.StatusInternalServerError, "Unable to retrieve timetable")
			return
		}

		cal := createCal(timetable, course, annoInt)
		buf := bytes.NewBuffer(nil)
		err = cal.SerializeTo(buf)
		if err != nil {
			_ = c.Error(err)
			c.String(http.StatusInternalServerError, "Unable to serialize calendar")
			return
		}
		calcache.Set(cacheKey, buf, cache.DefaultExpiration)

		successCalendar(c, buf)
	}
}

func successCalendar(c *gin.Context, cal *bytes.Buffer) {
	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=lezioni.ics")
	// Allow CORS
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization")
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")

	c.String(http.StatusOK, cal.String())
}

func createCal(timetable unibo.Timetable, course *unibo.Course, year int) (cal *ics.Calendar) {
	cal = timetable.ToICS()
	cal.SetName(fmt.Sprintf("%s - %d year", course.Descrizione, year))
	cal.SetDescription(
		fmt.Sprintf("Orario delle lezioni del %d anno del corso di %s", year, course.Descrizione),
	)
	return
}
