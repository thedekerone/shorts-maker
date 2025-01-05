package services_test

import (
	"testing"

	"github.com/thedekerone/shorts-maker/services"
)

func TestGetImages(t *testing.T) {
	rs, err := services.NewReplicateService()

	if err != nil {
		t.Fatalf("%v", err)
	}

	images, err := rs.GetImages("image of a dog", 1)

	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("%v", images)

}
