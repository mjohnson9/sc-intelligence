package models

import (
	"encoding/binary"
	"strings"
	"time"

	"github.com/qedus/nds"
	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
)

type Citizen struct {
	ID            int64  `datastore:"-"`
	Handle        string `datastore:",noindex"`
	Moniker       string `datastore:",noindex"`
	Organizations []string

	// HandleSearch is a variable used to search for this citizen by their
	// handle. It is automatically set by PutCitizen.
	HandleSearch   string
	originalHandle string `datastore:"-"`

	FirstSeen   time.Time
	LastUpdated time.Time
}

func GetCitizen(c context.Context, id int64) (*Citizen, error) {
	var citizen Citizen

	err := nds.Get(c, GenerateCitizenKey(c, id), &citizen)
	if err == datastore.ErrNoSuchEntity {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	citizen.ID = id
	citizen.originalHandle = citizen.Handle

	return &citizen, nil
}

func PutCitizen(c context.Context, citizen *Citizen) error {
	citizen.HandleSearch = strings.ToLower(citizen.Handle)

	_, err := nds.Put(c, GenerateCitizenKey(c, citizen.ID), citizen)

	if err == nil && citizen.Handle != citizen.originalHandle {
		memcache.Delete(c, "handle-to-id:"+strings.ToLower(citizen.originalHandle))
		citizen.originalHandle = citizen.Handle
	}

	return err
}

func FindCitizenByHandle(c context.Context, handle string) (*Citizen, error) {
	handle = strings.ToLower(handle)

	memcacheItem, err := memcache.Get(c, "handle-to-id:"+handle)
	if err == nil {
		id, n := binary.Varint(memcacheItem.Value)
		if n > 0 {
			// ID was successfully read

			return GetCitizen(c, id)
		}
	}

	queryIterator := datastore.NewQuery("Citizen").Filter("HandleSearch=", handle).KeysOnly().Limit(1).Run(c)

	citizenKey, err := queryIterator.Next(nil)
	if err == datastore.Done {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	id := citizenKey.IntID()
	idEncoded := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(idEncoded, id)
	idEncoded = idEncoded[:n]
	memcache.Set(c, &memcache.Item{
		Key:   "handle-to-id:" + handle,
		Value: idEncoded,
	})

	return GetCitizen(c, id)
}

func GenerateCitizenKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, "Citizen", "", id, nil)
}
