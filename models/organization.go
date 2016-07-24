package models

import (
	"strings"
	"time"

	"github.com/qedus/nds"
	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore"
)

type Organization struct {
	ID string `datastore:"-"`

	// IDSearch is a variable used to search for this organization by their SID.
	// It is automatically set by PutOrganization.
	IDSearch string

	FirstSeen   time.Time
	LastUpdated time.Time
}

func GetOrganization(c context.Context, spectrumID string) (*Organization, error) {
	var organization Organization

	err := nds.Get(c, GenerateOrganizationKey(c, spectrumID), &organization)
	if err == datastore.ErrNoSuchEntity {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	organization.ID = spectrumID

	return &organization, nil
}

func PutOrganization(c context.Context, organization *Organization) error {
	organization.IDSearch = strings.ToLower(organization.ID)

	_, err := nds.Put(c, GenerateOrganizationKey(c, organization.ID), organization)

	return err
}

func GenerateOrganizationKey(c context.Context, id string) *datastore.Key {
	return datastore.NewKey(c, "Organization", id, 0, nil)
}
