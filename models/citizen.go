package models

import (
	"time"

	"github.com/qedus/nds"
	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore"
)

type Citizen struct {
	ID      int64 `datastore:"-"`
	Handle  string
	Moniker string

	LastUpdated time.Time
}

func GetCitizen(c context.Context, id int64) (*Citizen, error) {
	var citizen Citizen

	err := nds.Get(c, generateCitizenKey(c, id), &citizen)
	if err == datastore.ErrNoSuchEntity {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	citizen.ID = id

	return &citizen, nil
}

func PutCitizen(c context.Context, citizen *Citizen) error {
	_, err := nds.Put(c, generateCitizenKey(c, citizen.ID), citizen)
	return err
}

func generateCitizenKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, "Citizen", "", id, nil)
}
