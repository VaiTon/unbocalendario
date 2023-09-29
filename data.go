package main

import (
	"encoding/json"
	"github.com/csunibo/unibo-go/opendata"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/VaiTon/unibocalendar/unibo_integ"
)

const (
	coursesPathJson = "data/courses.json"
	packageId       = "degree-programmes"
	resourceAlias   = "corsi_latest_it"
)

func downloadOpenDataIfNewer() {

	// Get package
	pack, err := opendata.FetchPackage(packageId)
	if err != nil {
		log.Warn().Err(err).Msg("unable to get package")
		return
	}

	// If no resources, return nil
	if len(pack.Result.Resources) == 0 {
		log.Warn().Msg("no resources found while downloading open data")
		return
	}

	// Get wanted resource
	resource, found := pack.Result.Resources.GetByAlias(resourceAlias)
	if !found {
		log.Warn().Msgf("unable to find resource '%s'", resourceAlias)
		return
	}

	// Get last modified resource
	lastMod := resource.LastMod

	// Parse last modified time
	lastModTime, err := time.Parse("2006-01-02T15:04:05.999999999", lastMod)
	if err != nil {
		log.Panic().Err(err).Msg("Unable to parse last modified time")
	}

	old := false
	// Get file last modified time, if file does not exist return lastMod.Url
	stat, err := os.Stat(coursesPathJson)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Panic().Err(err).Msg("Unable to get file stat")
		} else {
			old = true
		}
	}

	if !old && stat.ModTime().After(lastModTime) {
		log.Info().Msg("Opendata file is up to date")
		return
	}

	courses, err := unibo_integ.DownloadResource(resource)
	if err != nil {
		log.Panic().Err(err).Msg("Unable to download courses")
	}

	actualYear := time.Now().Year()

	// Filter courses by actual year
	courses = lo.Filter(courses, func(c unibo_integ.Course, _ int) bool {
		return strings.Contains(c.AnnoAccademico, strconv.Itoa(actualYear))
	})

	err = saveData(courses)
	if err != nil {
		log.Panic().Err(err).Msg("Unable to save courses")
	}

	log.Info().Msg("Opendata file downloaded")
}

func saveData(courses []unibo_integ.Course) error {
	err := createDataFolder()
	if err != nil {
		return err
	}

	jsonFile, err := os.Create(coursesPathJson)
	if err != nil {
		return err
	}

	err = json.NewEncoder(jsonFile).Encode(courses)
	if err != nil {
		return err
	}

	return nil
}

func createDataFolder() error {
	return os.MkdirAll(path.Dir(coursesPathJson), os.ModePerm)
}

func openData() (unibo_integ.CoursesMap, error) {
	// Open file
	file, err := os.Open(coursesPathJson)
	if err != nil {
		return nil, err
	}

	// Decode json
	courses := make([]unibo_integ.Course, 0)
	err = json.NewDecoder(file).Decode(&courses)
	if err != nil {
		return nil, err
	}

	// Close file
	err = file.Close()
	if err != nil {
		return nil, err
	}

	// Create the map
	courseMap := make(unibo_integ.CoursesMap, len(courses))
	for _, course := range courses {
		courseMap[course.Codice] = course
	}

	return courseMap, nil
}
