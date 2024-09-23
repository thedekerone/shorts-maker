package pkg

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/thedekerone/shorts-maker/models"
)

func TestCreateDialogFromWords(t *testing.T) {
	segment := models.Segment{
		Start: 0.0,
		End:   10.0,
		Words: []models.Word{
			{
				Start: 0.2,
				End:   2.0,
				Word:  "hola",
			},
			{
				Start: 2.4,
				End:   4.0,
				Word:  "como",
			},
			{
				Start: 4.2,
				End:   9.0,
				Word:  "estas",
			},
		},
	}

	dialog, err := CreateDialogFromWords(segment)

	if err != nil {
		t.Fatalf("Failed to run function")
	}

	rows := strings.Split(dialog, "\n")

	expectedRows := []string{
		fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s", floatToAssTimeStamp(0.0), floatToAssTimeStamp(2.4), segment.Words[0].Word),
		fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s", floatToAssTimeStamp(2.4), floatToAssTimeStamp(4.2), segment.Words[1].Word),
		fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s", floatToAssTimeStamp(4.2), floatToAssTimeStamp(10.0), segment.Words[2].Word),
	}

	for i := range rows {
		if i == len(rows)-1 {
			break
		}
		expectedRow := expectedRows[i]
		match, _ := regexp.MatchString(expectedRow, rows[i])
		if !match {
			t.Fatalf("wrong row %d: expected %s, got %s", i+1, expectedRow, rows[i])
		}
	}
}
