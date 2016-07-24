package models

import (
	"time"

	"github.com/qedus/nds"
	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore"
)

type CitizenHistory struct {
	ID int64 `datastore:"-"`

	Handles       []HistoryItem
	Monikers      []HistoryItem
	Organizations []HistoryItem
}

type HistoryItem struct {
	Value     string
	FirstSeen time.Time
	LastSeen  time.Time
}

func GetCitizenHistory(c context.Context, id int64) (*CitizenHistory, error) {
	var citizenHistory CitizenHistory

	err := nds.Get(c, GenerateCitizenHistoryKey(c, id), &citizenHistory)
	if err == datastore.ErrNoSuchEntity {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	citizenHistory.ID = id

	return &citizenHistory, nil
}

func PutCitizenHistory(c context.Context, citizenHistory *CitizenHistory) error {
	_, err := nds.Put(c, GenerateCitizenHistoryKey(c, citizenHistory.ID), citizenHistory)
	return err
}

func GenerateCitizenHistoryKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, "CitizenHistory", "", id, nil)
}
