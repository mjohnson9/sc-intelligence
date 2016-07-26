package models

import (
	"time"

	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore"
)

const (
	ChangedCrawl     int16 = 1
	ChangedHandle    int16 = 2
	ChangedMoniker   int16 = 3
	ChangedOrgJoined int16 = 4
	ChangedOrgLeft   int16 = 5
)

type CitizenHistory struct {
	key *datastore.Key `datastore:"-"`

	WhatChanged int16
	NewValue    string
	Timestamp   time.Time
}

func generateCitizenHistoryItemKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewIncompleteKey(c, "CitizenHistory", generateCitizenKey(c, id))
}
