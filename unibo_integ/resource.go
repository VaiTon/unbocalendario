package unibo_integ

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/csunibo/unibo-go/opendata"
)

func DownloadResource(resource *opendata.Resource) ([]Course, error) {
	// Get the resource
	res, err := Client.Get(resource.Url)
	if err != nil {
		return nil, err
	}

	// Parse the body
	var courses []Course
	if strings.HasSuffix(resource.Url, ".csv") {
		courses, err = downloadCSV(res.Body)
	}
	if err != nil {
		return nil, err
	}

	// Close the body
	err = res.Body.Close()
	if err != nil {
		return nil, err
	}

	if courses == nil {
		return nil, fmt.Errorf("resource is not a csv file")
	}

	return courses, nil
}

func downloadCSV(body io.Reader) ([]Course, error) {
	courses := make([]Course, 0, 100)

	reader := csv.NewReader(body)

	// Skip first line
	_, err := reader.Read()
	if err != nil {
		return nil, err
	}

	for {
		row, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}

		code, err := strconv.ParseInt(row[2], 10, 32)
		if err != nil {
			return nil, err
		}

		years, err := strconv.ParseInt(row[9], 10, 32)
		if err != nil {
			return nil, err
		}

		international, err := strconv.ParseBool(row[10])
		if err != nil {
			return nil, err
		}

		courses = append(courses, Course{
			AnnoAccademico:       row[0],
			Immatricolabile:      row[1],
			Codice:               int(code),
			Descrizione:          row[3],
			Url:                  row[4],
			Campus:               row[5],
			SedeDidattica:        row[6],
			Ambiti:               row[7],
			Tipologia:            row[8],
			DurataAnni:           int(years),
			Internazionale:       international,
			InternazionaleTitolo: row[11],
			InternazionaleLingua: row[12],
			Lingue:               row[13],
			Accesso:              row[14],
		})
	}
	return courses, nil
}
